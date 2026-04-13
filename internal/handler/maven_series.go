package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"io"
	"strings"
	"sync"
	"time"
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

type monthCount struct {
	Month  string         `json:"month"`
	Groups map[string]int `json:"groups"`
}

const cacheFile = "data/maven_series_cache.json"

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
		strings.ReplaceAll(pkg, ":", "%3A"))

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

func loadCache() ([]monthCount, bool) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}
	var cached struct {
		Months []monthCount `json:"months"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}
	if len(cached.Months) == 0 {
		return nil, false
	}
	return cached.Months, true
}

func fetchSeries() {
	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0)
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Build month buckets — exclude current month unless it's the 30th or 31st
	var monthStarts []time.Time
	for m := start; m.Before(now); m = m.AddDate(0, 1, 0) {
		if m.Year() == now.Year() && m.Month() == now.Month() && now.Day() < 30 {
			continue
		}
		monthStarts = append(monthStarts, m)
	}

	// Always initialize fresh month structure, then merge cache on top
	months := make([]monthCount, len(monthStarts))
	for i, m := range monthStarts {
		months[i] = monthCount{
			Month:  m.Format("2006-01"),
			Groups: make(map[string]int),
		}
	}

	cachedMonths, cached := loadCache()
	if cached {
		// Merge cached group data into fresh months
		for i, m := range months {
			for _, cm := range cachedMonths {
				if cm.Month == m.Month {
					for k, v := range cm.Groups {
						months[i].Groups[k] = v
					}
					break
				}
			}
		}
	}

	for _, g := range groups {
		// Skip groups that already have data in cache
		if cached {
			hasData := false
			for _, m := range months {
				if v, ok := m.Groups[g.Namespace]; ok && v > 0 {
					hasData = true
					break
				}
			}
			if hasData {
				slog.Info("skipping cached group", "namespace", g.Namespace)
				continue
			}
		}

		slog.Info("fetching artifact list", "namespace", g.Namespace)
		artifacts, err := listArtifacts(g.Namespace)
		if err != nil {
			slog.Error("failed to list artifacts", "namespace", g.Namespace, "error", err)
			for i := range months {
				months[i].Groups[g.Namespace] = -1
			}
			writeCache(months)
			continue
		}
		slog.Info("found artifacts", "namespace", g.Namespace, "count", len(artifacts))

		// Collect all publish dates across all artifacts in this namespace
		namespaceDates := make(map[string]int) // month -> count

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
				// Intermediate save so progress isn't lost on crash
				for i, m := range months {
					if count, ok := namespaceDates[m.Month]; ok {
						months[i].Groups[g.Namespace] = count
					}
				}
				writeCache(months)
			}

			time.Sleep(100 * time.Millisecond)
		}

		// Fill in month counts for this group
		for i, m := range months {
			if count, ok := namespaceDates[m.Month]; ok {
				months[i].Groups[g.Namespace] = count
			} else {
				months[i].Groups[g.Namespace] = 0
			}
		}

		writeCache(months)
		slog.Info("namespace complete", "namespace", g.Namespace, "artifacts", len(artifacts))
	}

	slog.Info("fetch complete", "months", len(months))
}

func writeCache(months []monthCount) {
	type resp struct {
		Groups []struct {
			Namespace string `json:"namespace"`
			Label     string `json:"label"`
		} `json:"groups"`
		Months []monthCount `json:"months"`
	}

	out := resp{Months: months}
	for _, g := range groups {
		out.Groups = append(out.Groups, struct {
			Namespace string `json:"namespace"`
			Label     string `json:"label"`
		}{g.Namespace, g.Label})
	}

	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	os.WriteFile(cacheFile, b, 0644)
}

func MavenSeries(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		http.Error(w, "data not yet available — fetch in progress", http.StatusServiceUnavailable)
		return
	}

	// Filter out current partial month unless it's the 30th or 31st
	now := time.Now().UTC()
	if now.Day() < 30 {
		var cached struct {
			Groups json.RawMessage `json:"groups"`
			Months []monthCount    `json:"months"`
		}
		if err := json.Unmarshal(data, &cached); err == nil {
			currentMonth := now.Format("2006-01")
			filtered := make([]monthCount, 0, len(cached.Months))
			for _, m := range cached.Months {
				if m.Month != currentMonth {
					filtered = append(filtered, m)
				}
			}
			out := struct {
				Groups json.RawMessage `json:"groups"`
				Months []monthCount    `json:"months"`
			}{cached.Groups, filtered}
			data, _ = json.Marshal(out)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
