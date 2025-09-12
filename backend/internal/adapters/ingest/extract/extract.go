// Package extract provides functionality to extract utterances from GitHub event payloads
package extract

import (
	json "encoding/json/v2"
	"strings"
	"time"

	"swearjar/internal/adapters/ingest/gharchive"
	"swearjar/internal/core/langhint"
)

// Normalizer is a small seam so we don't depend on your concrete type/method names
type Normalizer interface {
	Normalize(string) string
}

// Utterance is what the detector ingests and the store persists
type Utterance struct {
	UtteranceID    string
	EventType      string
	Repo           string // owner/name
	Actor          string // login
	CreatedAt      time.Time
	Source         string // coarse: commit/issue/pr/comment
	SourceDetail   string // granular: issues:title, pr:body, etc.
	TextRaw        string
	TextNormalized string
	LangCode       string // optional; empty => NULL
	Script         string // optional; empty => NULL
	Ordinal        int
}

// FromEvent extracts utterances from a GitHub event envelope
func FromEvent(env gharchive.EventEnvelope, norm Normalizer) []Utterance {
	var outs []Utterance

	// keep a stable per-source ordinal for this event
	ord := map[string]int{}

	coarseOf := func(detail string) string {
		if i := strings.IndexByte(detail, ':'); i > 0 {
			switch detail[:i] {
			case "push":
				return "commit"
			case "issues", "issue_comment":
				return "issue"
			case "pr", "pr_review_comment":
				return "pr"
			case "commit_comment":
				return "comment"
			}
		}
		// fallback: map known event types to coarse buckets
		switch env.Type {
		case "PushEvent":
			return "commit"
		case "IssuesEvent", "IssueCommentEvent":
			return "issue"
		case "PullRequestEvent", "PullRequestReviewCommentEvent":
			return "pr"
		case "CommitCommentEvent":
			return "comment"
		default:
			return "comment"
		}
	}

	add := func(detail, txt string) {
		t := strings.TrimSpace(txt)
		if t == "" {
			return
		}

		var normed string
		if norm != nil {
			normed = norm.Normalize(t)
		} else {
			normed = t
		}

		script, lang := langhint.DetectScriptAndLang(normed)

		// increment ordinal for this granular source
		ord[detail]++
		ordinal := ord[detail]

		// build deterministic utterance ID from the envelope's raw payload + (detail, ordinal)
		uuid := env.DeterministicUUID(detail, ordinal)
		u := Utterance{
			UtteranceID:    uuid.String(),
			EventType:      env.Type,
			Repo:           env.Repo.Name,
			Actor:          env.Actor.Login,
			CreatedAt:      time.Time(env.CreatedAt),
			Source:         coarseOf(detail), // coarse bucket for enums/analytics
			SourceDetail:   detail,           // granular selector
			TextRaw:        t,
			TextNormalized: normed,
			LangCode:       lang,
			Script:         script,
			Ordinal:        ordinal,
		}

		outs = append(outs, u)
	}

	switch env.Type {
	case "PushEvent":
		var p struct {
			Commits []struct {
				SHA     string `json:"sha"`
				Message string `json:"message"`
			} `json:"commits"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			for _, c := range p.Commits {
				add("push:commit", c.Message)
			}
		}

	case "IssuesEvent":
		var p struct {
			Issue struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"issue"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			add("issues:title", p.Issue.Title)
			add("issues:body", p.Issue.Body)
		}

	case "IssueCommentEvent":
		var p struct {
			Comment struct {
				Body string `json:"body"`
			} `json:"comment"`
			Issue struct {
				Title string `json:"title"`
			} `json:"issue"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			add("issue_comment:title", p.Issue.Title)
			add("issue_comment:body", p.Comment.Body)
		}

	case "PullRequestEvent":
		var p struct {
			PullRequest struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			add("pr:title", p.PullRequest.Title)
			add("pr:body", p.PullRequest.Body)
		}

	case "PullRequestReviewCommentEvent":
		var p struct {
			Comment struct {
				Body string `json:"body"`
			} `json:"comment"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			add("pr_review_comment:body", p.Comment.Body)
		}

	case "CommitCommentEvent":
		var p struct {
			Comment struct {
				Body string `json:"body"`
			} `json:"comment"`
		}
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			add("commit_comment:body", p.Comment.Body)
		}
	}

	return outs
}
