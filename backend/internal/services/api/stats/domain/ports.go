package domain

import "context"

// ServicePort is consumed by handlers and other modules
type ServicePort interface {
	ByLang(ctx context.Context, in ByLangInput) ([]ByLangRow, error)
	ByRepo(ctx context.Context, in ByRepoInput) ([]ByRepoRow, error)
	ByCategory(ctx context.Context, in ByCategoryInput) ([]ByCategoryRow, error)
}
