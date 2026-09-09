package config

import (
	"flag"
	"os"
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
			oldArgs, oldFlags := os.Args, flag.CommandLine
			t.Cleanup(func() { os.Args, flag.CommandLine = oldArgs, oldFlags })
			os.Args = append([]string{"agent"}, tt.args...)
			flag.CommandLine = flag.NewFlagSet("agent", flag.ContinueOnError)
			t.Setenv("KEY", tt.envKey)
			t.Setenv("ADDRESS", "localhost:8080")
			t.Setenv("REPORT_INTERVAL", "10")
			t.Setenv("POLL_INTERVAL", "2")
			cfg, err := AgentConfig()
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Key)
		})
	}
}
