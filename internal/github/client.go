package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

const maxResponseSize = 5 * 1024 * 1024 // 5 MiB safety cap for HTTP response bodies

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var (
	ghToken     string
	ghTokenOnce sync.Once
)

func getGHToken() string {
	ghTokenOnce.Do(func() {
		ghToken = os.Getenv("GITHUB_TOKEN")
	})
	return ghToken
}

func doGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if token := getGHToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return httpClient.Do(req)
}

func buildRawURL(repo, branch, agentPath, filename string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", repo, branch, agentPath, filename)
}

// escapeRepoPath splits an "owner/repo" string and URL-escapes each segment
// individually so the slash between them is preserved as a path separator.
func escapeRepoPath(repo string) string {
	owner, name, _ := strings.Cut(repo, "/")
	return url.PathEscape(owner) + "/" + url.PathEscape(name)
}

// readLimitedBody reads up to maxResponseSize bytes from r and returns an
// explicit error if the response exceeds the cap (instead of silently truncating).
func readLimitedBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("response exceeds max size of %d bytes", maxResponseSize)
	}
	return body, nil
}

// ErrNotFound is returned when a requested file does not exist (HTTP 404).
var ErrNotFound = fmt.Errorf("not found")

// ValidateBranch checks that a branch exists in the given GitHub repository.
// Returns the resolved commit SHA on success (safe for use in raw URLs even
// when the branch name contains slashes), or an error if the branch does not exist.
func ValidateBranch(repo, branch string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/branches/%s", escapeRepoPath(repo), url.PathEscape(branch))

	resp, err := doGet(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to validate branch %q: %v", branch, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("branch %q does not exist in repository %q", branch, repo)
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("unexpected status %d when validating branch %q in %q", resp.StatusCode, branch, repo)
	}

	body, err := readLimitedBody(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read branch response for %q: %v", branch, err)
	}

	var branchInfo struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &branchInfo); err != nil {
		return "", fmt.Errorf("failed to parse branch response for %q: %v", branch, err)
	}
	if branchInfo.Commit.SHA == "" {
		return "", fmt.Errorf("branch %q in %q has no commit SHA", branch, repo)
	}

	return branchInfo.Commit.SHA, nil
}

// FetchAgentYAML fetches and parses an agent.yaml file from a GitHub repository.
func FetchAgentYAML(repo, branch, agentPath string) (*types.UpstreamAgentYAML, error) {
	url := buildRawURL(repo, branch, agentPath, "agent.yaml")

	resp, err := doGet(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed for %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := readLimitedBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from %s: %v", url, err)
	}

	var agent types.UpstreamAgentYAML
	if err := yaml.Unmarshal(body, &agent); err != nil {
		return nil, fmt.Errorf("error parsing agent.yaml from %s: %v", agentPath, err)
	}

	// Capture fields not in the struct as Extra for customProperties forwarding.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(body, &raw); err == nil {
		extra := make(map[string]interface{})
		for k, v := range raw {
			if !types.KnownUpstreamFields[k] {
				extra[k] = v
			}
		}
		if len(extra) > 0 {
			agent.Extra = extra
		}
	}

	return &agent, nil
}

// FetchReadme fetches the README.md content from a GitHub repository path.
// Returns empty string (not error) when README is not found (404).
func FetchReadme(repo, branch, agentPath string) (string, error) {
	url := buildRawURL(repo, branch, agentPath, "README.md")

	resp, err := doGet(url)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed for %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := readLimitedBody(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response from %s: %v", url, err)
	}

	return string(body), nil
}
