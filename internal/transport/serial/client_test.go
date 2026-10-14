package serialtransport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/zzzblxx/glance/internal/devices/e1003"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/protocol"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type transcript struct {
	read    *bytes.Reader
	written bytes.Buffer
	partial int
}

func (p *transcript) Read(b []byte) (int, error) { return p.read.Read(b) }
func (p *transcript) Write(b []byte) (int, error) {
	if p.partial > 0 && len(b) > p.partial {
		b = b[:p.partial]
	}
	return p.written.Write(b)
}
func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func hello(snapshot string) string {
	b, _ := json.Marshal(Reply{OK: true, Op: "hello", Protocol: 2, Device: "e1003", ID: "012345abcdef", Renderer: e1003.RendererVersion, Snapshot: snapshot})
	return string(b) + "\n"
}

func TestHandshakeUsesSelectedProfile(t *testing.T) {
	profile := e1003.Profile()
	profile.ID, profile.RendererVersion = "test-panel", "test-renderer"
	line := strings.ReplaceAll(strings.ReplaceAll(hello(""), "e1003", profile.ID), e1003.RendererVersion, profile.RendererVersion)
	p := &transcript{read: bytes.NewReader([]byte(line))}
	if _, err := New(p, profile).Hello(); err != nil {
		t.Fatal("selected profile was rejected", err)
	}
	p = &transcript{read: bytes.NewReader([]byte(line))}
	if _, err := New(p, e1003.Profile()).Hello(); err == nil {
		t.Fatal("wrong hardware profile accepted")
	}
}

func TestChunkedTransfer(t *testing.T) {
	host, dev := net.Pipe()
	defer host.Close()
	defer dev.Close()
	_ = host.SetDeadline(time.Now().Add(3 * time.Second))
	_ = dev.SetDeadline(time.Now().Add(3 * time.Second))
	payload := bytes.Repeat([]byte("structured-data"), 200)
	sha := hash(payload)
	done := make(chan error, 1)
	go func() {
		r := bufio.NewReader(dev)
		line, err := r.ReadBytes('\n')
		if err != nil {
			done <- err
			return
		}
		var begin map[string]any
		if err = json.Unmarshal(line, &begin); err != nil {
			done <- err
			return
		}
		if begin["bytes"] != float64(len(payload)) || begin["sha256"] != sha {
			done <- ErrProtocol
			return
		}
		_, err = io.WriteString(dev, "{\"ok\":true,\"op\":\"ready\"}\n")
		if err != nil {
			done <- err
			return
		}
		for offset := 0; offset < len(payload); {
			end := min(offset+ChunkSize, len(payload))
			chunk := make([]byte, end-offset)
			if _, err = io.ReadFull(r, chunk); err != nil {
				done <- err
				return
			}
			if !bytes.Equal(chunk, payload[offset:end]) {
				done <- ErrProtocol
				return
			}
			offset = end
			reply := Reply{OK: true, Op: "chunk", Offset: offset}
			if offset == len(payload) {
				reply.Op = "applied"
				reply.Snapshot = sha
				reply.Frame = sha
			}
			if err = json.NewEncoder(dev).Encode(reply); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	reply, err := New(host, e1003.Profile()).Send(payload, sha)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Snapshot != sha {
		t.Fatal("wrong applied hash")
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestIdentityAndUnchangedGate(t *testing.T) {
	d := model.Dashboard{}
	_, sha, _ := protocol.Encode(d)
	for _, tc := range []struct {
		id, snapshot string
		wantErr      bool
	}{{"000000000000", "", true}, {"012345abcdef", sha, false}} {
		p := &transcript{read: bytes.NewReader([]byte(hello(tc.snapshot))), partial: 3}
		_, applied, err := New(p, e1003.Profile()).Sync(tc.id, func() model.Dashboard {
			if tc.wantErr {
				t.Fatal("read snapshot before identity verified")
			}
			return d
		})
		if (err != nil) != tc.wantErr || applied {
			t.Fatalf("unexpected sync: %v %v", applied, err)
		}
		if strings.Contains(p.written.String(), "begin") {
			t.Fatal("transmitted unnecessarily")
		}
	}
}

func TestRejectBadReplies(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 1025)
	sha := hash(payload)
	for _, response := range []string{"{\"ok\":false}\n", "{\"ok\":true,\"op\":\"ready\"}\n{\"ok\":true,\"op\":\"chunk\",\"offset\":1}\n", strings.Repeat("x", 5000) + "\n"} {
		p := &transcript{read: bytes.NewReader([]byte(response))}
		if _, err := New(p, e1003.Profile()).Send(payload, sha); err == nil {
			t.Fatal("accepted corrupt response")
		}
	}
	p := &transcript{read: bytes.NewReader(nil)}
	if _, err := New(p, e1003.Profile()).Send(payload, strings.Repeat("0", 64)); err == nil || p.written.Len() != 0 {
		t.Fatal("accepted bad outgoing hash")
	}
}

func TestOldRendererNeverReceivesSnapshot(t *testing.T) {
	old := strings.ReplaceAll(hello(""), e1003.RendererVersion, "portrait-data-1")
	p := &transcript{read: bytes.NewReader([]byte(old))}
	_, _, err := New(p, e1003.Profile()).Sync("012345abcdef", func() model.Dashboard {
		t.Fatal("read dashboard before renderer version verified")
		return model.Dashboard{}
	})
	if err == nil || strings.Contains(p.written.String(), "begin") {
		t.Fatal("old renderer accepted")
	}
}

func TestExpandedPageBounds(t *testing.T) {
	for _, page := range []int{0, 29, protocol.MaxPages - 1, protocol.MaxPages, -1} {
		b, _ := json.Marshal(Reply{OK: true, Op: "hello", Protocol: 2, Device: "e1003", ID: "012345abcdef", Renderer: e1003.RendererVersion, Page: page})
		p := &transcript{read: bytes.NewReader(append(b, '\n'))}
		_, err := New(p, e1003.Profile()).Hello()
		if (err == nil) != (page >= 0 && page < protocol.MaxPages) {
			t.Fatal("incorrect page bound", page, err)
		}
	}
}

func TestUnpairedNeverReadsDashboard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Run(ctx, "", "", e1003.Profile(), func() model.Dashboard { t.Fatal("unpaired data access"); return model.Dashboard{} }, func(s Status) {
		if s.State != "unpaired" {
			t.Fatal(s)
		}
		cancel()
	})
	if err != nil {
		t.Fatal(err)
	}
}
