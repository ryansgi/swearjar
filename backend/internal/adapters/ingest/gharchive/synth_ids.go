package gharchive

import (
	"crypto/sha256"
	"encoding/json/v2"
	"hash/fnv"
	"net/url"
	"path"
	"strings"

	identdom "swearjar/internal/services/ident/domain"
)

// Synthetic IDs and HIDs: legacy support for missing actor.id and repo.id.
// We're at the whim of github's event shapes; the json schema has drifted over time.
// At one point (first encounterd in 2012 data) actor.id and repo.id were removed.
// They were later restored, but many old events lack them.
// We fill in synthetic negative IDs based on stable hashes of actor.login and repo.name.
// This allows us to maintain referential integrity for historical data, even if the IDs are not real GitHub IDs.
// The synthetic IDs are negative int64 values, so they will never collide with real GitHub IDs (which are positive).
// We also provide functions to compute stable HIDs from login/name, which are used in the ident service

// SyntheticActorID returns a deterministic negative int64 from actor login
func SyntheticActorID(login string) int64 {
	return synthNegID("actor:", strings.ToLower(strings.TrimSpace(login)))
}

// SyntheticRepoIDFromName returns a deterministic negative int64 from "owner/repo"
func SyntheticRepoIDFromName(fullName string) int64 {
	return synthNegID("repo:", CanonRepoName(fullName))
}

// ActorHID32FromLogin makes a stable HID from login (works even when ID=0)
func ActorHID32FromLogin(login string) identdom.HID32 {
	return sha256.Sum256([]byte("actor:" + strings.ToLower(strings.TrimSpace(login))))
}

// RepoHID32FromName makes a stable HID from "owner/repo" (works even when ID=0)
func RepoHID32FromName(fullName string) identdom.HID32 {
	return sha256.Sum256([]byte("repo:" + CanonRepoName(fullName)))
}

// HID32 prefers numeric ID, fallback to strings
func (a Actor) HID32() identdom.HID32 {
	if a.ID != 0 {
		return identdom.ActorHID32(a.ID)
	}
	return ActorHID32FromLogin(a.Login)
}

// HID32 prefers numeric ID, falls back to name-based HID
func (r Repo) HID32() identdom.HID32 {
	if r.ID != 0 {
		return identdom.RepoHID32(r.ID)
	}
	return RepoHID32FromName(r.Name)
}

// FillSyntheticIDs populates Actor.ID/Repo.ID when zero using legacy payload hints
func (e *EventEnvelope) FillSyntheticIDs() {
	// Actor
	if e.Actor.ID == 0 {
		login := strings.ToLower(strings.TrimSpace(e.Actor.Login))
		if login == "" {
			if l, _ := guessLegacyActorRepo(e.RawPayload); l != "" {
				login = l
			}
		}
		if login != "" {
			e.Actor.Login = login
			e.Actor.ID = SyntheticActorID(login)
		}
	}

	// Repo
	if e.Repo.ID == 0 {
		name := CanonRepoName(e.Repo.Name)
		if name == "" {
			if _, rn := guessLegacyActorRepo(e.RawPayload); rn != "" {
				name = rn
			}
		}
		if name != "" {
			e.Repo.Name = name
			e.Repo.ID = SyntheticRepoIDFromName(name)
		}
	}
}

func synthNegID(prefix, key string) int64 {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(prefix))
	_, _ = h.Write([]byte(key))
	v := int64(h.Sum64() & 0x7fffffffffffffff)
	if v == 0 {
		v = 1
	}
	return -v // negative => never collides with real GitHub IDs
}

// CanonRepoName normalizes to "owner/repo" (lowercase), accepts URLs or raw names
func CanonRepoName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")

	// git/https/ssh URLs
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 {
				s = parts[len(parts)-2] + "/" + parts[len(parts)-1]
			}
		}
	} else if strings.Contains(s, ":") && strings.Contains(s, "@") {
		// e.g. git@github.com:owner/repo(.git)
		if i := strings.Index(s, ":"); i >= 0 {
			s = s[i+1:]
		}
	}

	s = strings.TrimSuffix(s, ".git")
	s = strings.ToLower(strings.Trim(s, "/"))

	if s == "" {
		return ""
	}
	parts := strings.Split(s, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return s
}

// guessLegacyActorRepo sniffs old GHArchive shapes: actor/login and repository owner/name/url
func guessLegacyActorRepo(raw []byte) (login string, fullRepo string) {
	if len(raw) == 0 {
		return "", ""
	}

	// minimal shim for old payloads
	var aux struct {
		Actor           string `json:"actor"` // old top-level "actor": "login"
		ActorAttributes struct {
			Login string `json:"login"`
		} `json:"actor_attributes"`
		Repository struct {
			Name  string      `json:"name"`
			Owner interface{} `json:"owner"` // string or object with "login"|"name"
			URL   string      `json:"url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &aux); err != nil {
		return "", ""
	}

	// login
	login = strings.ToLower(strings.TrimSpace(aux.ActorAttributes.Login))
	if login == "" {
		login = strings.ToLower(strings.TrimSpace(aux.Actor))
	}

	// repo owner
	var owner string
	switch v := aux.Repository.Owner.(type) {
	case string:
		owner = v
	case map[string]any:
		if s, ok := v["login"].(string); ok && s != "" {
			owner = s
		} else if s, ok := v["name"].(string); ok && s != "" {
			owner = s
		}
	}
	owner = strings.ToLower(strings.TrimSpace(owner))
	name := strings.ToLower(strings.TrimSpace(aux.Repository.Name))

	// fallback via URL
	if (owner == "" || name == "") && aux.Repository.URL != "" {
		if rr, ok := ownerRepoFromURL(aux.Repository.URL); ok {
			fullRepo = rr
		}
	} else if owner != "" && name != "" {
		fullRepo = owner + "/" + name
	}

	fullRepo = CanonRepoName(fullRepo)
	return login, fullRepo
}

func ownerRepoFromURL(u string) (string, bool) {
	// works for https://github.com/owner/repo and raw path-ish strings
	if parsed, err := url.Parse(u); err == nil && parsed.Host != "" {
		p := strings.Trim(parsed.EscapedPath(), "/")
		parts := strings.Split(p, "/")
		if len(parts) >= 2 {
			return strings.ToLower(path.Join(parts[len(parts)-2], parts[len(parts)-1])), true
		}
	}
	// best effort for non-URL inputs
	u = strings.Trim(u, "/")
	parts := strings.Split(u, "/")
	if len(parts) >= 2 {
		return strings.ToLower(path.Join(parts[len(parts)-2], parts[len(parts)-1])), true
	}
	return "", false
}
