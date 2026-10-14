package installation

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Uses real packaged binaries and plist generation, but a fake launchctl and a
// private installation root. Never changes HOME or touches the user's services.
func TestInstallerLifecycle(t *testing.T) {
	if os.Getenv("GLANCE_INSTALL_TEST") != "1" {
		t.Skip("build binaries and set GLANCE_INSTALL_TEST=1")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if extracted := os.Getenv("GLANCE_INSTALL_SOURCE"); extracted != "" {
		repo, err = filepath.Abs(extracted)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range []string{"fresh", "legacy", "repeat", "conflict", "bad-config", "bootstrap-failure", "stop-failure", "custom-data", "uninstall"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			support := filepath.Join(root, "Library", "Application Support")
			old := filepath.Join(support, "AIUsageDashboard")
			app := filepath.Join(support, "Glance")
			agents := filepath.Join(root, "Library", "LaunchAgents")
			mock := filepath.Join(root, "mock-bin")
			state := filepath.Join(root, "launch-state")
			for _, dir := range []string{mock, state, agents} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, body string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), mode); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(mock, "codex"), "#!/bin/sh\nexit 0\n", 0700)
			write(filepath.Join(mock, "launchctl"), `#!/bin/bash
set -eu
case "$1" in
 print) test -f "$GLANCE_TEST_STATE/${2##*/}" ;;
 bootout)
  [[ "${GLANCE_TEST_FAIL:-}" != stop ]] || exit 9
  name=$(/usr/bin/plutil -extract Label raw "$3")
  rm -f "$GLANCE_TEST_STATE/$name" ;;
 bootstrap)
  name=$(/usr/bin/plutil -extract Label raw "$3")
  if [[ "${GLANCE_TEST_FAIL:-}" == bootstrap && "$name" == com.glance.companion ]]; then exit 9; fi
  touch "$GLANCE_TEST_STATE/$name" ;;
 *) exit 8 ;;
esac
`, 0700)
			fail := ""
			if scenario == "bootstrap-failure" {
				fail = "bootstrap"
			}
			if scenario == "stop-failure" {
				fail = "stop"
			}
			env := append(os.Environ(), "PATH="+mock+":"+os.Getenv("PATH"), "GLANCE_INSTALL_HOME="+root, "GLANCE_TEST_STATE="+state, "GLANCE_TEST_FAIL="+fail)
			run := func(script string) ([]byte, error) {
				c := exec.Command("bash", filepath.Join(repo, "scripts", script))
				c.Env = env
				return c.CombinedOutput()
			}
			var original []byte
			if scenario != "fresh" {
				os.MkdirAll(filepath.Join(old, "bin"), 0700)
				data := old
				if scenario == "custom-data" {
					data = filepath.Join(root, "external-data")
					os.MkdirAll(data, 0700)
				}
				cfg := map[string]any{"transport": "usb", "device_token": "test-token-never-real-012345678901234", "data_directory": data, "renderer_binary": filepath.Join(old, "bin", "e1003-render"), "preview_listen": "127.0.0.1:8788", "usb_port": "/dev/fake", "usb_device_id": "012345abcdef", "notes": []string{"保留便签", "", "", ""}, "future": "keep"}
				original, _ = json.Marshal(cfg)
				write(filepath.Join(old, "config.json"), string(original), 0600)
				write(filepath.Join(old, "state.json"), "cached-state", 0600)
				write(filepath.Join(old, "bin", "ai-usage-dashboard"), "old-program", 0700)
				plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>Label</key><string>com.ai-usage-dashboard.companion</string></dict></plist>`
				write(filepath.Join(agents, "com.ai-usage-dashboard.companion.plist"), plist, 0600)
				write(filepath.Join(state, "com.ai-usage-dashboard.companion"), "", 0600)
				if scenario == "conflict" {
					os.MkdirAll(app, 0700)
					write(filepath.Join(app, "config.json"), "do-not-overwrite", 0600)
				}
				if scenario == "bad-config" {
					original = []byte("invalid")
					write(filepath.Join(old, "config.json"), string(original), 0600)
				}
			}
			out, err := run("install.sh")
			shouldFail := scenario == "conflict" || scenario == "bad-config" || scenario == "bootstrap-failure" || scenario == "stop-failure"
			if shouldFail {
				if err == nil {
					t.Fatalf("expected failure: %s", out)
				}
				b, _ := os.ReadFile(filepath.Join(old, "config.json"))
				if string(b) != string(original) {
					t.Fatal("old config not restored")
				}
				if _, err := os.Stat(filepath.Join(state, "com.ai-usage-dashboard.companion")); err != nil {
					t.Fatalf("old service not restored: %s", out)
				}
				if scenario != "conflict" {
					if _, err := os.Stat(app); !os.IsNotExist(err) {
						t.Fatalf("partial new install left active: %s", out)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("install: %v\n%s", err, out)
			}
			if scenario == "repeat" {
				if out, err = run("install.sh"); err != nil {
					t.Fatalf("repeat: %v %s", err, out)
				}
			}
			b, err := os.ReadFile(filepath.Join(app, "config.json"))
			if err != nil {
				t.Fatal(err)
			}
			var cfg map[string]any
			json.Unmarshal(b, &cfg)
			if cfg["preview_listen"] != "127.0.0.1:8888" {
				t.Fatal("port not migrated")
			}
			wantData := app
			if scenario == "custom-data" {
				wantData = filepath.Join(root, "external-data")
			}
			if cfg["data_directory"] != wantData {
				t.Fatal("wrong data directory", cfg["data_directory"])
			}
			if scenario != "fresh" {
				if cfg["future"] != "keep" || cfg["usb_device_id"] != "012345abcdef" || cfg["notes"].([]any)[0] != "保留便签" {
					t.Fatal("settings lost")
				}
				if cfg["renderer_binary"] != filepath.Join(app, "bin", "glance-render") {
					t.Fatal("helper path not migrated")
				}
				if cached, _ := os.ReadFile(filepath.Join(app, "state.json")); string(cached) != "cached-state" {
					t.Fatal("cache lost")
				}
				matches, _ := filepath.Glob(filepath.Join(support, "Glance Backups", "install.*", "AIUsageDashboard", "config.json"))
				if len(matches) != 1 {
					t.Fatal("backup missing")
				}
				archived, _ := os.ReadFile(matches[0])
				if string(archived) != string(original) {
					t.Fatal("original backup changed")
				}
			}
			if _, err := os.Stat(old); !os.IsNotExist(err) {
				t.Fatal("legacy directory remains active")
			}
			plist, _ := os.ReadFile(filepath.Join(agents, "com.glance.companion.plist"))
			if strings.Contains(string(plist), "AIUsageDashboard") || !strings.Contains(string(plist), "/bin/glance") {
				t.Fatal("old launch configuration")
			}
			if _, err := os.Stat(filepath.Join(state, "com.glance.companion")); err != nil {
				t.Fatal("new service absent")
			}
			if _, err := os.Stat(filepath.Join(agents, "com.ai-usage-dashboard.companion.plist")); !os.IsNotExist(err) {
				t.Fatal("old plist active")
			}
			if scenario == "uninstall" {
				for i := 0; i < 2; i++ {
					if out, err = run("uninstall.sh"); err != nil {
						t.Fatalf("uninstall: %v %s", err, out)
					}
				}
				preserved, _ := os.ReadFile(filepath.Join(app, "config.json"))
				if string(preserved) != string(b) {
					t.Fatal("uninstall removed config")
				}
				if _, err := os.Stat(filepath.Join(app, "bin", "glance")); !os.IsNotExist(err) {
					t.Fatal("binary not removed")
				}
				if out, err = run("install.sh"); err != nil {
					t.Fatalf("reinstall: %v %s", err, out)
				}
			}
		})
	}
}
