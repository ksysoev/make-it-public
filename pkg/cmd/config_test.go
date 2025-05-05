package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ksysoev/make-it-public/pkg/api"
	"github.com/ksysoev/make-it-public/pkg/edge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	const validConfig = `
http:
  listen: ":8080"
reverse_proxy:
  listen: ":8081"
api:
  listen: ":8082"
`

	tests := []struct {
		envVars      map[string]string
		expectConfig *appConfig
		name         string
		configData   string
		expectError  bool
	}{
		{
			name:        "valid config file",
			envVars:     nil,
			expectError: false,
			configData:  validConfig,
			expectConfig: &appConfig{
				HTTP:     edge.Config{Listen: ":8080"},
				RevProxy: revProxyConfig{Listen: ":8081"},
				API:      api.Config{Listen: ":8082"},
			},
		},
		{
			name:        "missing config file",
			envVars:     nil,
			expectError: true,
		},
		{
			name:        "unparseable config file",
			envVars:     nil,
			expectError: true,
			configData:  `http: "invalid"`,
		},
		{
			name: "valid config with environment overrides",
			envVars: map[string]string{
				"HTTP_LISTEN": ":8083",
			},
			expectError: false,
			configData:  validConfig,
			expectConfig: &appConfig{
				HTTP:     edge.Config{Listen: ":8083"},
				RevProxy: revProxyConfig{Listen: ":8081"},
				API:      api.Config{Listen: ":8082"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if tt.configData != "" {
				err := os.WriteFile(configPath, []byte(tt.configData), 0o600)

				require.NoError(t, err)
			}

			// Set up environment variables
			if tt.envVars != nil {
				for key, value := range tt.envVars {
					_ = os.Setenv(key, value)

					t.Cleanup(func() {
						_ = os.Unsetenv(key)
					})
				}
			}

			arg := &args{
				configPath: configPath,
			}

			cfg, err := loadConfig(arg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectConfig, cfg)
			}
		})
	}
}
