// Package service contains hallmonitor workflows
package service

import (
	"strings"
	"time"

	gh "swearjar/internal/adapters/ingest/github"
	bls "swearjar/internal/platform/bools"
	num "swearjar/internal/platform/numbers"
	str "swearjar/internal/platform/strings"
	tim "swearjar/internal/platform/time"
	"swearjar/internal/services/hallmonitor/domain"
)

// mapRepoToRecord maps a GitHub repo and languages set to a repository record
func mapRepoToRecord(cc CadenceConfig, r gh.Repo, langs map[string]int64, etag string) domain.RepositoryRecord {
	primary := choosePrimaryLanguage(langs, r.Language)
	return domain.RepositoryRecord{
		RepoID:        r.ID,
		FullName:      str.Ptr(str.EmptyToNil(r.FullName)),
		DefaultBranch: str.Ptr(str.EmptyToNil(r.DefaultBranch)),
		PrimaryLang:   str.Ptr(str.EmptyToNil(primary)),
		Languages:     langs,
		Stars:         num.Ptr(r.Stargazers),
		Forks:         num.Ptr(r.ForksCount),
		Subscribers:   num.Ptr(r.Subscribers),
		OpenIssues:    num.Ptr(r.OpenIssues),
		LicenseKey:    str.Ptr(licenseKeyOf(r)),
		IsFork:        bls.Ptr(r.Fork),
		PushedAt:      tim.Ptr(r.PushedAt),
		UpdatedAt:     tim.Ptr(r.UpdatedAt),
		NextRefreshAt: tim.Ptr(nextRefreshRepo(cc, r)),
		ETag:          str.Ptr(str.EmptyToNil(etag)),
		APIURL:        str.Ptr(str.EmptyToNil(r.APIURL)),
	}
}

// mapUserToActorRecord maps a GitHub user to an actor record
func mapUserToActorRecord(cc CadenceConfig, u gh.User, etag string) domain.ActorRecord {
	return domain.ActorRecord{
		ActorID:       u.ID,
		Login:         str.Ptr(str.EmptyToNil(u.Login)),
		Name:          str.Ptr(str.EmptyToNil(u.Name)),
		Type:          str.Ptr(str.EmptyToNil(u.Type)),
		Company:       str.Ptr(str.EmptyToNil(u.Company)),
		Location:      str.Ptr(str.EmptyToNil(u.Location)),
		Bio:           str.Ptr(str.EmptyToNil(u.Bio)),
		Blog:          str.Ptr(str.EmptyToNil(u.Blog)),
		Twitter:       str.Ptr(str.EmptyToNil(u.Twitter)),
		Followers:     num.Ptr(u.Followers),
		Following:     num.Ptr(u.Following),
		PublicRepos:   num.Ptr(u.PublicRepos),
		PublicGists:   num.Ptr(u.PublicGists),
		CreatedAt:     tim.Ptr(u.CreatedAt),
		UpdatedAt:     tim.Ptr(u.UpdatedAt),
		NextRefreshAt: tim.Ptr(nextRefreshActor(cc, u)),
		ETag:          str.Ptr(str.EmptyToNil(etag)),
		APIURL:        str.Ptr(str.EmptyToNil(u.APIURL)),
	}
}

// choosePrimaryLanguage returns the largest language by bytes or a fallback
func choosePrimaryLanguage(langs map[string]int64, fallback string) string {
	var best string
	var max int64
	for k, v := range langs {
		if v > max {
			best = k
			max = v
		}
	}
	if best != "" {
		return best
	}
	return fallback
}

// nextRefreshRepo computes next refresh for a repo using cadence config
func nextRefreshRepo(cc CadenceConfig, r gh.Repo) time.Time {
	return nextRefreshRepoFromFields(cc, r.Stargazers, r.PushedAt, time.Now().UTC())
}

// nextRefreshRepoFromFields computes repo cadence from stored fields
func nextRefreshRepoFromFields(cc CadenceConfig, stars int, pushedAt time.Time, now time.Time) time.Time {
	var cadence time.Duration
	switch {
	case stars >= cc.RepoHighStars:
		cadence = cc.RepoHighEvery
	case stars >= cc.RepoMidStars:
		cadence = cc.RepoMidEvery
	default:
		cadence = cc.RepoLowEvery
	}
	candidate := now.Add(cadence)

	if !pushedAt.IsZero() {
		soon := pushedAt.Add(cc.RepoPushMin)
		if soon.After(now) && soon.Before(candidate) {
			return soon
		}
	}
	return candidate
}

// nextRefreshActor computes next refresh for an actor using cadence config
func nextRefreshActor(cc CadenceConfig, u gh.User) time.Time {
	return nextRefreshActorFromFields(cc, u.Followers, time.Now().UTC())
}

// nextRefreshActorFromFields computes actor cadence from stored fields
func nextRefreshActorFromFields(cc CadenceConfig, followers int, now time.Time) time.Time {
	if followers >= cc.ActorHighFollowers {
		return now.Add(cc.ActorHighEvery)
	}
	return now.Add(cc.ActorLowEvery)
}

func splitOwnerName(fullName string) (owner, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func licenseKeyOf(r gh.Repo) string {
	if r.License == nil {
		return ""
	}
	return r.License.Key
}
