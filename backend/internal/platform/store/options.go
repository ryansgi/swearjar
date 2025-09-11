package store

import "github.com/rs/zerolog"

type options struct {
	log *zerolog.Logger
}

// Option customizes store behavior
type Option func(*options)

func buildOptions(opts ...Option) *options {
	o := &options{}
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithLogger sets a logger to use inside the store package
func WithLogger(l zerolog.Logger) Option {
	return func(o *options) { o.log = &l }
}
