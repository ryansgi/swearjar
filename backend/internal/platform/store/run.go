package store

import "context"

// RunInTenant wraps ctx with tenant and calls fn inside the provided TxRunner
func RunInTenant(ctx context.Context, tx TxRunner, tenantID string, fn func(ctx context.Context, q RowQuerier) error) error {
	ctx = WithTenant(ctx, tenantID)
	return tx.Tx(ctx, func(q RowQuerier) error {
		return fn(ctx, q)
	})
}

// RunAsSuperadmin wraps ctx as superadmin and calls fn inside the provided TxRunner
func RunAsSuperadmin(ctx context.Context, tx TxRunner, fn func(ctx context.Context, q RowQuerier) error) error {
	ctx = WithSuperadmin(ctx)
	return tx.Tx(ctx, func(q RowQuerier) error {
		return fn(ctx, q)
	})
}
