package github

import "time"

// Repo is a partial GitHub repository document with fields we use
type Repo struct {
	ID            int64     `json:"id"`
	NodeID        string    `json:"node_id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Private       bool      `json:"private"`
	Owner         User      `json:"owner"`
	DefaultBranch string    `json:"default_branch"`
	Language      string    `json:"language"`
	ForksCount    int       `json:"forks_count"`
	Stargazers    int       `json:"stargazers_count"`
	Subscribers   int       `json:"subscribers_count"`
	OpenIssues    int       `json:"open_issues_count"`
	License       *License  `json:"license"`
	Fork          bool      `json:"fork"`
	PushedAt      time.Time `json:"pushed_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	HTMLURL       string    `json:"html_url"`
	APIURL        string    `json:"url"`
}

// License is a partial GitHub license document
type License struct {
	Key string `json:"key"`
}

// User is a partial GitHub user or org document
type User struct {
	ID          int64     `json:"id"`
	Login       string    `json:"login"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Company     string    `json:"company"`
	Location    string    `json:"location"`
	Bio         string    `json:"bio"`
	Blog        string    `json:"blog"`
	Twitter     string    `json:"twitter_username"`
	Followers   int       `json:"followers"`
	Following   int       `json:"following"`
	PublicRepos int       `json:"public_repos"`
	PublicGists int       `json:"public_gists"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	HTMLURL     string    `json:"html_url"`
	APIURL      string    `json:"url"`
}
