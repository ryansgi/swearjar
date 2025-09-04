package http

import (
	"context"
	stdhttp "net/http"
	"time"

	"swearjar/internal/platform/config"
	"swearjar/internal/platform/logger"

	"github.com/go-chi/chi/v5"
)

// Server is a thin wrapper over chi + stdlib http.Server
type Server struct {
	addr string
	mux  *chi.Mux
	srv  *stdhttp.Server
}

// NewServer creates a zero-value friendly http server
// opts receive the *chi.Mux so callers can mount routes/mw
func NewServer(cfg config.Conf, opts ...func(*chi.Mux)) *Server {
	addr := cfg.MayString("API_PORT", ":4000")
	m := chi.NewRouter()
	for _, o := range opts {
		o(m)
	}
	return &Server{
		addr: addr,
		mux:  m,
		srv: &stdhttp.Server{
			Addr:              addr,
			Handler:           m,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Router returns a Router facade over the internal chi mux
func (s *Server) Router() Router {
	return AdaptChi(s.mux)
}

// Addr returns the listening address
func (s *Server) Addr() string { return s.addr }

// Run starts the server and blocks
func (s *Server) Run(ctx context.Context) error {
	log := logger.Named("http")
	log.Info().Str("addr", s.addr).Msg("http listening")
	err := s.srv.ListenAndServe()
	if err == stdhttp.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown stops the server gracefully
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
