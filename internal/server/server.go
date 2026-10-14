package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/render"
	"github.com/zzzblxx/glance/internal/transport/serial"
	"golang.org/x/image/font/gofont/gomono"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed preview.html
var previewHTML string

// Store holds immutable frame byte slices. HTTP handlers retain a consistent
// generation for the entire response even if the next frame arrives mid-write.
type Store struct {
	mu        sync.RWMutex
	frame     render.Frame
	dashboard model.Dashboard
	lastPull  time.Time
	usb       *serialtransport.Status
}
type Status struct {
	Dashboard model.Dashboard         `json:"dashboard"`
	LastPull  time.Time               `json:"last_device_pull"`
	SHA256    string                  `json:"sha256"`
	USB       *serialtransport.Status `json:"usb,omitempty"`
}

func (s *Store) Update(d model.Dashboard, f *render.Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dashboard = d
	if f != nil {
		s.frame = *f
	}
}
func (s *Store) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Status{Dashboard: s.dashboard, LastPull: s.lastPull, SHA256: s.frame.SHA256, USB: s.usb}
}
func (s *Store) SetUSB(status serialtransport.Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usb = &status
}
func (s *Store) Frame() render.Frame { s.mu.RLock(); defer s.mu.RUnlock(); return s.frame }
func (s *Store) pulled()             { s.mu.Lock(); s.lastPull = time.Now(); s.mu.Unlock() }

func readOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "read only", 405)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func DeviceHandler(store *Store, token string) http.Handler {
	expected := sha256.Sum256([]byte("Bearer " + token))
	return readOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if len(token) < 32 || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.URL.Path != "/v1/frame.bin" && r.URL.Path != "/v1/frame" {
			http.NotFound(w, r)
			return
		}
		f := store.Frame()
		if len(f.Raw) != render.FrameBytes {
			http.Error(w, "frame unavailable", 503)
			return
		}
		etag := `"` + f.SHA256 + `"`
		w.Header().Set("ETag", etag)
		if matches(r.Header.Get("If-None-Match"), etag) {
			if r.Method == "GET" && r.URL.Path == "/v1/frame.bin" {
				store.pulled()
			}
			w.WriteHeader(304)
			return
		}
		if r.URL.Path == "/v1/frame" {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "GET" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"width": render.NativeWidth, "height": render.NativeHeight,
					"display_width": render.Width, "display_height": render.Height,
					"orientation": "portrait", "display_to_buffer_rotation_degrees": 90,
					"bytes": render.FrameBytes, "format": "gray4-high-nibble-first",
					"sha256": f.SHA256, "url": "/v1/frame.bin",
				})
			}
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprint(len(f.Raw)))
		if r.Method == "GET" {
			if n, err := w.Write(f.Raw); err == nil && n == len(f.Raw) {
				store.pulled()
			}
		}
	}))
}
func matches(header, etag string) bool {
	for _, v := range strings.Split(header, ",") {
		v = strings.TrimSpace(v)
		if v == "*" || strings.TrimPrefix(v, "W/") == etag {
			return true
		}
	}
	return false
}

func PreviewHandler(store *Store, editors ...*NotesEditor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		// Protect localhost data from DNS rebinding and cross-site embedding.
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" {
			http.Error(w, "loopback host required", 403)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; connect-src 'self'")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		if r.URL.Path == "/api/notes" && len(editors) > 0 && editors[0] != nil {
			editors[0].ServeHTTP(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "read only", 405)
			return
		}
		switch r.URL.Path {
		case "/note-latin.ttf":
			w.Header().Set("Content-Type", "font/ttf")
			if r.Method == "GET" {
				_, _ = w.Write(gomono.TTF)
			}
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.Method == "GET" {
				_, _ = io.WriteString(w, previewHTML)
			}
		case "/status":
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "GET" {
				_ = json.NewEncoder(w).Encode(store.Status())
			}
		case "/frame.png":
			f := store.Frame()
			if len(f.PNG) == 0 {
				http.Error(w, "frame unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			if r.Method == "GET" {
				_, _ = w.Write(f.PNG)
			}
		default:
			http.NotFound(w, r)
		}
	})
}
func Serve(ctx context.Context, deviceAddress, previewAddress, token string, store *Store, editors ...*NotesEditor) error {
	host, _, err := net.SplitHostPort(previewAddress)
	if err != nil || host != "127.0.0.1" {
		return fmt.Errorf("preview must bind to 127.0.0.1")
	}
	b, err := net.Listen("tcp", previewAddress)
	if err != nil {
		return err
	}
	defer b.Close()
	makeServer := func(h http.Handler) *http.Server {
		return &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	}
	device := makeServer(DeviceHandler(store, token))
	preview := makeServer(PreviewHandler(store, editors...))
	defer device.Close()
	defer preview.Close()
	failures := make(chan error, 2)
	// USB mode never opens the legacy LAN listener.
	if deviceAddress != "" {
		a, err := net.Listen("tcp", deviceAddress)
		if err != nil {
			return err
		}
		defer a.Close()
		go func() { failures <- device.Serve(a) }()
	}
	go func() { failures <- preview.Serve(b) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-failures:
		return err
	}
}
