package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

func TestLicenseTrendsHandler(t *testing.T) {
	setupTestStore(t)

	store.UpsertGroup(store.Group{GroupID: "com.a", FirstPublished: "2024-06-15", FirstArtifact: "a", License: "Apache-2.0", TotalVersions: 5, EnrichedDepsDev: true})
	store.UpsertGroup(store.Group{GroupID: "com.b", FirstPublished: "2024-06-20", FirstArtifact: "b", License: "MIT", TotalVersions: 3, EnrichedDepsDev: true})

	req := httptest.NewRequest(http.MethodGet, "/api/license-trends", nil)
	rec := httptest.NewRecorder()
	LicenseTrends(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var data []store.LicenseTrend
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected at least 1 month of data")
	}

	found := false
	for _, d := range data {
		if d.Month == "2024-06" {
			found = true
			if d.Licenses["Apache-2.0"] != 1 {
				t.Errorf("Apache-2.0 = %d, want 1", d.Licenses["Apache-2.0"])
			}
			if d.Licenses["MIT"] != 1 {
				t.Errorf("MIT = %d, want 1", d.Licenses["MIT"])
			}
		}
	}
	if !found {
		t.Error("expected 2024-06 in results")
	}
}

func TestLicenseTrends503WhenEmpty(t *testing.T) {
	setupTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/license-trends", nil)
	rec := httptest.NewRecorder()
	LicenseTrends(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with no data, got %d", rec.Code)
	}
}

func TestOneAndDoneHandler(t *testing.T) {
	setupTestStore(t)

	store.UpsertGroup(store.Group{GroupID: "com.one", FirstPublished: "2024-06-01", FirstArtifact: "o", TotalVersions: 1, EnrichedDepsDev: true})
	store.UpsertGroup(store.Group{GroupID: "com.multi", FirstPublished: "2024-06-15", FirstArtifact: "m", TotalVersions: 10, EnrichedDepsDev: true})

	req := httptest.NewRequest(http.MethodGet, "/api/one-and-done", nil)
	rec := httptest.NewRecorder()
	OneAndDone(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var data []store.OneAndDoneStat
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("parse: %v", err)
	}

	found := false
	for _, d := range data {
		if d.Month == "2024-06" {
			found = true
			if d.OneVersion != 1 {
				t.Errorf("one_version = %d, want 1", d.OneVersion)
			}
			if d.Multiple != 1 {
				t.Errorf("multiple = %d, want 1", d.Multiple)
			}
		}
	}
	if !found {
		t.Error("expected 2024-06 in results")
	}
}

func TestGrowthDataHandler(t *testing.T) {
	setupTestStore(t)

	store.UpsertGroup(store.Group{GroupID: "com.a", FirstPublished: "2024-06-15", FirstArtifact: "a", ArtifactCount: 5})
	store.UpsertGroup(store.Group{GroupID: "com.b", FirstPublished: "2024-07-01", FirstArtifact: "b", ArtifactCount: 10})

	req := httptest.NewRequest(http.MethodGet, "/api/growth", nil)
	rec := httptest.NewRecorder()
	GrowthData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var data []store.GrowthStat
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected growth data")
	}
}

func TestVersionTrendsHandler(t *testing.T) {
	setupTestStore(t)

	store.AddMonthlyVersions("2024-06", 50)
	store.AddMonthlyVersions("2024-07", 30)

	req := httptest.NewRequest(http.MethodGet, "/api/version-trends", nil)
	rec := httptest.NewRecorder()
	VersionTrends(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var data []store.MonthCount
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected version trend data")
	}
}

func TestVersionTrends503WhenEmpty(t *testing.T) {
	setupTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/version-trends", nil)
	rec := httptest.NewRecorder()
	VersionTrends(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}
