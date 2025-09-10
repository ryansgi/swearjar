// Package service provides the ident service implementation
package service

import (
	"context"
	"errors"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/ident/domain"
)

// Reusable error when resolver isn't wired yet
var errResolverNotImplemented = errors.New("ident.ResolverPort not implemented")

// Svc implements the ident upserter (and later, the resolver)
type Svc struct {
	db     repokit.TxRunner
	binder repokit.Binder[domain.Repo]
}

// New constructs the ident service
func New(db repokit.TxRunner, binder repokit.Binder[domain.Repo]) *Svc {
	if db == nil {
		panic("ident.Service requires a non-nil TxRunner")
	}
	if binder == nil {
		panic("ident.Service requires a non-nil Repo binder")
	}
	return &Svc{db: db, binder: binder}
}

// EnsurePrincipalsAndMaps ensures principals and GH maps exist for the given repos and actors
func (s *Svc) EnsurePrincipalsAndMaps(
	ctx context.Context,
	repos map[domain.HID32]int64, actors map[domain.HID32]int64,
) error {
	if len(repos) == 0 && len(actors) == 0 {
		return nil
	}
	// Single tx to guarantee temp-table connection affinity
	return s.db.Tx(ctx, func(q repokit.Queryer) error {
		return s.binder.Bind(q).EnsurePrincipalsAndMaps(ctx, repos, actors)
	})
}

// ActorHID is intentionally not implemented yet
func (s *Svc) ActorHID(ctx context.Context, login string) (domain.HID, bool, error) {
	return nil, false, errResolverNotImplemented
}

// RepoHID is intentionally not implemented yet
func (s *Svc) RepoHID(ctx context.Context, resource string) (domain.HID, bool, error) {
	return nil, false, errResolverNotImplemented
}
