package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConfigKey(t *testing.T) {
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
			os.Args = append([]string{"server"}, tt.args...)
			flag.CommandLine = flag.NewFlagSet("server", flag.ContinueOnError)
			t.Setenv("KEY", tt.envKey)
			t.Setenv("ADDRESS", ":8080")
			t.Setenv("DATABASE_DSN", "")
			t.Setenv("STORE_INTERVAL", "300")
			t.Setenv("FILE_STORAGE_PATH", "")
			t.Setenv("RESTORE", "true")
			cfg, err := ServerConfig()
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Key)
		})
	}
}
