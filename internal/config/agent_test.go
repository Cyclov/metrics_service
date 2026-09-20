package config

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentConfigKey(t *testing.T) {
	for _, tt := range []struct {
		name   string
		args   []string
		envKey string
		want   string
	}{
		{name: "no key"},
		{name: "flag", args: []string{"-k=flag-key"}, want: "flag-key"},
		{name: "environment", envKey: "env-key", want: "env-key"},
		{name: "environment overrides flag", args: []string{"-k=flag-key"}, envKey: "env-key", want: "env-key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KEY", tt.envKey)
			t.Setenv("ADDRESS", "localhost:8080")
			t.Setenv("REPORT_INTERVAL", "10")
			t.Setenv("POLL_INTERVAL", "2")
			t.Setenv("RATE_LIMIT", "")
			cfg, err := parseAgentConfig(flag.NewFlagSet("agent", flag.ContinueOnError), tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Key)
		})
	}
}

func TestAgentConfigRateLimit(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		envRate string
		want    int64
		wantErr bool
	}{
		{name: "default", want: 1},
		{name: "flag", args: []string{"-l=4"}, want: 4},
		{name: "environment", envRate: "3", want: 3},
		{name: "environment overrides flag", args: []string{"-l=4"}, envRate: "2", want: 2},
		{name: "zero", args: []string{"-l=0"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RATE_LIMIT", tt.envRate)
			cfg, err := parseAgentConfig(flag.NewFlagSet("agent", flag.ContinueOnError), tt.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.RateLimit)
		})
	}
}
