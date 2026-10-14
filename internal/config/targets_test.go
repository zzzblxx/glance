package config

import "testing"

func TestSupportedTargetsAndLegacyDefaults(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "openai" || cfg.DeviceModel != "e1003" {
		t.Fatal("incorrect defaults")
	}
	cfg.DeviceToken = "test-only-token-with-at-least-32-characters"
	// Validate accepts omitted targets from old in-memory configurations.
	cfg.Provider, cfg.DeviceModel = "", ""
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Provider = "unimplemented"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unimplemented provider accepted")
	}
	cfg.Provider, cfg.DeviceModel = "openai", "unimplemented"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unimplemented device accepted")
	}
}
