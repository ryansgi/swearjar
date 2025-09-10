// Package github provides a resilient GitHub REST v3 client for hallmonitor
package github

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
)

// RepoByID fetches a repository by numeric id with optional etag
func (c *Client) RepoByID(ctx context.Context, id int64, etag string) (Repo, string, bool, error) {
	path := fmt.Sprintf("/repositories/%d", id)
	return c.repoCommon(ctx, path, etag)
}

// RepoByFullName fetches a repository by owner and name with optional etag
func (c *Client) RepoByFullName(ctx context.Context, owner, name, etag string) (Repo, string, bool, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, name)
	return c.repoCommon(ctx, path, etag)
}

func (c *Client) repoCommon(ctx context.Context, path string, etag string) (Repo, string, bool, error) {
	resp, err := c.Do(ctx, http.MethodGet, path, etag)
	if err != nil {
		return Repo{}, "", false, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Error().Err(cerr).Str("path", path).Msg("github close body failed")
		}
	}()

	if resp.StatusCode == http.StatusNotModified {
		return Repo{}, resp.Header.Get("ETag"), true, nil
	}

	var out Repo
	lim := io.LimitReader(resp.Body, 1<<20)
	b, err := io.ReadAll(lim)
	if err != nil {
		return Repo{}, "", false, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Repo{}, "", false, err
	}
	return out, resp.Header.Get("ETag"), false, nil
}

// RepoLanguages fetches the language byte breakdown for a repo
func (c *Client) RepoLanguages(ctx context.Context, owner, name, etag string) (map[string]int64, string, bool, error) {
	path := fmt.Sprintf("/repos/%s/%s/languages", owner, name)
	resp, err := c.Do(ctx, http.MethodGet, path, etag)
	if err != nil {
		return nil, "", false, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Error().Err(cerr).Str("path", path).Msg("github close body failed")
		}
	}()

	if resp.StatusCode == http.StatusNotModified {
		return nil, resp.Header.Get("ETag"), true, nil
	}

	var out map[string]int64
	lim := io.LimitReader(resp.Body, 1<<20)
	b, err := io.ReadAll(lim)
	if err != nil {
		return nil, "", false, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, "", false, err
	}
	return out, resp.Header.Get("ETag"), false, nil
}

// UserByID fetches a user by numeric id with optional etag
func (c *Client) UserByID(ctx context.Context, id int64, etag string) (User, string, bool, error) {
	path := fmt.Sprintf("/user/%d", id)
	return c.userCommon(ctx, path, etag)
}

// UserByLogin fetches a user by login with optional etag
func (c *Client) UserByLogin(ctx context.Context, login, etag string) (User, string, bool, error) {
	path := fmt.Sprintf("/users/%s", login)
	return c.userCommon(ctx, path, etag)
}

func (c *Client) userCommon(ctx context.Context, path, etag string) (User, string, bool, error) {
	resp, err := c.Do(ctx, http.MethodGet, path, etag)
	if err != nil {
		return User{}, "", false, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Error().Err(cerr).Str("path", path).Msg("github close body failed")
		}
	}()

	if resp.StatusCode == http.StatusNotModified {
		return User{}, resp.Header.Get("ETag"), true, nil
	}

	var out User
	lim := io.LimitReader(resp.Body, 1<<20)
	b, err := io.ReadAll(lim)
	if err != nil {
		return User{}, "", false, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return User{}, "", false, err
	}
	return out, resp.Header.Get("ETag"), false, nil
}

// RepoContent returns html_url (when a file) or empty when 404/missing
func (c *Client) RepoContent(ctx context.Context, owner, repo, path, ref, etag string) (string, string, bool, error) {
	p := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
	resp, err := c.Do(ctx, http.MethodGet, p, etag)
	if err != nil {
		return "", "", false, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Error().Err(cerr).Str("path", p).Msg("github close body failed")
		}
	}()
	if resp.StatusCode == http.StatusNotModified {
		return "", resp.Header.Get("ETag"), true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", resp.Header.Get("ETag"), false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", false, &GHStatusError{Status: resp.StatusCode, Err: fmt.Errorf("contents %d", resp.StatusCode)}
	}
	var out struct {
		HTMLURL string `json:"html_url"`
		Type    string `json:"type"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(b, &out); err != nil {
		return "", "", false, err
	}
	if out.Type != "file" && out.Type != "symlink" {
		return "", resp.Header.Get("ETag"), false, nil
	}
	return out.HTMLURL, resp.Header.Get("ETag"), false, nil
}

// ListPublicGists returns a page of gists for a user.
// NOTE (2025): Many callers now receive 401 when unauthenticated.
// We therefore use the authenticated Do() path (Bearer PAT)
func (c *Client) ListPublicGists(
	ctx context.Context,
	user string,
	page, perPage int,
	etag string,
) ([]map[string]any, string, bool, error) {
	p := fmt.Sprintf("/users/%s/gists?per_page=%d&page=%d", user, perPage, page)
	resp, err := c.Do(ctx, http.MethodGet, p, etag)
	if err != nil {
		return nil, "", false, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Error().Err(cerr).Str("path", p).Msg("github close body failed")
		}
	}()
	if resp.StatusCode == http.StatusNotModified {
		return nil, resp.Header.Get("ETag"), true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", false, &GHStatusError{Status: resp.StatusCode, Err: fmt.Errorf("gists %d", resp.StatusCode)}
	}
	var out []map[string]any
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, "", false, err
	}
	return out, resp.Header.Get("ETag"), false, nil
}
