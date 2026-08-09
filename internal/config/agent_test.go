package config

import (
	"flag"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAgentConfig_FlagsAndEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		environ  map[string]string
		expected AgentSettings
	}{
		{
			name: "address from environment and intervals from flags",
			args: []string{"agent", "-a", "flag-address:8080", "-r", "15", "-p", "4"},
			environ: map[string]string{
				"ADDRESS": "env-address:9090",
			},
			expected: AgentSettings{
				SrvAdr:         "env-address:9090",
				ReportInterval: 15 * time.Second,
				PollInterval:   4 * time.Second,
			},
		},
		{
			name: "report interval from environment and other values from flags",
			args: []string{"agent", "-a", "flag-address:8081", "-r", "16", "-p", "5"},
			environ: map[string]string{
				"REPORT_INTERVAL": "30s",
			},
			expected: AgentSettings{
				SrvAdr:         "flag-address:8081",
				ReportInterval: 30 * time.Second,
				PollInterval:   5 * time.Second,
			},
		},
		{
			name: "poll interval and address from environment and report interval from flag",
			args: []string{"agent", "-a", "flag-address:8082", "-r", "17", "-p", "6"},
			environ: map[string]string{
				"ADDRESS":       "env-address:9092",
				"POLL_INTERVAL": "9s",
			},
			expected: AgentSettings{
				SrvAdr:         "env-address:9092",
				ReportInterval: 17 * time.Second,
				PollInterval:   9 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAgentEnvironment(t)
			for name, value := range tt.environ {
				t.Setenv(name, value)
			}

			originalCommandLine := flag.CommandLine
			originalArgs := os.Args
			t.Cleanup(func() {
				flag.CommandLine = originalCommandLine
				os.Args = originalArgs
			})

			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = tt.args

			assert.Equal(t, tt.expected, AgentConfig())
		})
	}
}

func clearAgentEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{"ADDRESS", "REPORT_INTERVAL", "POLL_INTERVAL"} {
		value, exists := os.LookupEnv(name)
		if exists {
			t.Cleanup(func() {
				_ = os.Setenv(name, value)
			})
		} else {
			t.Cleanup(func() {
				_ = os.Unsetenv(name)
			})
		}
		assert.NoError(t, os.Unsetenv(name))
	}
}
