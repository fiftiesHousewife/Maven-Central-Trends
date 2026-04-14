package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

var groups = []struct {
	Namespace string `json:"namespace"`
	Label     string `json:"label"`
}{
	{"software.amazon.awssdk", "AWS SDK"},
	{"io.quarkus", "Quarkus"},
	{"com.google.cloud", "Google Cloud"},
	{"org.springframework.boot", "Spring Boot"},
	{"org.eclipse.jetty", "Eclipse Jetty"},
	{"dev.langchain4j", "LangChain4j"},
	{"io.opentelemetry", "OpenTelemetry"},
	{"org.testcontainers", "Testcontainers"},
	{"com.azure", "Azure SDK"},
	{"io.micronaut", "Micronaut"},
}

var (
	fetchOnce sync.Once
	fetchDone = make(chan struct{})
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func StartFetch() {
	go fetchOnce.Do(func() {
		defer close(fetchDone)
		fetchSeries()
	})
}

// listArtifacts scrapes repo1.maven.org to find artifact names under a namespace.
func listArtifacts(namespace string) ([]string, error) {
	path := strings.ReplaceAll(namespace, ".", "/")
	u := fmt.Sprintf("https://repo1.maven.org/maven2/%s/", path)

	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`href="([^"]+)/"`)
	matches := re.FindAllStringSubmatch(string(bodyBytes), -1)

	var artifacts []string
	for _, m := range matches {
		name := m[1]
		if name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		artifacts = append(artifacts, name)
	}
	return artifacts, nil
}

type depsDevResponse struct {
	Versions []struct {
		VersionKey struct {
			Version string `json:"version"`
		} `json:"versionKey"`
		PublishedAt string `json:"publishedAt"`
	} `json:"versions"`
}

// getVersionDates returns all publish dates for a given artifact from deps.dev.
func getVersionDates(namespace, artifact string) ([]time.Time, error) {
	pkg := fmt.Sprintf("%s:%s", namespace, artifact)
	u := fmt.Sprintf("https://api.deps.dev/v3alpha/systems/maven/packages/%s",
		urlEncode(pkg))

	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d for %s", resp.StatusCode, pkg)
	}

	var result depsDevResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var dates []time.Time
	for _, v := range result.Versions {
		if v.PublishedAt == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, v.PublishedAt)
		if err != nil {
			continue
		}
		dates = append(dates, t)
	}
	return dates, nil
}

func fetchSeries() {
	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0)
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Build month list
	var monthKeys []string
	for m := start; m.Before(now); m = m.AddDate(0, 1, 0) {
		if m.Year() == now.Year() && m.Month() == now.Month() && now.Day() < 30 {
			continue
		}
		monthKeys = append(monthKeys, m.Format("2006-01"))
	}

	// Check what's already in the DB
	existing, err := store.VersionPublishes()
	if err != nil {
		slog.Error("failed to load version publishes", "error", err)
	}
	cachedNS := make(map[string]bool)
	for _, vp := range existing {
		for ns, count := range vp.Groups {
			if count > 0 {
				cachedNS[ns] = true
			}
		}
	}

	for _, g := range groups {
		if cachedNS[g.Namespace] {
			slog.Info("skipping cached namespace", "namespace", g.Namespace)
			continue
		}

		slog.Info("fetching artifact list", "namespace", g.Namespace)
		artifacts, err := listArtifacts(g.Namespace)
		if err != nil {
			slog.Error("failed to list artifacts", "namespace", g.Namespace, "error", err)
			continue
		}
		slog.Info("found artifacts", "namespace", g.Namespace, "count", len(artifacts))

		namespaceDates := make(map[string]int)

		for j, artifact := range artifacts {
			dates, err := getVersionDates(g.Namespace, artifact)
			if err != nil {
				slog.Warn("failed to get versions", "artifact", artifact, "error", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			for _, d := range dates {
				key := d.Format("2006-01")
				namespaceDates[key]++
			}

			if (j+1)%50 == 0 {
				slog.Info("artifact progress", "namespace", g.Namespace, "done", j+1, "of", len(artifacts))
				// Intermediate save
				for _, mk := range monthKeys {
					store.UpsertVersionPublish(g.Namespace, mk, namespaceDates[mk])
				}
			}

			time.Sleep(100 * time.Millisecond)
		}

		// Save all months for this namespace
		for _, mk := range monthKeys {
			store.UpsertVersionPublish(g.Namespace, mk, namespaceDates[mk])
		}
		slog.Info("namespace complete", "namespace", g.Namespace, "artifacts", len(artifacts))
	}

	slog.Info("fetch complete", "months", len(monthKeys))
}

func MavenSeries(w http.ResponseWriter, r *http.Request) {
	vps, err := store.VersionPublishes()
	if err != nil || len(vps) == 0 {
		http.Error(w, "data not yet available — fetch in progress", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	currentMonth := now.Format("2006-01")

	// Sort by month
	sort.Slice(vps, func(i, j int) bool { return vps[i].Month < vps[j].Month })

	// Filter out current partial month
	var months []store.VersionPublishMonth
	for _, vp := range vps {
		if vp.Month == currentMonth && now.Day() < 30 {
			continue
		}
		months = append(months, vp)
	}

	type resp struct {
		Groups []struct {
			Namespace string `json:"namespace"`
			Label     string `json:"label"`
		} `json:"groups"`
		Months []store.VersionPublishMonth `json:"months"`
	}

	out := resp{Months: months}
	for _, g := range groups {
		out.Groups = append(out.Groups, struct {
			Namespace string `json:"namespace"`
			Label     string `json:"label"`
		}{g.Namespace, g.Label})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
