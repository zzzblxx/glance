// Package serialtransport sends bounded structured snapshots, never image frames.
package serialtransport

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/zzzblxx/glance/internal/devices"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/protocol"
	"go.bug.st/serial"
)

const ChunkSize = 1024

var ErrProtocol = errors.New("USB device protocol unavailable")

type Status struct {
	State       string    `json:"state"`
	LastSeen    time.Time `json:"last_seen"`
	LastApplied time.Time `json:"last_applied"`
	SnapshotSHA string    `json:"snapshot_sha256,omitempty"`
	FrameSHA    string    `json:"frame_sha256,omitempty"`
	Page        int       `json:"page"`
	Reason      string    `json:"reason,omitempty"`
}
type Reply struct {
	OK       bool   `json:"ok"`
	Op       string `json:"op"`
	Protocol int    `json:"protocol"`
	Device   string `json:"device"`
	ID       string `json:"id"`
	Renderer string `json:"renderer"`
	Offset   int    `json:"offset"`
	Snapshot string `json:"snapshot"`
	Frame    string `json:"frame"`
	Page     int    `json:"page"`
}
type Client struct {
	port    io.ReadWriter
	reader  *bufio.Reader
	profile devices.Profile
}

func New(port io.ReadWriter, profile devices.Profile) *Client {
	return &Client{port, bufio.NewReaderSize(port, 4096), profile}
}
func (c *Client) read() (Reply, error) {
	line, err := c.reader.ReadSlice('\n')
	if err != nil {
		return Reply{}, ErrProtocol
	}
	var reply Reply
	if len(line) > 2048 || json.Unmarshal(line, &reply) != nil || !reply.OK {
		return Reply{}, ErrProtocol
	}
	return reply, nil
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return ErrProtocol
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func (c *Client) command(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return ErrProtocol
	}
	return writeAll(c.port, append(b, '\n'))
}
func (c *Client) Hello() (Reply, error) {
	if err := c.command(map[string]any{"op": "hello", "protocol": c.profile.Protocol}); err != nil {
		return Reply{}, err
	}
	r, err := c.read()
	if err != nil || c.profile.ID == "" || c.profile.RendererVersion == "" || r.Op != "hello" || r.Protocol != c.profile.Protocol || r.Device != c.profile.ID || r.Renderer != c.profile.RendererVersion || len(r.ID) != 12 {
		return Reply{}, ErrProtocol
	}
	if _, err := hex.DecodeString(r.ID); err != nil || r.Page < 0 || r.Page >= c.profile.MaxPages {
		return Reply{}, ErrProtocol
	}
	return r, nil
}
func (c *Client) Send(payload []byte, hash string) (Reply, error) {
	if len(payload) == 0 || len(payload) > protocol.MaxPayload || len(hash) != 64 {
		return Reply{}, ErrProtocol
	}
	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != hash {
		return Reply{}, ErrProtocol
	}
	if err := c.command(map[string]any{"op": "begin", "protocol": c.profile.Protocol, "bytes": len(payload), "sha256": hash}); err != nil {
		return Reply{}, err
	}
	r, err := c.read()
	if err != nil || r.Op != "ready" {
		return Reply{}, ErrProtocol
	}
	for offset := 0; offset < len(payload); {
		end := min(offset+ChunkSize, len(payload))
		if err := writeAll(c.port, payload[offset:end]); err != nil {
			return Reply{}, err
		}
		r, err = c.read()
		if err != nil {
			return Reply{}, err
		}
		offset = end
		if offset < len(payload) {
			if r.Op != "chunk" || r.Offset != offset {
				return Reply{}, ErrProtocol
			}
		} else if r.Op != "applied" || r.Snapshot != hash || !validHash(r.Frame) || r.Page < 0 || r.Page >= c.profile.MaxPages {
			return Reply{}, ErrProtocol
		}
	}
	return r, nil
}

func validHash(s string) bool { b, err := hex.DecodeString(s); return err == nil && len(b) == 32 }

// Sync checks the paired identity before transmitting any display data.
func (c *Client) Sync(id string, snapshot func() model.Dashboard) (Reply, bool, error) {
	hello, err := c.Hello()
	if err != nil || hello.ID != id {
		return Reply{}, false, ErrProtocol
	}
	payload, hash, err := protocol.Encode(snapshot())
	if err != nil {
		return Reply{}, false, err
	}
	if hello.Snapshot == hash {
		return hello, false, nil
	}
	applied, err := c.Send(payload, hash)
	return applied, err == nil, err
}

// timedPort converts serial zero-byte timeouts into bounded I/O operations.
// Closing the actual port on cancellation also interrupts a blocked write.
type timedPort struct{ serial.Port }

func (p timedPort) Read(b []byte) (int, error) {
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		n, err := p.Port.Read(b)
		if n > 0 || err != nil {
			return n, err
		}
	}
	return 0, ErrProtocol
}
func Open(path string, profile devices.Profile) (serial.Port, *Client, error) {
	p, err := serial.Open(path, &serial.Mode{BaudRate: profile.BaudRate, InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false}})
	if err != nil {
		return nil, nil, ErrProtocol
	}
	if err = p.SetReadTimeout(250 * time.Millisecond); err != nil {
		p.Close()
		return nil, nil, ErrProtocol
	}
	if err = p.ResetInputBuffer(); err != nil {
		p.Close()
		return nil, nil, ErrProtocol
	}
	return p, New(timedPort{p}, profile), nil
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Run opens only the explicitly paired port, verifies identity before sending
// any dashboard, and retries with bounded delays. It never executes device text.
func Run(ctx context.Context, path, id string, profile devices.Profile, snapshot func() model.Dashboard, report func(Status)) error {
	status := Status{State: "disconnected"}
	if path == "" || id == "" {
		status.State = "unpaired"
		report(status)
		<-ctx.Done()
		return nil
	}
	for ctx.Err() == nil {
		p, c, err := Open(path, profile)
		if err != nil {
			status.State = "disconnected"
			status.Reason = "port_unavailable"
			report(status)
			if !wait(ctx, 6*time.Second) {
				break
			}
			continue
		}
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				p.Close()
			case <-done:
			}
		}()
		for ctx.Err() == nil {
			// A whole exchange is bounded, including writes and malicious slow reads.
			watchdog := time.AfterFunc(60*time.Second, func() { _ = p.Close() })
			result, applied, e := c.Sync(id, snapshot)
			if e != nil {
				watchdog.Stop()
				break
			}
			status.State = "connected"
			status.LastSeen = time.Now()
			status.Reason = ""
			status.Page = result.Page
			status.FrameSHA = result.Frame
			status.SnapshotSHA = result.Snapshot
			if applied {
				status.LastApplied = time.Now()
				status.LastSeen = status.LastApplied
			}
			watchdog.Stop()
			report(status)
			if !wait(ctx, 5*time.Second) {
				break
			}
		}
		close(done)
		p.Close()
		if ctx.Err() != nil {
			break
		}
		status.State = "disconnected"
		status.Reason = "protocol_or_device_unavailable"
		report(status)
		if !wait(ctx, 6*time.Second) {
			break
		}
	}
	return nil
}
