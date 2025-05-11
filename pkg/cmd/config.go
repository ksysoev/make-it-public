package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ksysoev/make-it-public/pkg/api"
	"github.com/ksysoev/make-it-public/pkg/edge"
	"github.com/ksysoev/make-it-public/pkg/metric"
	"github.com/ksysoev/make-it-public/pkg/repo/auth"
	"github.com/spf13/viper"
)

type appConfig struct {
	Auth     auth.Config    `mapstructure:"auth"`
	API      api.Config     `mapstructure:"api"`
	Metrics  metric.Config  `mapstructure:"metrics"`
	RevProxy revProxyConfig `mapstructure:"reverse_proxy"`
	HTTP     edge.Config    `mapstructure:"http"`
}

type revProxyConfig struct {
	Listen string `mapstructure:"listen"`
}

// loadConfig loads the application configuration from the specified file path and environment variables.
// It uses the provided args structure to determine the configuration path.
// The function returns a pointer to the appConfig structure and an error if something goes wrong.
func loadConfig(arg *args) (*appConfig, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())

	if arg.ConfigPath != "" {
		v.SetConfigFile(arg.ConfigPath)

		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var cfg appConfig

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	slog.Debug("Config loaded", slog.Any("config", cfg))

	return &cfg, nil
}
