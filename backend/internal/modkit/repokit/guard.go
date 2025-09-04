package repokit

import (
	"context"
	"fmt"
	"time"
)

type guarder interface {
	Guard(context.Context) error
}

// MustPing panics if a dependency doesn't answer a Ping within timeout
func MustPing(ctx context.Context, name string, p interface{ Ping(context.Context) error }) {
	if p == nil {
		panic(fmt.Sprintf("%s: nil dependency", name))
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	if err := p.Ping(ctx); err != nil {
		panic(fmt.Sprintf("%s ping failed: %v", name, err))
	}
}

// MustGuard runs store.Guard and panics on any error (nice for service startup)
func MustGuard(ctx context.Context, st guarder) {
	if err := st.Guard(ctx); err != nil {
		panic(fmt.Errorf("dependency guard failed: %w", err))
	}
}
