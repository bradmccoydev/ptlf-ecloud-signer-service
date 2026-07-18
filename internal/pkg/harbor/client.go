package harbor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client provides access to the Harbor registry API.
type Client interface {
	GetScanReport(ctx context.Context, project, repo, digest string) (*ScanReport, error)
	ListArtifacts(ctx context.Context, project string) ([]Artifact, error)
	HasSignature(ctx context.Context, project, repo, digest string) (bool, error)
}

// ClientConfig holds configuration for the Harbor API client.
type ClientConfig struct {
	BaseURL        string
	Username       string
	Password       string
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

// DefaultClient implements the Client interface for Harbor API interactions.
type DefaultClient struct {
	baseURL        string
	username       string
	password       string
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

// NewClient creates a new Harbor API client with the given configuration.
func NewClient(cfg ClientConfig) *DefaultClient {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryBaseDelay := cfg.RetryBaseDelay
	if retryBaseDelay <= 0 {
		retryBaseDelay = 1 * time.Second
	}
	retryMaxDelay := cfg.RetryMaxDelay
	if retryMaxDelay <= 0 {
		retryMaxDelay = 10 * time.Second
	}

	return &DefaultClient{
		baseURL:        strings.TrimRight(cfg.BaseURL, "/"),
		username:       cfg.Username,
		password:       cfg.Password,
		httpClient:     &http.Client{},
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
		retryMaxDelay:  retryMaxDelay,
	}
}

// GetScanReport retrieves the vulnerability scan report for a specific artifact.
// It uses a 30-second timeout per attempt and retries transient failures.
func (c *DefaultClient) GetScanReport(ctx context.Context, project, repo, digest string) (*ScanReport, error) {
	encodedRepo := url.PathEscape(repo)
	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s/additions/vulnerabilities",
		c.baseURL, project, encodedRepo, digest)

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// 30-second timeout per attempt
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		body, statusCode, err := c.doRequest(attemptCtx, http.MethodGet, endpoint)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt+1, c.maxRetries, err)
			continue
		}

		// Don't retry client errors (4xx)
		if statusCode >= 400 && statusCode < 500 {
			return nil, fmt.Errorf("harbor API returned client error %d for scan report", statusCode)
		}

		// Retry server errors (5xx)
		if statusCode >= 500 {
			lastErr = fmt.Errorf("harbor API returned server error %d (attempt %d/%d)", statusCode, attempt+1, c.maxRetries)
			continue
		}

		// Harbor returns a map where the key is the scanner name and value is the scan report.
		// Parse and extract the first (usually only) report.
		var reportMap map[string]*ScanReport
		if err := json.Unmarshal(body, &reportMap); err != nil {
			return nil, fmt.Errorf("failed to parse scan report response: %w", err)
		}

		// Extract the first report from the map
		for _, report := range reportMap {
			if report != nil {
				return report, nil
			}
		}

		return nil, fmt.Errorf("no scan report found in response for %s/%s@%s", project, repo, digest)
	}

	return nil, fmt.Errorf("failed to get scan report after %d attempts: %w", c.maxRetries, lastErr)
}

// ListArtifacts lists all artifacts in a project with scan status "Success" for reconciliation.
// It first lists repositories in the project, then fetches artifacts for each repository.
func (c *DefaultClient) ListArtifacts(ctx context.Context, project string) ([]Artifact, error) {
	repos, err := c.listRepositories(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for project %s: %w", project, err)
	}

	var artifacts []Artifact
	for _, repo := range repos {
		repoArtifacts, err := c.listArtifactsForRepo(ctx, project, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to list artifacts for repo %s/%s: %w", project, repo, err)
		}
		artifacts = append(artifacts, repoArtifacts...)
	}

	return artifacts, nil
}

// HasSignature checks whether the given artifact already has a cosign signature attached.
// Uses a 5-second timeout.
func (c *DefaultClient) HasSignature(ctx context.Context, project, repo, digest string) (bool, error) {
	encodedRepo := url.PathEscape(repo)
	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s/accessories",
		c.baseURL, project, encodedRepo, digest)

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(delay):
			}
		}

		// 5-second timeout for signature checks
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		body, statusCode, err := c.doRequest(attemptCtx, http.MethodGet, endpoint)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt+1, c.maxRetries, err)
			continue
		}

		// Don't retry client errors (4xx)
		if statusCode >= 400 && statusCode < 500 {
			return false, fmt.Errorf("harbor API returned client error %d for signature check", statusCode)
		}

		// Retry server errors (5xx)
		if statusCode >= 500 {
			lastErr = fmt.Errorf("harbor API returned server error %d (attempt %d/%d)", statusCode, attempt+1, c.maxRetries)
			continue
		}

		var accessories []accessory
		if err := json.Unmarshal(body, &accessories); err != nil {
			return false, fmt.Errorf("failed to parse accessories response: %w", err)
		}

		for _, acc := range accessories {
			if acc.Type == "signature.cosign" {
				return true, nil
			}
		}

		return false, nil
	}

	return false, fmt.Errorf("failed to check signature after %d attempts: %w", c.maxRetries, lastErr)
}

// accessory represents an accessory (e.g., signature) attached to an artifact in Harbor.
type accessory struct {
	Type   string `json:"type"`
	Digest string `json:"digest"`
}

// repository represents a repository returned by the Harbor API.
type repository struct {
	Name string `json:"name"`
}

// harborArtifact represents an artifact as returned by the Harbor list artifacts API.
type harborArtifact struct {
	Digest       string                 `json:"digest"`
	Tags         []Tag                  `json:"tags"`
	ScanOverview map[string]scanSummary `json:"scan_overview"`
	ProjectID    int64                  `json:"project_id"`
}

// scanSummary represents the scan overview status for an artifact.
type scanSummary struct {
	ScanStatus string `json:"scan_status"`
}

// listRepositories fetches all repositories in a given project.
func (c *DefaultClient) listRepositories(ctx context.Context, project string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories?page_size=100",
		c.baseURL, project)

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		body, statusCode, err := c.doRequest(attemptCtx, http.MethodGet, endpoint)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt+1, c.maxRetries, err)
			continue
		}

		if statusCode >= 400 && statusCode < 500 {
			return nil, fmt.Errorf("harbor API returned client error %d listing repositories", statusCode)
		}

		if statusCode >= 500 {
			lastErr = fmt.Errorf("harbor API returned server error %d (attempt %d/%d)", statusCode, attempt+1, c.maxRetries)
			continue
		}

		var repos []repository
		if err := json.Unmarshal(body, &repos); err != nil {
			return nil, fmt.Errorf("failed to parse repositories response: %w", err)
		}

		names := make([]string, 0, len(repos))
		for _, r := range repos {
			// Harbor returns repo names as "project/reponame"; strip the project prefix
			name := r.Name
			prefix := project + "/"
			if strings.HasPrefix(name, prefix) {
				name = strings.TrimPrefix(name, prefix)
			}
			names = append(names, name)
		}

		return names, nil
	}

	return nil, fmt.Errorf("failed to list repositories after %d attempts: %w", c.maxRetries, lastErr)
}

// listArtifactsForRepo fetches artifacts for a specific repository, filtering to those
// with scan status "Success".
func (c *DefaultClient) listArtifactsForRepo(ctx context.Context, project, repo string) ([]Artifact, error) {
	encodedRepo := url.PathEscape(repo)
	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts?with_scan_overview=true&page_size=100",
		c.baseURL, project, encodedRepo)

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		body, statusCode, err := c.doRequest(attemptCtx, http.MethodGet, endpoint)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt+1, c.maxRetries, err)
			continue
		}

		if statusCode >= 400 && statusCode < 500 {
			return nil, fmt.Errorf("harbor API returned client error %d listing artifacts", statusCode)
		}

		if statusCode >= 500 {
			lastErr = fmt.Errorf("harbor API returned server error %d (attempt %d/%d)", statusCode, attempt+1, c.maxRetries)
			continue
		}

		var harborArtifacts []harborArtifact
		if err := json.Unmarshal(body, &harborArtifacts); err != nil {
			return nil, fmt.Errorf("failed to parse artifacts response: %w", err)
		}

		// Filter to artifacts with scan status "Success"
		var result []Artifact
		for _, ha := range harborArtifacts {
			if hasScanSuccess(ha.ScanOverview) {
				result = append(result, Artifact{
					Digest:     ha.Digest,
					Tags:       ha.Tags,
					ScanStatus: "Success",
					ProjectID:  ha.ProjectID,
					Repo:       repo,
				})
			}
		}

		return result, nil
	}

	return nil, fmt.Errorf("failed to list artifacts after %d attempts: %w", c.maxRetries, lastErr)
}

// hasScanSuccess checks whether any scanner in the overview reports status "Success".
func hasScanSuccess(overview map[string]scanSummary) bool {
	for _, summary := range overview {
		if summary.ScanStatus == "Success" {
			return true
		}
	}
	return false
}

// doRequest performs an HTTP request with basic auth and returns the response body.
func (c *DefaultClient) doRequest(ctx context.Context, method, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, resp.StatusCode, nil
}

// calculateBackoff returns the delay for a given retry attempt using exponential
// backoff with jitter, capped at retryMaxDelay.
func (c *DefaultClient) calculateBackoff(attempt int) time.Duration {
	// Exponential: base * 2^(attempt-1)
	backoff := float64(c.retryBaseDelay) * math.Pow(2, float64(attempt-1))
	if backoff > float64(c.retryMaxDelay) {
		backoff = float64(c.retryMaxDelay)
	}

	// Add jitter: random value between 0 and backoff
	jitter := rand.Float64() * backoff //nolint:gosec // jitter does not need crypto rand
	return time.Duration(jitter)
}
