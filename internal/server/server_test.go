package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/render"
)

const testToken = "test-device-012345678901234567890123456789"

func request(h http.Handler, path, token, etag string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8788"+path, nil)
	r.Header.Set("Authorization", token)
	r.Header.Set("If-None-Match", etag)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDeviceBoundaryAndConditionalDelivery(t *testing.T) {
	store := &Store{}
	f := render.Frame{Raw: make([]byte, render.FrameBytes), SHA256: strings.Repeat("a", 64)}
	store.Update(model.Dashboard{}, &f)
	h := DeviceHandler(store, testToken)
	for _, token := range []string{"", "Bearer invalid", "Bearer other-device-012345678901234567890123456789"} {
		if got := request(h, "/v1/frame.bin", token, ""); got.Code != 401 {
			t.Fatalf("invalid credential: %d", got.Code)
		}
	}
	if !store.Status().LastPull.IsZero() {
		t.Fatal("unauthorized request marked device connected")
	}
	for _, path := range []string{"/", "/status", "/frame.png", "/config", "/v1/command"} {
		if got := request(h, path, "Bearer "+testToken, ""); got.Code != 404 {
			t.Fatalf("exposed %s", path)
		}
	}
	first := request(h, "/v1/frame.bin", "Bearer "+testToken, "")
	if first.Code != 200 || first.Body.Len() != render.FrameBytes {
		t.Fatal("invalid frame response")
	}
	if store.Status().LastPull.IsZero() {
		t.Fatal("frame pull not tracked")
	}
	next := request(h, "/v1/frame.bin", "Bearer "+testToken, first.Header().Get("ETag"))
	if next.Code != 304 || next.Body.Len() != 0 {
		t.Fatal("unchanged frame downloaded")
	}
	meta := request(h, "/v1/frame", "Bearer "+testToken, "")
	var metadata struct {
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		DisplayWidth  int    `json:"display_width"`
		DisplayHeight int    `json:"display_height"`
		Orientation   string `json:"orientation"`
		Rotation      int    `json:"display_to_buffer_rotation_degrees"`
	}
	if err := json.Unmarshal(meta.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Width != 1872 || metadata.Height != 1404 || metadata.DisplayWidth != 1404 || metadata.DisplayHeight != 1872 || metadata.Orientation != "portrait" || metadata.Rotation != 90 {
		t.Fatalf("incorrect native/portrait metadata: %+v", metadata)
	}
	if strings.Contains(meta.Body.String(), testToken) || strings.Contains(meta.Body.String(), "tasks") {
		t.Fatal("metadata leaks internal state")
	}
	r := httptest.NewRequest("POST", "/v1/frame.bin", strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal("write accepted")
	}
}
func TestPreviewRejectsReboundHost(t *testing.T) {
	h := PreviewHandler(&Store{})
	r := httptest.NewRequest("GET", "http://attacker.example/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("untrusted Host accepted")
	}
	if request(h, "/status", "", "").Code != 200 {
		t.Fatal("loopback rejected")
	}
	page := request(h, "/", "", "")
	if page.Code != 200 || !strings.Contains(page.Body.String(), "aspect-ratio:3/4") || !strings.Contains(page.Body.String(), `width="1404" height="1872"`) {
		t.Fatal("preview page does not preserve portrait aspect ratio")
	}
}
func TestNoFrameIsUnavailableNotZeroImage(t *testing.T) {
	w := request(DeviceHandler(&Store{}, testToken), "/v1/frame.bin", "Bearer "+testToken, "")
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
