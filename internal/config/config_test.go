package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteInitialCreatesOwnerOnlyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := WriteInitial(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	var raw map[string]any
	contents, _ := os.ReadFile(path)
	if err := json.Unmarshal(contents, &raw); err != nil {
		t.Fatal(err)
	}
	if token, _ := raw["device_token"].(string); len(token) < 32 {
		t.Fatal("generated token is too short")
	}
	if err := WriteInitial(path); err == nil {
		t.Fatal("second initialization unexpectedly overwrote config")
	}
}

func TestValidateRejectsNonLoopbackPreview(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DeviceToken = "01234567890123456789012345678901"
	cfg.PreviewListen = "0.0.0.0:8888"
	if err := cfg.Validate(); err == nil {
		t.Fatal("non-loopback preview listener was accepted")
	}
}

func TestUSBDefaultAndPairingValidation(t *testing.T) {
	cfg, _ := Default()
	cfg.DeviceToken = "01234567890123456789012345678901"
	if cfg.Transport != "usb" {
		t.Fatal("USB must be default")
	}
	if cfg.PreviewListen != "127.0.0.1:8888" {
		t.Fatal("preview must default to port 8888")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.USBPort = "/dev/example"
	if cfg.Validate() == nil {
		t.Fatal("partial pairing accepted")
	}
	cfg.USBDeviceID = "012345abcdef"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.USBDeviceID = "not-a-device"
	if cfg.Validate() == nil {
		t.Fatal("invalid ID accepted")
	}
}

func TestMigratePreviewListenPreservesConfiguration(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8788", "127.0.0.1:8888", "127.0.0.1:9999", ""} {
		t.Run(address, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := WriteInitial(path); err != nil {
				t.Fatal(err)
			}
			contents, _ := os.ReadFile(path)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(contents, &fields); err != nil {
				t.Fatal(err)
			}
			fields["usb_port"] = json.RawMessage(`"/dev/cu.wchusbserial20120"`)
			fields["usb_device_id"] = json.RawMessage(`"012345abcdef"`)
			fields["privacy_mode"] = json.RawMessage(`true`)
			fields["future_setting"] = json.RawMessage(`9007199254740993`)
			fields["preview_listen"], _ = json.Marshal(address)
			if address == "" {
				delete(fields, "preview_listen")
			}
			contents, _ = json.Marshal(fields)
			if err := os.WriteFile(path, contents, 0600); err != nil {
				t.Fatal(err)
			}
			// An environment override must never overwrite the saved credential.
			t.Setenv(tokenEnvironment, "different-environment-token-0123456789")
			for i := 0; i < 2; i++ {
				if err := MigratePreviewListen(path); err != nil {
					t.Fatal(err)
				}
			}
			if address == "" || address == "127.0.0.1:8788" {
				fields["preview_listen"] = json.RawMessage(`"127.0.0.1:8888"`)
			}
			contents, _ = os.ReadFile(path)
			var got map[string]json.RawMessage
			if err := json.Unmarshal(contents, &got); err != nil {
				t.Fatal(err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(fields)
			if !bytes.Equal(gotJSON, wantJSON) {
				t.Fatal("migration changed unrelated settings")
			}
			info, _ := os.Stat(path)
			if info.Mode().Perm() != 0600 {
				t.Fatal("configuration is not owner-only")
			}
		})
	}
}

func TestMigratePreviewListenRejectsInvalidConfigWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	contents := []byte(`{"preview_listen":"127.0.0.1:8788",BROKEN`)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if MigratePreviewListen(path) == nil {
		t.Fatal("invalid configuration accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(contents) {
		t.Fatal("invalid config overwritten")
	}
}

func TestPresentationConfiguration(t *testing.T) {
	cfg, _ := Default()
	cfg.DeviceToken = "test-configuration-token-0123456789"
	if len(cfg.Notes) != 4 || cfg.BannerID != "auto" || cfg.Validate() != nil {
		t.Fatal("invalid display defaults")
	}
	cfg.Notes = append(cfg.Notes, "fifth")
	if cfg.Validate() == nil {
		t.Fatal("fifth note accepted")
	}
	cfg.Notes = nil
	cfg.BannerID = "missing-artwork"
	if cfg.Validate() == nil {
		t.Fatal("unknown artwork accepted")
	}
}
