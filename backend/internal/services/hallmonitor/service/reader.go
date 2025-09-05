package service

import (
	"context"

	"swearjar/internal/services/hallmonitor/domain"
)

// PrimaryLanguageOfRepo delegates to repo
func (s *Svc) PrimaryLanguageOfRepo(ctx context.Context, repoID int64) (string, bool, error) {
	return s.Repo.PrimaryLanguageOfRepo(ctx, repoID)
}

// LanguagesOfRepo delegates to repo
func (s *Svc) LanguagesOfRepo(ctx context.Context, repoID int64) (map[string]int64, bool, error) {
	return s.Repo.LanguagesOfRepo(ctx, repoID)
}

// PrimaryLanguageOfActor delegates to repo
func (s *Svc) PrimaryLanguageOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (string, bool, error) {
	return s.Repo.PrimaryLanguageOfActor(ctx, actorID, w)
}

// LanguagesOfActor delegates to repo
func (s *Svc) LanguagesOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (map[string]int64, error) {
	return s.Repo.LanguagesOfActor(ctx, actorID, w)
}

// PrimaryLanguageOfActorHID delegates to repo
func (s *Svc) PrimaryLanguageOfActorHID(
	ctx context.Context,
	actorHID []byte,
	w domain.LangWindow,
) (string, bool, error) {
	return s.Repo.PrimaryLanguageOfActorHID(ctx, actorHID, w)
}

// LanguagesOfActorHID delegates to repo
func (s *Svc) LanguagesOfActorHID(ctx context.Context, actorHID []byte, w domain.LangWindow) (map[string]int64, error) {
	return s.Repo.LanguagesOfActorHID(ctx, actorHID, w)
}
