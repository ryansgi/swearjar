package repo

import "context"

// HIDResolver resolves principal HIDs from natural keys
type HIDResolver interface {
	RepoHIDByResource(ctx context.Context, resource string) ([]byte, bool, error)
	ActorHIDByLogin(ctx context.Context, login string) ([]byte, bool, error)
}
