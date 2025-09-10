package service

import (
	"context"
)

// EvidenceProbe abstracts "does the artifact exist?" checks
type EvidenceProbe interface {
	DefaultBranch(ctx context.Context, ownerRepo string) (string, error)
	RepoFile(ctx context.Context, ownerRepo, defaultBranch, filename string) (bool, string, error)
	GistFile(ctx context.Context, login, filename string) (bool, string, error)
}
