package server

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

type appConfig struct {
	HTTP     httpConfig     `mapstructure:"http"`
	RevProxy revProxyConfig `mapstructure:"reverse_proxy"`
}

type httpConfig struct {
	Listen string `mapstructure:"listen"`
}

type revProxyConfig struct {
	Listen string `mapstructure:"listen"`
}

func loadConfig(arg *flags) (*appConfig, error) {
	v := viper.New()

	v.SetConfigFile(arg.configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
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
