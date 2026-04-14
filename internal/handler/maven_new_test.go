package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

func setupTestStore(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := store.Open(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
}

func TestScanStatusJSON(t *testing.T) {
	scanStatus.mu.Lock()
	scanStatus.CurrentPrefix = "com"
	scanStatus.PrefixTotal = 12914
	scanStatus.PrefixDone = 5000
	scanStatus.CompletedCount = 5
	scanStatus.TotalGroupsFound = 3700
	scanStatus.Scanning = true
	scanStatus.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/scan-progress", nil)
	rec := httptest.NewRecorder()

	ScanProgress(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var state scanState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if state.CurrentPrefix != "com" {
		t.Errorf("CurrentPrefix = %q, want %q", state.CurrentPrefix, "com")
	}
	if state.PrefixTotal != 12914 {
		t.Errorf("PrefixTotal = %d, want 12914", state.PrefixTotal)
	}
	if state.PrefixDone != 5000 {
		t.Errorf("PrefixDone = %d, want 5000", state.PrefixDone)
	}
	if state.TotalGroupsFound != 3700 {
		t.Errorf("TotalGroupsFound = %d, want 3700", state.TotalGroupsFound)
	}
	if !state.Scanning {
		t.Error("Scanning should be true")
	}

	// Reset
	scanStatus.mu.Lock()
	scanStatus.Scanning = false
	scanStatus.CurrentPrefix = ""
	scanStatus.PrefixTotal = 0
	scanStatus.PrefixDone = 0
	scanStatus.CompletedCount = 0
	scanStatus.TotalGroupsFound = 0
	scanStatus.mu.Unlock()
}

func TestMavenNewHandler(t *testing.T) {
	setupTestStore(t)

	// Insert test groups
	store.UpsertGroup(store.Group{GroupID: "com.foo", FirstPublished: "2024-06-15", FirstArtifact: "foo"})
	store.UpsertGroup(store.Group{GroupID: "com.bar", FirstPublished: "2024-06-20", FirstArtifact: "bar"})
	store.UpsertGroup(store.Group{GroupID: "com.baz", FirstPublished: "2024-07-10", FirstArtifact: "baz"})

	req := httptest.NewRequest(http.MethodGet, "/api/new-groups", nil)
	rec := httptest.NewRecorder()
	MavenNew(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Months []store.MonthCount `json:"months"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	// Find 2024-06 in the response
	m := make(map[string]int)
	for _, mc := range resp.Months {
		m[mc.Month] = mc.NewGroups
	}

	if m["2024-06"] != 2 {
		t.Errorf("2024-06 = %d, want 2", m["2024-06"])
	}
	if m["2024-07"] != 1 {
		t.Errorf("2024-07 = %d, want 1", m["2024-07"])
	}
}

func TestMavenNewHandler503WhenEmpty(t *testing.T) {
	setupTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/new-groups", nil)
	rec := httptest.NewRecorder()
	MavenNew(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with no data, got %d", rec.Code)
	}
}

func TestMavenNewGroupsHandler(t *testing.T) {
	setupTestStore(t)

	store.UpsertGroup(store.Group{GroupID: "com.foo", FirstPublished: "2024-06-15", FirstArtifact: "foo"})
	store.UpsertGroup(store.Group{GroupID: "com.bar", FirstPublished: "2024-06-20", FirstArtifact: "bar"})

	req := httptest.NewRequest(http.MethodGet, "/api/new-groups/details?month=2024-06", nil)
	rec := httptest.NewRecorder()
	MavenNewGroups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var groups []groupJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &groups); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("got %d groups, want 2", len(groups))
	}
}

func TestMavenNewGroupsHandlerNoDuplicates(t *testing.T) {
	setupTestStore(t)

	// Insert the same group twice — SQLite primary key handles dedup
	store.UpsertGroup(store.Group{GroupID: "com.foo", FirstPublished: "2024-06-15", FirstArtifact: "foo-v1"})
	store.UpsertGroup(store.Group{GroupID: "com.foo", FirstPublished: "2024-06-15", FirstArtifact: "foo-v2"})

	req := httptest.NewRequest(http.MethodGet, "/api/new-groups/details?month=2024-06", nil)
	rec := httptest.NewRecorder()
	MavenNewGroups(rec, req)

	var groups []groupJSON
	json.Unmarshal(rec.Body.Bytes(), &groups)

	if len(groups) != 1 {
		t.Errorf("got %d groups after upsert, want 1 (dedup)", len(groups))
	}
	if groups[0].FirstArtifact != "foo-v2" {
		t.Errorf("expected updated artifact foo-v2, got %s", groups[0].FirstArtifact)
	}
}

func TestGroupJSONSerialization(t *testing.T) {
	g := groupJSON{
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
		t.Fatalf("marshal: %v", err)
	}

	var decoded groupJSON
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TotalVersions != 42 {
		t.Errorf("TotalVersions = %d, want 42", decoded.TotalVersions)
	}
	if decoded.License != "Apache-2.0" {
		t.Errorf("License = %q, want Apache-2.0", decoded.License)
	}
	if decoded.CVECount != 3 {
		t.Errorf("CVECount = %d, want 3", decoded.CVECount)
	}
}

func TestGroupJSONOmitsEmptyEnrichment(t *testing.T) {
	g := groupJSON{
		GroupID:        "com.old",
		FirstArtifact:  "old-lib",
		FirstPublished: "2023-01-01",
	}

	b, _ := json.Marshal(g)
	var m map[string]any
	json.Unmarshal(b, &m)

	if _, ok := m["total_versions"]; ok {
		t.Error("total_versions should be omitted when zero")
	}
	if _, ok := m["license"]; ok {
		t.Error("license should be omitted when empty")
	}
	if _, ok := m["cve_count"]; ok {
		t.Error("cve_count should be omitted when zero")
	}
}
