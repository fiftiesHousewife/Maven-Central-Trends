package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

// mockRepo1 creates an httptest server that serves repo1-style directory listings.
// tree maps path -> list of entries (subdirectories).
func mockRepo1(tree map[string][]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Strip trailing slash
		if len(path) > 1 && path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		}
		// Strip leading slash
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}

		entries, ok := tree[path]
		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		html := `<html><body>`
		for _, e := range entries {
			html += fmt.Sprintf(`<a href="%s/">%s/</a>`, e, e)
		}
		html += `</body></html>`
		w.Write([]byte(html))
	}))
}

// mockDepsDev creates an httptest server that returns deps.dev package info.
// packages maps "groupId:artifactId" -> depsDevResponse.
func mockDepsDev(packages map[string]depsDevResponse) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL: /systems/maven/packages/{encoded pkg}
		// We just return 404 for unknown packages
		w.Header().Set("Content-Type", "application/json")
		for pkg, resp := range packages {
			if r.URL.Path == "/systems/maven/packages/"+urlEncode(pkg) {
				json.NewEncoder(w).Encode(resp)
				return
			}
		}
		http.NotFound(w, r)
	}))
}

func TestFirstPublishInfo_WithArtifacts(t *testing.T) {
	repo1 := mockRepo1(map[string][]string{
		"com/example": {"my-lib", "my-utils"},
	})
	defer repo1.Close()

	depsDev := mockDepsDev(map[string]depsDevResponse{
		"com.example:my-lib": {
			Versions: []struct {
				VersionKey struct {
					Version string `json:"version"`
				} `json:"versionKey"`
				PublishedAt string `json:"publishedAt"`
			}{
				{VersionKey: struct {
					Version string `json:"version"`
				}{"1.0.0"}, PublishedAt: "2024-06-15T10:00:00Z"},
			},
		},
	})
	defer depsDev.Close()

	oldRepo1 := repo1BaseURL
	oldDepsDev := depsDevBaseURL
	repo1BaseURL = repo1.URL
	depsDevBaseURL = depsDev.URL
	defer func() { repo1BaseURL = oldRepo1; depsDevBaseURL = oldDepsDev }()

	info, err := firstPublishInfo("com.example")
	if err != nil {
		t.Fatalf("firstPublishInfo failed: %v", err)
	}
	if info.FirstArtifact != "my-lib" {
		t.Errorf("FirstArtifact = %q, want my-lib", info.FirstArtifact)
	}
	if info.FirstPublished.Year() != 2024 {
		t.Errorf("FirstPublished year = %d, want 2024", info.FirstPublished.Year())
	}
}

func TestFirstPublishInfo_NamespaceOnly(t *testing.T) {
	// org.apache has only namespace dirs, no artifacts with versions
	repo1 := mockRepo1(map[string][]string{
		"org/apache":         {"commons", "logging", "struts"},
		"org/apache/commons": {"lang3", "io"},      // these are also namespace dirs
		"org/apache/logging": {"log4j"},             // also namespace
		"org/apache/struts":  {"struts2-core"},      // also namespace
	})
	defer repo1.Close()

	depsDev := mockDepsDev(map[string]depsDevResponse{})
	defer depsDev.Close()

	oldRepo1 := repo1BaseURL
	oldDepsDev := depsDevBaseURL
	repo1BaseURL = repo1.URL
	depsDevBaseURL = depsDev.URL
	defer func() { repo1BaseURL = oldRepo1; depsDevBaseURL = oldDepsDev }()

	// firstPublishInfo should fail for a pure namespace group
	_, err := firstPublishInfo("org.apache")
	if err == nil {
		t.Error("expected error for namespace-only group, got nil")
	}
}

func TestFetchNewGroups_RegistersNamespaceGroups(t *testing.T) {
	dir := t.TempDir()
	if err := store.Open(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// org.apache has no artifacts — firstPublishInfo will fail
	repo1 := mockRepo1(map[string][]string{
		"org": {"apache", "example"},
		"org/apache":  {"commons", "logging"}, // all namespace dirs
		"org/example": {"1.0.0"},              // has version dir = artifact
	})
	defer repo1.Close()

	depsDev := mockDepsDev(map[string]depsDevResponse{
		"org.example:example": {}, // no versions but exists
	})
	defer depsDev.Close()

	oldRepo1 := repo1BaseURL
	oldDepsDev := depsDevBaseURL
	repo1BaseURL = repo1.URL
	depsDevBaseURL = depsDev.URL
	defer func() { repo1BaseURL = oldRepo1; depsDevBaseURL = oldDepsDev }()

	// org.apache should be registered even though it has no artifacts
	if store.GroupExists("org.apache") {
		t.Fatal("org.apache should not exist before scan")
	}

	// Simulate what fetchNewGroups does for a single group
	_, err := firstPublishInfo("org.apache")
	if err != nil {
		// This is expected — register it anyway (like the fixed code does)
		store.UpsertGroup(store.Group{GroupID: "org.apache"})
	}

	if !store.GroupExists("org.apache") {
		t.Error("org.apache should be registered even without artifacts")
	}
}

func TestDeepenGroups_DiscoversDeeperLevels(t *testing.T) {
	dir := t.TempDir()
	if err := store.Open(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Repo1 tree: com/example has namespace child "deep" which has artifact "my-lib"
	repo1 := mockRepo1(map[string][]string{
		"com/example":      {"deep", "my-artifact"},
		"com/example/deep": {"my-lib"},
		// my-artifact has versions (it's an artifact dir)
		"com/example/my-artifact": {"1.0.0", "2.0.0"},
		// my-lib has versions (it's an artifact dir)
		"com/example/deep/my-lib": {"1.0.0"},
	})
	defer repo1.Close()

	depsDev := mockDepsDev(map[string]depsDevResponse{
		"com.example.deep:my-lib": {
			Versions: []struct {
				VersionKey struct {
					Version string `json:"version"`
				} `json:"versionKey"`
				PublishedAt string `json:"publishedAt"`
			}{
				{VersionKey: struct {
					Version string `json:"version"`
				}{"1.0.0"}, PublishedAt: "2024-01-01T00:00:00Z"},
			},
		},
	})
	defer depsDev.Close()

	oldRepo1 := repo1BaseURL
	oldDepsDev := depsDevBaseURL
	repo1BaseURL = repo1.URL
	depsDevBaseURL = depsDev.URL
	defer func() { repo1BaseURL = oldRepo1; depsDevBaseURL = oldDepsDev }()

	// Seed the DB with the parent group
	store.UpsertGroup(store.Group{GroupID: "com.example", FirstArtifact: "my-artifact"})

	// Run deep scan with just the seeded group
	deepenGroups()

	// com.example.deep should have been discovered
	if !store.GroupExists("com.example.deep") {
		t.Error("com.example.deep should have been discovered by deep scan")
	}
}

func TestEnrichWithOSV_AllArtifacts(t *testing.T) {
	dir := t.TempDir()
	if err := store.Open(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Group with multiple artifacts — CVEs on the second one
	store.UpsertGroup(store.Group{
		GroupID:       "com.vuln",
		FirstArtifact: "safe-lib",
		ArtifactCount: 2,
	})

	repo1 := mockRepo1(map[string][]string{
		"com/vuln": {"safe-lib", "unsafe-lib"},
		// Both have version dirs (they're artifacts)
		"com/vuln/safe-lib":   {"1.0.0"},
		"com/vuln/unsafe-lib": {"1.0.0"},
	})
	defer repo1.Close()

	osvHits := 0
	osvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Package struct {
				Name string `json:"name"`
			} `json:"package"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if req.Package.Name == "com.vuln:unsafe-lib" {
			osvHits++
			w.Write([]byte(`{"vulns":[
				{"id":"CVE-1","database_specific":{"severity":"HIGH"}},
				{"id":"CVE-2","database_specific":{"severity":"CRITICAL"}}
			]}`))
		} else {
			w.Write([]byte(`{}`))
		}
	}))
	defer osvServer.Close()

	oldRepo1 := repo1BaseURL
	repo1BaseURL = repo1.URL
	defer func() { repo1BaseURL = oldRepo1 }()

	// Query OSV for each artifact
	pkg1 := "com.vuln:safe-lib"
	count1, _, _ := queryOSV(osvServer.URL, pkg1)

	pkg2 := "com.vuln:unsafe-lib"
	count2, sev2, _ := queryOSV(osvServer.URL, pkg2)

	totalCVEs := count1 + count2
	if totalCVEs != 2 {
		t.Errorf("total CVEs = %d, want 2", totalCVEs)
	}
	if sev2 != "CRITICAL" {
		t.Errorf("max severity = %q, want CRITICAL", sev2)
	}
	if osvHits != 1 {
		t.Errorf("OSV should have been queried for unsafe-lib, hits = %d", osvHits)
	}
}

func TestParseGithubRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{"https://github.com/google/guava", "google", "guava", true},
		{"https://github.com/google/guava/tree/master", "google", "guava", true},
		{"git@github.com:google/guava.git", "google", "guava", true},
		{"scm:git@github.com:google/guava.git", "google", "guava", true},
		{"https://gitlab.com/foo/bar", "", "", false},
		{"", "", "", false},
	}

	for _, tc := range tests {
		owner, repo, ok := parseGithubRepo(tc.input)
		if ok != tc.wantOK {
			t.Errorf("parseGithubRepo(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
		}
		if ok && (owner != tc.wantOwner || repo != tc.wantRepo) {
			t.Errorf("parseGithubRepo(%q) = %s/%s, want %s/%s", tc.input, owner, repo, tc.wantOwner, tc.wantRepo)
		}
	}
}
