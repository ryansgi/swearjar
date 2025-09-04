package store

import (
	"swearjar/internal/platform/logger"
)

// Option mutates Store during Open
type Option func(*Store) error

// WithLogger sets the logger used by subclients
func WithLogger(log logger.Logger) Option {
	return func(s *Store) error {
		s.Log = log
		return nil
	}
}
