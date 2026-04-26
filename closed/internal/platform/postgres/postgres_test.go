package postgres

import "testing"

func TestConfigValidate(t *testing.T) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv() err=%v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() err=%v", err)
	}
}
