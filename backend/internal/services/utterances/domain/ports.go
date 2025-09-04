package domain

import "context"

// ReaderPort defines the read interface for utterances
type ReaderPort interface {
	// List returns up to Limit rows ordered by (created_at, id), applying governance opt-outs and filters
	List(ctx context.Context, in ListInput) (rows []Row, next AfterKey, err error)
}
