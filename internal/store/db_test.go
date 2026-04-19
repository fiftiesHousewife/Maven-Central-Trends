package store

import (
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	if err := Open(path); err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { Close() })
}

func TestOpenAndMigrate(t *testing.T) {
	setupTestDB(t)

	// Tables should exist
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='groups'").Scan(&name)
	if err != nil {
		t.Fatalf("groups table not created: %v", err)
	}
}

func TestUpsertAndQueryGroup(t *testing.T) {
	setupTestDB(t)

	g := Group{
		GroupID:        "com.example",
		FirstArtifact:  "example-core",
		FirstPublished: "2024-06-15",
		ArtifactCount:  5,
		LastUpdated:    "2026-03-01",
		TotalVersions:  42,
		License:        "Apache-2.0",
		SourceRepo:     "https://github.com/example/example",
	}

	if err := UpsertGroup(g); err != nil {
		t.Fatalf("UpsertGroup: %v", err)
	}

	if !GroupExists("com.example") {
		t.Error("GroupExists returned false for inserted group")
	}
	if GroupExists("com.nonexistent") {
		t.Error("GroupExists returned true for nonexistent group")
	}

	if n := TotalGroups(); n != 1 {
		t.Errorf("TotalGroups = %d, want 1", n)
	}

	// Upsert again with updated fields
	g.TotalVersions = 50
	if err := UpsertGroup(g); err != nil {
		t.Fatalf("UpsertGroup (update): %v", err)
	}
	if n := TotalGroups(); n != 1 {
		t.Errorf("TotalGroups after upsert = %d, want 1", n)
	}
}

func TestGroupsByMonth(t *testing.T) {
	setupTestDB(t)

	groups := []Group{
		{GroupID: "com.a", FirstPublished: "2024-01-15", FirstArtifact: "a"},
		{GroupID: "com.b", FirstPublished: "2024-01-20", FirstArtifact: "b"},
		{GroupID: "com.c", FirstPublished: "2024-02-10", FirstArtifact: "c"},
	}
	for _, g := range groups {
		if err := UpsertGroup(g); err != nil {
			t.Fatalf("UpsertGroup: %v", err)
		}
	}

	byMonth, err := GroupsByMonth()
	if err != nil {
		t.Fatalf("GroupsByMonth: %v", err)
	}

	if len(byMonth["2024-01"]) != 2 {
		t.Errorf("2024-01 has %d groups, want 2", len(byMonth["2024-01"]))
	}
	if len(byMonth["2024-02"]) != 1 {
		t.Errorf("2024-02 has %d groups, want 1", len(byMonth["2024-02"]))
	}
}

func TestGroupsForMonth(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.jan", FirstPublished: "2024-01-05", FirstArtifact: "j"})
	UpsertGroup(Group{GroupID: "com.feb", FirstPublished: "2024-02-10", FirstArtifact: "f"})

	jan, err := GroupsForMonth("2024-01")
	if err != nil {
		t.Fatalf("GroupsForMonth: %v", err)
	}
	if len(jan) != 1 || jan[0].GroupID != "com.jan" {
		t.Errorf("GroupsForMonth(2024-01) = %v, want [com.jan]", jan)
	}
}

func TestNewGroupsPerMonth(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.a", FirstPublished: "2024-03-01", FirstArtifact: "a"})
	UpsertGroup(Group{GroupID: "com.b", FirstPublished: "2024-03-15", FirstArtifact: "b"})
	UpsertGroup(Group{GroupID: "com.c", FirstPublished: "2024-04-01", FirstArtifact: "c"})

	counts, err := NewGroupsPerMonth()
	if err != nil {
		t.Fatalf("NewGroupsPerMonth: %v", err)
	}

	m := make(map[string]int)
	for _, mc := range counts {
		m[mc.Month] = mc.NewGroups
	}
	if m["2024-03"] != 2 {
		t.Errorf("2024-03 count = %d, want 2", m["2024-03"])
	}
	if m["2024-04"] != 1 {
		t.Errorf("2024-04 count = %d, want 1", m["2024-04"])
	}
}

func TestDepsDevEnrichment(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.enrich", FirstPublished: "2024-01-01", FirstArtifact: "e"})

	if err := UpdateDepsDevEnrichment("com.enrich", 10, "MIT", "https://github.com/x/y"); err != nil {
		t.Fatalf("UpdateDepsDevEnrichment: %v", err)
	}

	groups, err := UnenrichedGroups("depsdev")
	if err != nil {
		t.Fatalf("UnenrichedGroups: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 unenriched depsdev groups, got %d", len(groups))
	}

	// Still unenriched for OSV
	groups, err = UnenrichedGroups("osv")
	if err != nil {
		t.Fatalf("UnenrichedGroups(osv): %v", err)
	}
	if len(groups) != 1 {
		t.Errorf("expected 1 unenriched osv group, got %d", len(groups))
	}
}

func TestOSVEnrichment(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.vuln", FirstPublished: "2024-01-01", FirstArtifact: "v"})

	if err := UpdateOSVEnrichment("com.vuln", 3, "HIGH"); err != nil {
		t.Fatalf("UpdateOSVEnrichment: %v", err)
	}

	groups, err := UnenrichedGroups("osv")
	if err != nil {
		t.Fatalf("UnenrichedGroups: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 unenriched osv groups, got %d", len(groups))
	}
}

func TestPortalEnrichment(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.pop", FirstPublished: "2024-01-01", FirstArtifact: "p"})

	if err := UpdatePortalEnrichment("com.pop", 100, 50, 10, 0.85); err != nil {
		t.Fatalf("UpdatePortalEnrichment: %v", err)
	}

	groups, err := UnenrichedGroups("portal")
	if err != nil {
		t.Fatalf("UnenrichedGroups: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 unenriched portal groups, got %d", len(groups))
	}
}

func TestGroupByID(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{
		GroupID:        "com.test",
		FirstArtifact:  "test-core",
		FirstPublished: "2024-01-15",
		ArtifactCount:  3,
		TotalVersions:  10,
		License:        "MIT",
		SourceRepo:     "https://github.com/test/test",
	})
	UpdateDepsDevEnrichment("com.test", 10, "MIT", "https://github.com/test/test")
	UpdateOSVEnrichment("com.test", 2, "HIGH")

	g, err := GroupByID("com.test")
	if err != nil {
		t.Fatalf("GroupByID: %v", err)
	}
	if g.GroupID != "com.test" {
		t.Errorf("GroupID = %q, want com.test", g.GroupID)
	}
	if g.License != "MIT" {
		t.Errorf("License = %q, want MIT", g.License)
	}
	if !g.EnrichedDepsDev {
		t.Error("expected EnrichedDepsDev to be true")
	}
	if !g.EnrichedOSV {
		t.Error("expected EnrichedOSV to be true")
	}
	if g.EnrichedPortal {
		t.Error("expected EnrichedPortal to be false")
	}
	if g.CVECount != 2 {
		t.Errorf("CVECount = %d, want 2", g.CVECount)
	}

	_, err = GroupByID("com.nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent group")
	}
}

func TestLicensesByMonth(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.a", FirstPublished: "2024-01-15", FirstArtifact: "a", License: "Apache-2.0", TotalVersions: 5, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.b", FirstPublished: "2024-01-20", FirstArtifact: "b", License: "MIT", TotalVersions: 3, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.c", FirstPublished: "2024-01-25", FirstArtifact: "c", License: "Apache-2.0", TotalVersions: 2, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.d", FirstPublished: "2024-02-10", FirstArtifact: "d", License: "MIT", TotalVersions: 1, EnrichedDepsDev: true})
	// Unenriched group — should not appear
	UpsertGroup(Group{GroupID: "com.e", FirstPublished: "2024-01-15", FirstArtifact: "e", License: "GPL-3.0"})
	// No license — should not appear
	UpsertGroup(Group{GroupID: "com.f", FirstPublished: "2024-01-15", FirstArtifact: "f", TotalVersions: 1, EnrichedDepsDev: true})

	data, err := LicensesByMonth()
	if err != nil {
		t.Fatalf("LicensesByMonth: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 months, got %d", len(data))
	}

	jan := data[0]
	if jan.Month != "2024-01" {
		t.Errorf("first month = %q, want 2024-01", jan.Month)
	}
	if jan.Licenses["Apache-2.0"] != 2 {
		t.Errorf("Apache-2.0 count = %d, want 2", jan.Licenses["Apache-2.0"])
	}
	if jan.Licenses["MIT"] != 1 {
		t.Errorf("MIT count = %d, want 1", jan.Licenses["MIT"])
	}
}

func TestOneAndDoneByMonth(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.one", FirstPublished: "2024-03-01", FirstArtifact: "o", TotalVersions: 1, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.two", FirstPublished: "2024-03-15", FirstArtifact: "t", TotalVersions: 5, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.three", FirstPublished: "2024-03-20", FirstArtifact: "th", TotalVersions: 1, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.four", FirstPublished: "2024-04-01", FirstArtifact: "f", TotalVersions: 10, EnrichedDepsDev: true})

	data, err := OneAndDoneByMonth("")
	if err != nil {
		t.Fatalf("OneAndDoneByMonth: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 months, got %d", len(data))
	}

	mar := data[0]
	if mar.OneVersion != 2 {
		t.Errorf("2024-03 one_version = %d, want 2", mar.OneVersion)
	}
	if mar.Multiple != 1 {
		t.Errorf("2024-03 multiple = %d, want 1", mar.Multiple)
	}
	if mar.Total != 3 {
		t.Errorf("2024-03 total = %d, want 3", mar.Total)
	}
}

func TestGrowthByMonth(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.a", FirstPublished: "2024-01-15", FirstArtifact: "a", ArtifactCount: 3})
	UpsertGroup(Group{GroupID: "com.b", FirstPublished: "2024-01-20", FirstArtifact: "b", ArtifactCount: 5})
	UpsertGroup(Group{GroupID: "com.c", FirstPublished: "2024-02-10", FirstArtifact: "c", ArtifactCount: 2})

	data, err := GrowthByMonth()
	if err != nil {
		t.Fatalf("GrowthByMonth: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 months, got %d", len(data))
	}

	if data[0].NewArtifacts != 8 {
		t.Errorf("2024-01 new artifacts = %d, want 8", data[0].NewArtifacts)
	}
	if data[0].CumulArtifacts != 8 {
		t.Errorf("2024-01 cumul = %d, want 8", data[0].CumulArtifacts)
	}
	if data[1].CumulArtifacts != 10 {
		t.Errorf("2024-02 cumul = %d, want 10", data[1].CumulArtifacts)
	}
}

func TestGrowthByMonthCapsOutliers(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "org.mvnpm", FirstPublished: "2024-10-01", FirstArtifact: "npm", ArtifactCount: 5000})
	UpsertGroup(Group{GroupID: "com.normal", FirstPublished: "2024-10-15", FirstArtifact: "n", ArtifactCount: 10})

	data, err := GrowthByMonth()
	if err != nil {
		t.Fatalf("GrowthByMonth: %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("expected 1 month, got %d", len(data))
	}
	// 5000 should be capped to 500, so total = 500 + 10 = 510
	if data[0].NewArtifacts != 510 {
		t.Errorf("new artifacts = %d, want 510 (outlier capped)", data[0].NewArtifacts)
	}
}

func TestVersionsByMonth(t *testing.T) {
	setupTestDB(t)

	UpsertGroup(Group{GroupID: "com.a", FirstPublished: "2024-01-15", FirstArtifact: "a", TotalVersions: 10, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.b", FirstPublished: "2024-01-20", FirstArtifact: "b", TotalVersions: 20, EnrichedDepsDev: true})
	UpsertGroup(Group{GroupID: "com.c", FirstPublished: "2024-02-10", FirstArtifact: "c", TotalVersions: 5, EnrichedDepsDev: true})
	// Unenriched — should not appear
	UpsertGroup(Group{GroupID: "com.d", FirstPublished: "2024-01-01", FirstArtifact: "d", TotalVersions: 100})

	data, err := VersionsByMonth()
	if err != nil {
		t.Fatalf("VersionsByMonth: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 months, got %d", len(data))
	}

	if data[0].NewVersions != 30 {
		t.Errorf("2024-01 versions = %d, want 30", data[0].NewVersions)
	}
	if data[1].CumulVersions != 35 {
		t.Errorf("2024-02 cumul = %d, want 35", data[1].CumulVersions)
	}
}

func TestScanProgress(t *testing.T) {
	setupTestDB(t)

	SetPrefixComplete("ai", 277)
	SetPrefixComplete("com", 12914)

	completed, err := CompletedPrefixes()
	if err != nil {
		t.Fatalf("CompletedPrefixes: %v", err)
	}

	if completed["ai"] != 277 {
		t.Errorf("ai = %d, want 277", completed["ai"])
	}
	if completed["com"] != 12914 {
		t.Errorf("com = %d, want 12914", completed["com"])
	}

	// Update existing
	SetPrefixComplete("ai", 280)
	completed, _ = CompletedPrefixes()
	if completed["ai"] != 280 {
		t.Errorf("ai after update = %d, want 280", completed["ai"])
	}
}
