package metric

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Listen string `mapstructure:"listen"`
}

type Server struct {
	config Config
}

func New(cfg Config) *Server {
	return &Server{
		config: cfg,
	}
}

// Runs the Prometheus metrics server
func (s *Server) Run(ctx context.Context) error {
	router := http.NewServeMux()

	router.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)

	server := &http.Server{
		Addr:              s.config.Listen,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		Handler:           router,
	}

	go func() {
		<-ctx.Done()

		_ = server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}
