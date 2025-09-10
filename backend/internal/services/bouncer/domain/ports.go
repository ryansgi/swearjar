// Package domain defines core business logic interfaces (ports) and types
package domain

import "context"

// EnqueueArgs holds parameters for enqueuing a verification request
type EnqueueArgs struct {
	Principal     string // "repo" | "actor"
	Resource      string // "owner/repo" or login
	PrincipalHID  []byte
	ChallengeHash string
	EvidenceKind  string // repo_file | actor_gist
	ArtifactHint  string
}

// EnqueuePort enqueues verification requests for processing
type EnqueuePort interface {
	EnqueueVerification(ctx context.Context, args EnqueueArgs) error
}

// WorkerPort (run loop) is separate
type WorkerPort interface {
	Run(ctx context.Context) error
}
