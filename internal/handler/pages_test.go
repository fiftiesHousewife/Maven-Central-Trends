package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pippanewbold/maven-central-trends/internal/handler"
)

func TestIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	for _, link := range []string{"/new-groups-per-month", "/publishes-per-month", "/license-trends", "/artifact-trends", "/version-trends"} {
		if !strings.Contains(body, link) {
			t.Errorf("index page missing link to %s", link)
		}
	}
}

func TestFavicon(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	rec := httptest.NewRecorder()
	handler.Favicon(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc == "" {
		t.Error("expected Cache-Control header")
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Error("expected SVG content")
	}
}

func TestChartPages(t *testing.T) {
	pages := []struct {
		name    string
		path    string
		handler http.HandlerFunc
		mustContain string
	}{
		{"Chart", "/publishes-per-month", handler.Chart, "echarts"},
		{"NewChart2", "/new-groups-per-month", handler.NewChart2, "echarts"},
		{"LicenseChart", "/license-trends", handler.LicenseChart, "license-trends"},
		{"ArtifactChart", "/artifact-trends", handler.ArtifactChart, "growth"},
		{"VersionsChart", "/version-trends", handler.VersionsChart, "version-trends"},
	}

	for _, tc := range pages {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			tc.handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
				t.Errorf("Content-Type = %q, want text/html", ct)
			}
			if !strings.Contains(rec.Body.String(), tc.mustContain) {
				t.Errorf("page %s missing expected content %q", tc.name, tc.mustContain)
			}
		})
	}
}
