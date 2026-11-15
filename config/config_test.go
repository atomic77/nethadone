package config

import "testing"

// The shipped sample must parse (unknown keys are fatal) and document the real defaults.
func TestSampleConfigMatchesDefaults(t *testing.T) {
	Cfg = Config{}
	ParseConfig("nethadone.yml", true)
	if Cfg != baseConfig {
		t.Errorf("config/nethadone.yml differs from baseConfig:\n got %+v\nwant %+v", Cfg, baseConfig)
	}
	Cfg = baseConfig
}
