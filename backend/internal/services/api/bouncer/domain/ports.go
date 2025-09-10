package domain

import "context"

// ServicePort is the interface implemented by the bouncer service
type ServicePort interface {
	Issue(ctx context.Context, in IssueInput) (IssueOutput, error)
	Reverify(ctx context.Context, in ReverifyInput) (StatusRow, error)
	Status(ctx context.Context, in StatusQuery) (StatusRow, error)
}
