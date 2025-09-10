package module

import (
	"context"
	"fmt"
	"strings"

	identdom "swearjar/internal/services/ident/domain"
)

// IdentityLookup is the small surface we need from GitHub for ID lookup.
// Implement this on top of your existing GH client (see module.go for wiring)
type IdentityLookup interface {
	RepoID(ctx context.Context, fullName string) (int64, error) // "owner/repo" -> numeric repo id
	ActorID(ctx context.Context, login string) (int64, error)   // "login" -> numeric user/org id
}

// resolverAdapter implements the bouncer PrincipalResolver contract.
// It does NOT read public identity tables; instead it:
//   - asks GitHub for the numeric id,
//   - computes the canonical HID,
//   - ensures principals + GH maps via ident.UpserterPort,
//   - returns the HID bytes
type resolverAdapter struct {
	gh    IdentityLookup
	ident identdom.UpserterPort
}

func newResolver(gh IdentityLookup, ident identdom.UpserterPort) *resolverAdapter {
	if gh == nil {
		panic("bouncer: resolver requires a non-nil IdentityLookup")
	}
	if ident == nil {
		panic("bouncer: resolver requires a non-nil ident.UpserterPort")
	}
	return &resolverAdapter{gh: gh, ident: ident}
}

func (r *resolverAdapter) RepoHID(ctx context.Context, resource string) ([]byte, bool, error) {
	full := strings.TrimSpace(resource)
	if full == "" || !strings.Contains(full, "/") {
		return nil, false, fmt.Errorf("invalid repo resource %q (want owner/repo)", resource)
	}

	id, err := r.gh.RepoID(ctx, full)
	if err != nil {
		return nil, false, err
	}

	h := identdom.RepoHID32(id)
	// Ensure principals + gh_repo_map in one shot
	if err := r.ident.EnsurePrincipalsAndMaps(ctx, map[identdom.HID32]int64{h: id}, nil); err != nil {
		return nil, false, err
	}
	return h.Bytes(), true, nil
}

func (r *resolverAdapter) ActorHID(ctx context.Context, login string) ([]byte, bool, error) {
	l := strings.TrimSpace(login)
	if l == "" {
		return nil, false, fmt.Errorf("invalid login")
	}

	id, err := r.gh.ActorID(ctx, l)
	if err != nil {
		return nil, false, err
	}

	h := identdom.ActorHID32(id)
	// Ensure principals + gh_actor_map in one shot
	if err := r.ident.EnsurePrincipalsAndMaps(ctx, nil, map[identdom.HID32]int64{h: id}); err != nil {
		return nil, false, err
	}
	return h.Bytes(), true, nil
}
