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
