package module

import "sync"

// simple global registry for cross wiring ports during bootstrap in main
// safe for tests and single process composition
var (
	mu  sync.RWMutex
	reg = map[string]any{}
)

// Register stores a port set for a module name
func Register(name string, ports any) {
	mu.Lock()
	reg[name] = ports
	mu.Unlock()
}

// PortsAs fetches and type asserts a port set for name
func PortsAs[T any](name string) (T, bool) {
	mu.RLock()
	v, ok := reg[name]
	mu.RUnlock()
	if !ok {
		var zero T
		return zero, false
	}
	out, ok := v.(T)
	return out, ok
}

// Reset clears the registry for tests
func Reset() {
	mu.Lock()
	reg = map[string]any{}
	mu.Unlock()
}
