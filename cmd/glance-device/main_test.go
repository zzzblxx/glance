package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type fakePort struct {
	*strings.Reader
	written bytes.Buffer
}

func (p *fakePort) Write(b []byte) (int, error) { return p.written.Write(b) }

func TestUSBExchange(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		configure, ok  bool
	}{
		{"ping", "boot message\n{\"ok\":true,\"protocol\":1,\"device\":\"e1003\"}\n", false, true},
		{"configure", "{\"ok\":true,\"configured\":true}\n", true, true},
		{"wrong device", "{\"ok\":true,\"protocol\":1,\"device\":\"other\"}\n", false, false},
		{"wrong protocol", "{\"ok\":true,\"protocol\":2,\"device\":\"e1003\"}\n", false, false},
		{"rejected", "{\"ok\":false,\"error\":\"secret-from-device\"}\n", true, false},
		{"oversize", strings.Repeat("x", 4097), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePort{Reader: strings.NewReader(tc.response)}
			err := exchange(p, map[string]string{"op": "ping"}, tc.configure)
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-from-device") {
				t.Fatal("raw device error leaked")
			}
			if !bytes.HasSuffix(p.written.Bytes(), []byte{'\n'}) || !json.Valid(bytes.TrimSpace(p.written.Bytes())) {
				t.Fatal("request is not newline-delimited JSON")
			}
		})
	}
}
