// Package harbor provides types and client for interacting with the Harbor registry API.
package harbor

// WebhookPayload represents the top-level payload sent by Harbor webhooks.
type WebhookPayload struct {
	Type      string           `json:"type"`
	OccurAt   int64            `json:"occur_at"`
	Operator  string           `json:"operator"`
	EventData WebhookEventData `json:"event_data"`
}

// WebhookEventData contains the resources and repository affected by the event.
type WebhookEventData struct {
	Resources  []WebhookResource `json:"resources"`
	Repository WebhookRepository `json:"repository"`
}

// WebhookResource describes an artifact resource within a webhook event.
type WebhookResource struct {
	Digest       string                  `json:"digest"`
	Tag          string                  `json:"tag"`
	ResourceURL  string                  `json:"resource_url"`
	ScanOverview map[string]ScanOverview `json:"scan_overview"`
}

// WebhookRepository describes the repository that contains the affected artifact.
type WebhookRepository struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	RepoFullName string `json:"repo_full_name"`
	RepoType     string `json:"repo_type"`
}

// ScanOverview provides a summary of the scan result for a resource.
type ScanOverview struct {
	Severity        int `json:"severity"`
	CompletePercent int `json:"complete_percent"`
}

// ScanReport represents the full vulnerability scan report from Harbor.
type ScanReport struct {
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Scanner         Scanner         `json:"scanner"`
}

// Vulnerability represents a single vulnerability finding in a scan report.
type Vulnerability struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	FixVersion  string `json:"fix_version"`
	Description string `json:"description"`
}

// Scanner identifies the scanner that produced the report.
type Scanner struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Artifact represents a container image artifact in Harbor, used during reconciliation.
type Artifact struct {
	Digest     string `json:"digest"`
	Tags       []Tag  `json:"tags"`
	ScanStatus string `json:"scan_overview"`
	ProjectID  int64  `json:"project_id"`
	Repo       string `json:"repo"`
}

// Tag represents a tag applied to an artifact.
type Tag struct {
	Name string `json:"name"`
}
