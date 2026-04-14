package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateFromJSON(t *testing.T) {
	setupTestDB(t)
	dir := t.TempDir()

	// Write fake groups cache
	os.WriteFile(filepath.Join(dir, "maven_new_groups_cache.json"), []byte(`{
		"2024-01": [
			{"group_id": "com.foo", "first_artifact": "foo-core", "first_published": "2024-01-15", "artifact_count": 3, "last_updated": "2024-06-01", "total_versions": 12, "license": "Apache-2.0", "source_repo": "https://github.com/foo/foo"},
			{"group_id": "com.bar", "first_artifact": "bar-lib", "first_published": "2024-01-20", "artifact_count": 1, "last_updated": "2024-01-20"}
		],
		"2024-02": [
			{"group_id": "com.baz", "first_artifact": "baz", "first_published": "2024-02-10", "artifact_count": 2, "last_updated": "2024-03-01", "total_versions": 5, "license": "MIT", "cve_count": 2, "max_cve_severity": "HIGH"}
		]
	}`), 0644)

	// Write fake scan progress
	os.WriteFile(filepath.Join(dir, "maven_scan_progress.json"), []byte(`{
		"completed_prefixes": {"com": 12914, "ai": 277}
	}`), 0644)

	if err := MigrateFromJSON(dir); err != nil {
		t.Fatalf("MigrateFromJSON: %v", err)
	}

	// Verify groups
	if n := TotalGroups(); n != 3 {
		t.Errorf("TotalGroups = %d, want 3", n)
	}

	// Verify enrichment flags
	unenrichedDeps, _ := UnenrichedGroups("depsdev")
	if len(unenrichedDeps) != 1 { // com.bar has no total_versions
		t.Errorf("unenriched depsdev = %d, want 1", len(unenrichedDeps))
	}

	unenrichedOSV, _ := UnenrichedGroups("osv")
	if len(unenrichedOSV) != 2 { // only com.baz has CVEs
		t.Errorf("unenriched osv = %d, want 2", len(unenrichedOSV))
	}

	// Verify scan progress
	completed, _ := CompletedPrefixes()
	if completed["com"] != 12914 {
		t.Errorf("com prefix = %d, want 12914", completed["com"])
	}
	if completed["ai"] != 277 {
		t.Errorf("ai prefix = %d, want 277", completed["ai"])
	}

	// Verify idempotency — second call should be a no-op
	if err := MigrateFromJSON(dir); err != nil {
		t.Fatalf("MigrateFromJSON (second call): %v", err)
	}
	if n := TotalGroups(); n != 3 {
		t.Errorf("TotalGroups after second migration = %d, want 3", n)
	}
}

func TestMigrateNoFiles(t *testing.T) {
	setupTestDB(t)
	dir := t.TempDir()

	// Should not error when files don't exist
	if err := MigrateFromJSON(dir); err != nil {
		t.Fatalf("MigrateFromJSON with no files: %v", err)
	}
	if n := TotalGroups(); n != 0 {
		t.Errorf("TotalGroups = %d, want 0", n)
	}
}
