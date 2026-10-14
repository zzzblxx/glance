package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"unicode/utf8"

	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/storage"
)

const tokenEnvironment = "GLANCE_DEVICE_TOKEN"
const legacyTokenEnvironment = "AI_USAGE_DASHBOARD_DEVICE_TOKEN"

type Config struct {
	Provider             string        `json:"provider"`
	DeviceModel          string        `json:"device_model"`
	Notes                []string      `json:"notes"`
	BannerID             string        `json:"banner_id"`
	Transport            string        `json:"transport"`
	USBPort              string        `json:"usb_port,omitempty"`
	USBDeviceID          string        `json:"usb_device_id,omitempty"`
	RendererBinary       string        `json:"renderer_binary,omitempty"`
	DeviceListen         string        `json:"device_listen"`
	PreviewListen        string        `json:"preview_listen"`
	DeviceToken          string        `json:"device_token"`
	PrivacyMode          bool          `json:"privacy_mode"`
	CodexBinary          string        `json:"codex_binary"`
	CodexHome            string        `json:"codex_home"`
	DataDirectory        string        `json:"data_directory"`
	QuotaPollInterval    time.Duration `json:"-"`
	ActivityPollInterval time.Duration `json:"-"`
	QuotaPoll            string        `json:"quota_poll_interval"`
	ActivityPoll         string        `json:"activity_poll_interval"`
	FontPath             string        `json:"font_path,omitempty"`
}

func Default() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return Config{
		Provider:             "openai",
		DeviceModel:          "e1003",
		Notes:                model.DefaultNotes(),
		BannerID:             "auto",
		Transport:            "usb",
		DeviceListen:         "0.0.0.0:8787",
		PreviewListen:        "127.0.0.1:8888",
		CodexBinary:          "codex",
		CodexHome:            filepath.Join(home, ".codex"),
		DataDirectory:        filepath.Join(home, "Library", "Application Support", "Glance"),
		QuotaPoll:            "10m",
		ActivityPoll:         "30s",
		QuotaPollInterval:    10 * time.Minute,
		ActivityPollInterval: 30 * time.Second,
	}, nil
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Glance", "config.json"), nil
}

func Load(path string) (Config, error) {
	cfg, err := Default()
	if err != nil {
		return Config{}, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(contents, &cfg); err != nil {
		return Config{}, errors.New("invalid configuration JSON")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Config{}, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return Config{}, errors.New("configuration must be owner-only (chmod 600)")
	}
	if token := os.Getenv(tokenEnvironment); token != "" {
		cfg.DeviceToken = token
	} else if token := os.Getenv(legacyTokenEnvironment); token != "" {
		cfg.DeviceToken = token
	}
	if cfg.QuotaPollInterval, err = time.ParseDuration(cfg.QuotaPoll); err != nil {
		return Config{}, fmt.Errorf("invalid quota_poll_interval: %w", err)
	}
	if cfg.ActivityPollInterval, err = time.ParseDuration(cfg.ActivityPoll); err != nil {
		return Config{}, fmt.Errorf("invalid activity_poll_interval: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if cfg.Provider != "" && cfg.Provider != "openai" {
		return errors.New("unsupported provider: only openai is currently implemented")
	}
	if cfg.DeviceModel != "" && cfg.DeviceModel != "e1003" {
		return errors.New("unsupported device_model: only e1003 is currently implemented")
	}
	if len(cfg.Notes) > 4 {
		return errors.New("notes accepts at most four texts")
	}
	for _, note := range cfg.Notes {
		if !utf8.ValidString(note) || utf8.RuneCountInString(note) > 160 {
			return errors.New("each note must be valid UTF-8 and at most 160 characters")
		}
	}
	if cfg.BannerID != "" && cfg.BannerID != "placeholder" && cfg.BannerID != "auto" {
		return errors.New("banner_id: use auto (legacy placeholder is also accepted)")
	}
	if cfg.Transport != "usb" && cfg.Transport != "wifi" {
		return errors.New("transport must be usb or wifi (legacy)")
	}
	if (cfg.USBPort == "") != (cfg.USBDeviceID == "") {
		return errors.New("usb_port and usb_device_id must be paired together")
	}
	if cfg.USBDeviceID != "" {
		if len(cfg.USBDeviceID) != 12 {
			return errors.New("invalid usb_device_id")
		}
		for _, c := range cfg.USBDeviceID {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return errors.New("invalid usb_device_id")
			}
		}
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH != "arm64" {
		return errors.New("the MVP supports Apple Silicon macOS only")
	}
	if len(cfg.DeviceToken) < 32 {
		return errors.New("device_token must contain at least 32 characters")
	}
	if len(cfg.DeviceToken) > 128 {
		return errors.New("device_token must be at most 128 characters")
	}
	for _, r := range cfg.DeviceToken {
		if r <= 32 || r >= 127 {
			return errors.New("device_token must use printable ASCII without spaces")
		}
	}
	if err := requireHostPort("device_listen", cfg.DeviceListen); err != nil {
		return err
	}
	previewHost, _, err := net.SplitHostPort(cfg.PreviewListen)
	if err != nil {
		return fmt.Errorf("invalid preview_listen: %w", err)
	}
	if previewHost != "127.0.0.1" {
		return errors.New("preview_listen must use 127.0.0.1")
	}
	if cfg.QuotaPollInterval < time.Minute || cfg.ActivityPollInterval < 5*time.Second || cfg.ActivityPollInterval > 30*time.Second {
		return errors.New("quota interval must be >= 1m; activity interval must be between 5s and 30s")
	}
	if cfg.CodexBinary == "" || cfg.CodexHome == "" || cfg.DataDirectory == "" {
		return errors.New("codex_binary, codex_home, and data_directory are required")
	}
	return nil
}

func requireHostPort(name, address string) error {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	return nil
}

func WriteInitial(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cfg, err := Default()
	if err != nil {
		return err
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return fmt.Errorf("generate device token: %w", err)
	}
	cfg.DeviceToken = base64.RawURLEncoding.EncodeToString(token)
	if binary, lookupErr := exec.LookPath("codex"); lookupErr == nil {
		cfg.CodexBinary = binary
	}
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	// Linking an existing temp file fails if another initializer won the race.
	return os.Link(temporaryName, path)
}

// MigratePreviewListen changes only the former default. Pairing, credentials,
// custom ports and unknown fields survive upgrades. Call with the service stopped.
func MigratePreviewListen(path string) error {
	if _, err := Load(path); err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(contents, &fields); err != nil || fields == nil {
		return errors.New("configuration must be a JSON object")
	}
	var address string
	if value, ok := fields["preview_listen"]; ok {
		if err := json.Unmarshal(value, &address); err != nil {
			return errors.New("invalid preview_listen")
		}
		if address != "127.0.0.1:8788" {
			return nil
		}
	}
	fields["preview_listen"] = json.RawMessage(`"127.0.0.1:8888"`)
	encoded, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	return storage.AtomicWrite(path, append(encoded, '\n'))
}
