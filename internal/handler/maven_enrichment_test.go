package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroupInfoEnrichedFields(t *testing.T) {
	// Test that enriched groupInfo serializes correctly
	g := groupInfo{
		GroupID:        "com.example",
		FirstArtifact:  "example-core",
		FirstPublished: "2024-06-15",
		ArtifactCount:  5,
		LastUpdated:    "2026-03-01",
		TotalVersions:  42,
		License:        "Apache-2.0",
		SourceRepo:     "https://github.com/example/example",
		CVECount:       3,
		MaxCVESeverity: "HIGH",
	}

	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded groupInfo
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TotalVersions != 42 {
		t.Errorf("TotalVersions = %d, want 42", decoded.TotalVersions)
	}
	if decoded.License != "Apache-2.0" {
		t.Errorf("License = %q, want %q", decoded.License, "Apache-2.0")
	}
	if decoded.SourceRepo != "https://github.com/example/example" {
		t.Errorf("SourceRepo = %q, want github URL", decoded.SourceRepo)
	}
	if decoded.CVECount != 3 {
		t.Errorf("CVECount = %d, want 3", decoded.CVECount)
	}
	if decoded.MaxCVESeverity != "HIGH" {
		t.Errorf("MaxCVESeverity = %q, want %q", decoded.MaxCVESeverity, "HIGH")
	}
}

func TestGroupInfoBackwardsCompatible(t *testing.T) {
	// Old cached data without enrichment fields should still deserialize
	oldJSON := `{"group_id":"com.old","first_artifact":"old-lib","first_published":"2023-01-01","artifact_count":2,"last_updated":"2023-06-01"}`

	var g groupInfo
	if err := json.Unmarshal([]byte(oldJSON), &g); err != nil {
		t.Fatalf("failed to unmarshal old format: %v", err)
	}

	if g.GroupID != "com.old" {
		t.Errorf("GroupID = %q, want %q", g.GroupID, "com.old")
	}
	// New fields should be zero values
	if g.TotalVersions != 0 {
		t.Errorf("TotalVersions = %d, want 0 (zero value)", g.TotalVersions)
	}
	if g.License != "" {
		t.Errorf("License = %q, want empty", g.License)
	}
	if g.CVECount != 0 {
		t.Errorf("CVECount = %d, want 0", g.CVECount)
	}
}

func TestParseOSVResponse(t *testing.T) {
	// Simulate an OSV response with vulnerabilities
	osvJSON := `{
		"vulns": [
			{
				"id": "GHSA-abc-123",
				"aliases": ["CVE-2024-1234"],
				"database_specific": {"severity": "HIGH"}
			},
			{
				"id": "GHSA-def-456",
				"aliases": ["CVE-2024-5678"],
				"database_specific": {"severity": "CRITICAL"}
			},
			{
				"id": "GHSA-ghi-789",
				"aliases": ["CVE-2024-9012"],
				"database_specific": {"severity": "MODERATE"}
			}
		]
	}`

	var resp osvResponse
	if err := json.Unmarshal([]byte(osvJSON), &resp); err != nil {
		t.Fatalf("failed to parse OSV response: %v", err)
	}

	count, maxSev := extractOSVSummary(resp)

	if count != 3 {
		t.Errorf("CVE count = %d, want 3", count)
	}
	if maxSev != "CRITICAL" {
		t.Errorf("max severity = %q, want %q", maxSev, "CRITICAL")
	}
}

func TestParseOSVResponseEmpty(t *testing.T) {
	// No vulnerabilities
	var resp osvResponse
	json.Unmarshal([]byte(`{}`), &resp)

	count, maxSev := extractOSVSummary(resp)

	if count != 0 {
		t.Errorf("CVE count = %d, want 0", count)
	}
	if maxSev != "" {
		t.Errorf("max severity = %q, want empty", maxSev)
	}
}

func TestParseDepsDevVersionDetail(t *testing.T) {
	detailJSON := `{
		"versionKey": {"system": "MAVEN", "name": "com.example:foo", "version": "1.0.0"},
		"licenses": ["Apache-2.0"],
		"advisoryKeys": [{"id": "GHSA-abc-123"}, {"id": "GHSA-def-456"}],
		"links": [
			{"label": "SOURCE_REPO", "url": "https://github.com/example/foo"},
			{"label": "HOMEPAGE", "url": "https://example.com"}
		]
	}`

	var detail depsDevVersionDetail
	if err := json.Unmarshal([]byte(detailJSON), &detail); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(detail.Licenses) != 1 || detail.Licenses[0] != "Apache-2.0" {
		t.Errorf("licenses = %v, want [Apache-2.0]", detail.Licenses)
	}
	if len(detail.AdvisoryKeys) != 2 {
		t.Errorf("advisory count = %d, want 2", len(detail.AdvisoryKeys))
	}

	repo := extractSourceRepo(detail)
	if repo != "https://github.com/example/foo" {
		t.Errorf("source repo = %q, want github URL", repo)
	}
}

func TestParseDepsDevVersionDetailNoRepo(t *testing.T) {
	detailJSON := `{
		"versionKey": {"system": "MAVEN", "name": "com.example:bar", "version": "1.0.0"},
		"licenses": [],
		"advisoryKeys": [],
		"links": [{"label": "HOMEPAGE", "url": "https://example.com"}]
	}`

	var detail depsDevVersionDetail
	json.Unmarshal([]byte(detailJSON), &detail)

	repo := extractSourceRepo(detail)
	if repo != "" {
		t.Errorf("source repo = %q, want empty", repo)
	}
}

func TestOSVEndpointMock(t *testing.T) {
	// Mock OSV server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"vulns":[{"id":"GHSA-test","database_specific":{"severity":"HIGH"}}]}`))
	}))
	defer server.Close()

	count, sev, err := queryOSV(server.URL, "com.example:foo")
	if err != nil {
		t.Fatalf("queryOSV failed: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if sev != "HIGH" {
		t.Errorf("severity = %q, want HIGH", sev)
	}
}
