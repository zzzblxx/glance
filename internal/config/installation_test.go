package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallMigrationPreservesUserSettings(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "AIUsageDashboard")
	target := filepath.Join(root, "Glance")
	for _, custom := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := WriteInitial(path); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		var f map[string]json.RawMessage
		json.Unmarshal(raw, &f)
		data := filepath.Join(old, "runtime")
		if custom {
			data = filepath.Join(root, "external")
		}
		f["data_directory"], _ = json.Marshal(data)
		f["renderer_binary"], _ = json.Marshal(filepath.Join(old, "bin", "e1003-render"))
		f["notes"] = json.RawMessage(`["保留便签","", "第三张", ""]`)
		f["usb_port"] = json.RawMessage(`"/dev/cu.wchusbserial20120"`)
		f["usb_device_id"] = json.RawMessage(`"012345abcdef"`)
		f["future_setting"] = json.RawMessage(`9007199254740993`)
		f["preview_listen"] = json.RawMessage(`"127.0.0.1:9999"`)
		raw, _ = json.Marshal(f)
		os.WriteFile(path, raw, 0600)
		t.Setenv(tokenEnvironment, "override-never-persisted-012345678901234")
		for i := 0; i < 2; i++ {
			if err := MigrateInstallation(path, old, target); err != nil {
				t.Fatal(err)
			}
		}
		if !custom {
			f["data_directory"], _ = json.Marshal(filepath.Join(target, "runtime"))
		}
		f["renderer_binary"], _ = json.Marshal(filepath.Join(target, "bin", "glance-render"))
		got, _ := os.ReadFile(path)
		var actual map[string]json.RawMessage
		json.Unmarshal(got, &actual)
		a, _ := json.Marshal(actual)
		want, _ := json.Marshal(f)
		if string(a) != string(want) {
			t.Fatal("migration changed unrelated values")
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("unsafe permissions")
		}
	}
}

func TestInstallMigrationFreshAndInvalid(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "stage", "config.json")
	target := filepath.Join(root, "Glance")
	if err := MigrateInstallation(path, "", target); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.DataDirectory != target {
		t.Fatal(cfg.DataDirectory, err)
	}
	os.WriteFile(path, []byte("invalid"), 0600)
	if MigrateInstallation(path, filepath.Join(root, "old"), target) == nil {
		t.Fatal("invalid accepted")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "invalid" {
		t.Fatal("invalid config overwritten")
	}
}

func TestGlanceDefaultsAndLegacyTokenFallback(t *testing.T) {
	cfg, _ := Default()
	path, _ := DefaultPath()
	if filepath.Base(cfg.DataDirectory) != "Glance" || filepath.Dir(path) != cfg.DataDirectory {
		t.Fatal("old default path")
	}
	path = filepath.Join(t.TempDir(), "config.json")
	if err := WriteInitial(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv(tokenEnvironment, "")
	t.Setenv(legacyTokenEnvironment, "legacy-token-012345678901234567890")
	cfg, err := Load(path)
	if err != nil || cfg.DeviceToken != "legacy-token-012345678901234567890" {
		t.Fatal("legacy fallback lost", err)
	}
	t.Setenv(tokenEnvironment, "new-token-0123456789012345678901234")
	cfg, err = Load(path)
	if err != nil || cfg.DeviceToken != "new-token-0123456789012345678901234" {
		t.Fatal("new token not preferred", err)
	}
}
