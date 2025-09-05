package service

import (
	"context"
	"time"
)

// SeenRepo enqueues a repository sighting without blocking ingest
func (s *Svc) SeenRepo(ctx context.Context, repoID int64, fullName string, seenAt time.Time) error {
	return s.Repo.SeenRepo(ctx, repoID, fullName, seenAt)
}

// SeenActor enqueues an actor sighting without blocking ingest
func (s *Svc) SeenActor(ctx context.Context, actorID int64, login string, seenAt time.Time) error {
	return s.Repo.SeenActor(ctx, actorID, login, seenAt)
}
