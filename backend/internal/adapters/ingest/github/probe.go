package github

import (
	"context"
	"strings"
)

// Probe implements bouncer's EvidenceProbe against the GitHub REST v3 API
type Probe struct{ c *Client }

// NewProbe constructs a Probe using the given GitHub client.
// (Name chosen to avoid confusion with NewClient.)
func NewProbe(c *Client) *Probe { return &Probe{c: c} }

// DefaultBranch performs GET /repos/{owner}/{repo}: Repo.DefaultBranch
func (p *Probe) DefaultBranch(ctx context.Context, ownerRepo string) (string, error) {
	owner, repo, _ := strings.Cut(ownerRepo, "/")
	r, _, _, err := p.c.RepoByFullName(ctx, owner, repo, "")
	if err != nil {
		return "", err
	}
	return r.DefaultBranch, nil
}

// RepoFile performs GET /repos/{owner}/{repo}/contents/{path}?ref={branch}
// Returns (exists, html_url, err)
func (p *Probe) RepoFile(ctx context.Context, ownerRepo, defaultBranch, filename string) (bool, string, error) {
	owner, repo, _ := strings.Cut(ownerRepo, "/")
	html, _, _, err := p.c.RepoContent(ctx, owner, repo, filename, defaultBranch, "")
	if err != nil {
		return false, "", err
	}
	if html == "" {
		return false, "", nil
	}
	return true, html, nil
}

// GistFile iterates public gists until we find a file named `filename`.
// Returns (exists, html_url, err)
func (p *Probe) GistFile(ctx context.Context, login, filename string) (bool, string, error) {
	page := 1
	for {
		items, _, _, err := p.c.ListPublicGists(ctx, login, page, 100, "")
		if err != nil {
			return false, "", err
		}
		if len(items) == 0 {
			return false, "", nil // exhausted
		}
		for _, g := range items {
			// Only public gists
			if pub, _ := g["public"].(bool); !pub {
				continue
			}
			// files is a map[string]any keyed by filename; check key and value
			files, _ := g["files"].(map[string]any)
			if files == nil {
				continue
			}
			if _, ok := files[filename]; ok {
				if url, _ := g["html_url"].(string); url != "" {
					return true, url, nil
				}
				return true, "", nil
			}
			for _, v := range files {
				if m, _ := v.(map[string]any); m != nil {
					if fn, _ := m["filename"].(string); fn == filename {
						if url, _ := g["html_url"].(string); url != "" {
							return true, url, nil
						}
						return true, "", nil
					}
				}
			}
		}
		page++
	}
}
