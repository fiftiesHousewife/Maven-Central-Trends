// export dumps all API data as static JSON files and generates a static site
// suitable for GitHub Pages deployment.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/pippanewbold/maven-central-trends/internal/handler"
	"github.com/pippanewbold/maven-central-trends/internal/store"
)

func main() {
	outDir := "docs/site"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	if err := store.Open("data/maven.db"); err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	os.MkdirAll(filepath.Join(outDir, "api"), 0755)

	// Export API endpoints as static JSON
	apis := map[string]http.HandlerFunc{
		"api/new-groups.json":                handler.MavenNew,
		"api/new-groups-new.json":            withQuery(handler.MavenNew, "filter=new"),
		"api/new-groups-extensions.json":     withQuery(handler.MavenNew, "filter=extensions"),
		"api/one-and-done.json":              handler.OneAndDone,
		"api/one-and-done-new.json":          withQuery(handler.OneAndDone, "filter=new"),
		"api/one-and-done-extensions.json":   withQuery(handler.OneAndDone, "filter=extensions"),
		"api/groups-by-prefix.json":          handler.GroupsByPrefix,
		"api/license-trends.json":            handler.LicenseTrends,
		"api/growth.json":                    handler.GrowthData,
		"api/version-trends.json":            handler.VersionTrends,
		"api/cve-trends.json":                handler.CVETrends,
		"api/source-repos.json":              handler.SourceRepoTrends,
		"api/popularity.json":                handler.PopularityData,
		"api/size-distribution.json":         handler.SizeData,
		"api/contributors.json":              handler.ContributorData,
	}

	// Also export detail data for each month
	months := getMonths()

	for file, h := range apis {
		exportHandler(outDir, file, h)
	}

	for _, month := range months {
		file := fmt.Sprintf("api/new-groups-details-%s.json", month)
		exportHandler(outDir, file, withQuery(handler.MavenNewGroups, "month="+month))
	}

	// Generate static HTML pages that load from JSON files
	pages := map[string]http.HandlerFunc{
		"index.html":              handler.Index,
		"new-groups-per-month.html": handler.NewChart2,
		"publishes-per-month.html":  handler.Chart,
		"license-trends.html":      handler.LicenseChart,
		"artifact-trends.html":     handler.ArtifactChart,
		"version-trends.html":      handler.VersionsChart,
		"cve-trends.html":          handler.CVEChart,
		"source-repos.html":        handler.SourceRepoChart,
		"popularity.html":          handler.PopularityChart,
		"size-distribution.html":   handler.SizeChart,
		"contributors.html":        handler.ContributorsChart,
		"favicon.svg":              handler.Favicon,
	}

	for file, h := range pages {
		exportHandler(outDir, file, h)
	}

	// Rewrite HTML files to use static JSON paths
	rewriteAPIPaths(outDir)

	// Rewrite index links to use .html extensions
	rewriteIndexLinks(outDir)

	fmt.Printf("Static site exported to %s/ (%d API files, %d pages, %d month details)\n",
		outDir, len(apis), len(pages), len(months))
}

func exportHandler(outDir, file string, h http.HandlerFunc) {
	req := httptest.NewRequest(http.MethodGet, "/"+file, nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != 200 {
		fmt.Printf("  SKIP %s (status %d)\n", file, rec.Code)
		return
	}

	path := filepath.Join(outDir, file)
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, rec.Body.Bytes(), 0644); err != nil {
		fmt.Printf("  ERROR %s: %v\n", file, err)
		return
	}
	fmt.Printf("  OK %s (%d bytes)\n", file, rec.Body.Len())
}

func withQuery(h http.HandlerFunc, query string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.RawQuery = query
		h(w, r)
	}
}

func getMonths() []string {
	counts, err := store.NewGroupsPerMonth()
	if err != nil {
		return nil
	}
	var months []string
	for _, mc := range counts {
		months = append(months, mc.Month)
	}
	return months
}

func rewriteAPIPaths(outDir string) {
	replacements := map[string]string{
		// Filtered endpoints
		`'/api/new-groups' + filterParam`:   `'/api/new-groups' + (currentFilter !== 'all' ? '-' + currentFilter : '') + '.json'`,
		`'/api/one-and-done' + filterParam`: `'/api/one-and-done' + (currentFilter !== 'all' ? '-' + currentFilter : '') + '.json'`,
		// Detail endpoints
		`'/api/new-groups/details?month=' + month`: `'/api/new-groups-details-' + month + '.json'`,
		// Simple endpoints
		`'/api/groups-by-prefix'`:   `'/api/groups-by-prefix.json'`,
		`'/api/license-trends'`:     `'/api/license-trends.json'`,
		`'/api/growth'`:             `'/api/growth.json'`,
		`'/api/version-trends'`:     `'/api/version-trends.json'`,
		`'/api/cve-trends'`:         `'/api/cve-trends.json'`,
		`'/api/source-repos'`:       `'/api/source-repos.json'`,
		`'/api/popularity'`:         `'/api/popularity.json'`,
		`'/api/size-distribution'`:  `'/api/size-distribution.json'`,
		`'/api/contributors'`:       `'/api/contributors.json'`,
		`'/api/scan-progress'`:      `'data:application/json,{}'`,  // stub
		`'/api/group-popularity?namespace=' + groupId`: `'data:application/json,{}'`, // stub
	}

	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, _ := os.ReadFile(path)
		content := string(data)
		for old, repl := range replacements {
			content = strings.ReplaceAll(content, old, repl)
		}
		// Remove auto-refresh timers (no live data)
		content = strings.ReplaceAll(content, "setTimeout(update, 60000)", "")
		content = strings.ReplaceAll(content, "setTimeout(update, 120000)", "")
		content = strings.ReplaceAll(content, "setTimeout(update, 15000)", "")
		content = strings.ReplaceAll(content, "setTimeout(update, 10000)", "")
		os.WriteFile(path, []byte(content), 0644)
		return nil
	})
}

func rewriteIndexLinks(outDir string) {
	indexPath := filepath.Join(outDir, "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}
	content := string(data)
	// Rewrite chart links to .html
	links := map[string]string{
		`href="/new-groups-per-month"`: `href="new-groups-per-month.html"`,
		`href="/publishes-per-month"`:  `href="publishes-per-month.html"`,
		`href="/license-trends"`:       `href="license-trends.html"`,
		`href="/artifact-trends"`:      `href="artifact-trends.html"`,
		`href="/version-trends"`:       `href="version-trends.html"`,
		`href="/cve-trends"`:           `href="cve-trends.html"`,
		`href="/source-repos"`:         `href="source-repos.html"`,
		`href="/popularity"`:           `href="popularity.html"`,
		`href="/size-distribution"`:    `href="size-distribution.html"`,
		`href="/contributors"`:         `href="contributors.html"`,
		`href="/health"`:               `href="#"`,
		`href="/favicon.svg"`:          `href="favicon.svg"`,
	}
	// API links in the index
	for old, repl := range links {
		content = strings.ReplaceAll(content, old, repl)
	}
	// Remove API endpoint list items that don't work statically
	os.WriteFile(indexPath, []byte(content), 0644)
}

func init() {
	// Suppress JSON marshalling of empty responses
	_ = json.Marshal
}
