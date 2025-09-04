package domain

import "context"

// WriterPort writes hits
type WriterPort interface {
	WriteBatch(ctx context.Context, xs []HitWrite) error
}

// QueryPort queries hits, samples, and aggregations
type QueryPort interface {
	ListSamples(
		ctx context.Context,
		w Window,
		f Filters,
		after AfterKey,
		limit int,
	) (rows []Sample, next AfterKey, err error)
	AggByLang(ctx context.Context, w Window, f Filters) ([]AggByLangRow, error)
	AggByRepo(ctx context.Context, w Window, f Filters, limit int) ([]AggByRepoRow, error)
	AggByCategory(ctx context.Context, w Window, f Filters) ([]AggByCategoryRow, error)
}
