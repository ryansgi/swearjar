package store

import "time"

// Config aggregates per backend configuration
type Config struct {
	AppName string

	PG   PGConfig
	CH   CHConfig
	NATS NATSConfig
	RDS  RedisConfig
}

// PGConfig configures postgres connectivity and tracing
type PGConfig struct {
	Enabled     bool
	URL         string
	MaxConns    int32
	LogSQL      bool
	SlowQueryMs int

	// Guard/boot knobs:
	ConnectRetries int           // default 6 (63s(ish) max with exponential backoff)
	PingTimeout    time.Duration // default 5s
}

// CHConfig configures clickhouse connectivity
type CHConfig struct {
	Enabled bool
	URL     string
}

// NATSConfig configures nats connectivity
type NATSConfig struct {
	Enabled   bool
	URL       string
	JetStream bool
}

// RedisConfig configures redis connectivity
type RedisConfig struct {
	Enabled bool
	Addr    string
	DB      int
}
