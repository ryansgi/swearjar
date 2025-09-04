package repokit

import "context"

// BeginHook runs at the start of a transaction with the tx bound Queryer
type BeginHook func(ctx context.Context, q Queryer) error

// WithBeginHooks wraps a TxRunner and runs hooks before fn inside the same tx
func WithBeginHooks(inner TxRunner, hooks ...BeginHook) TxRunner {
	return hookedTx{inner: inner, hooks: hooks}
}

type hookedTx struct {
	inner TxRunner
	hooks []BeginHook
}

// Tx starts a tx on inner then runs all hooks before fn
func (h hookedTx) Tx(ctx context.Context, fn func(q Queryer) error) error {
	return h.inner.Tx(ctx, func(q Queryer) error {
		for _, hk := range h.hooks {
			if err := hk(ctx, q); err != nil {
				return err
			}
		}
		return fn(q)
	})
}

// delegate so hookedTx satisfies TxRunner
func (h hookedTx) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	return h.inner.Exec(ctx, sql, args...)
}

func (h hookedTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return h.inner.Query(ctx, sql, args...)
}

func (h hookedTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return h.inner.QueryRow(ctx, sql, args...)
}

// MidHook is a function you call explicitly inside a tx when you need it
type MidHook func(ctx context.Context, q Queryer) error

// RunMidHooks runs the given mid hooks using the tx bound Queryer
func RunMidHooks(ctx context.Context, q Queryer, hooks ...MidHook) error {
	for _, hk := range hooks {
		if err := hk(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
