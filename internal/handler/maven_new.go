package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Top-level prefixes to scan for new groups.
var prefixes = []string{
	"ai", "app", "cc", "cloud", "co", "com", "de", "dev",
	"eu", "gg", "id", "io", "it", "me", "net", "nl",
	"org", "pl", "run", "se", "sh", "so", "tech", "top", "uk", "xyz",
}

const newCacheFile = "data/maven_new_cache.json"
const groupsCacheFile = "data/maven_new_groups_cache.json"
const scanProgressFile = "data/maven_scan_progress.json"

// --- Scan progress tracking (package-level, goroutine-safe) ---

type scanState struct {
	mu               sync.RWMutex
	CurrentPrefix    string `json:"current_prefix"`
	PrefixTotal      int    `json:"prefix_total"`
	PrefixDone       int    `json:"prefix_done"`
	PrefixSkipped    int    `json:"prefix_skipped"`
	CompletedCount   int    `json:"completed_prefixes"`
	TotalPrefixes    int    `json:"total_prefixes"`
	TotalGroupsFound int    `json:"total_groups_found"`
	Scanning         bool   `json:"scanning"`
	EnrichmentPhase  string `json:"enrichment_phase"`
	EnrichmentDone   int    `json:"enrichment_done"`
	EnrichmentTotal  int    `json:"enrichment_total"`
}

var scanStatus = &scanState{TotalPrefixes: len(prefixes)}

func (s *scanState) toJSON() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s)
	return b
}

func ScanProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(scanStatus.toJSON())
}

// --- Data types ---

var (
	fetchNewOnce sync.Once
	fetchNewDone = make(chan struct{})
)

type newMonthCount struct {
	Month     string `json:"month"`
	NewGroups int    `json:"new_groups"`
}

type groupInfo struct {
	GroupID        string `json:"group_id"`
	FirstArtifact  string `json:"first_artifact"`
	FirstPublished string `json:"first_published"`
	ArtifactCount  int    `json:"artifact_count"`
	LastUpdated    string `json:"last_updated"`
	TotalVersions  int    `json:"total_versions,omitempty"`
	License        string `json:"license,omitempty"`
	SourceRepo     string `json:"source_repo,omitempty"`
	CVECount       int    `json:"cve_count,omitempty"`
	MaxCVESeverity string `json:"max_cve_severity,omitempty"`
}

type publishInfo struct {
	FirstPublished time.Time
	LastUpdated    time.Time
	FirstArtifact  string
	ArtifactCount  int
	TotalVersions  int
	License        string
	SourceRepo     string
}

// --- Dedup ---

func dedup(groups []groupInfo) []groupInfo {
	seen := make(map[string]bool)
	var result []groupInfo
	for _, g := range groups {
		if !seen[g.GroupID] {
			seen[g.GroupID] = true
			result = append(result, g)
		}
	}
	return result
}

// --- Repo1 scraping ---

func listSubgroups(path string) ([]string, error) {
	u := fmt.Sprintf("https://repo1.maven.org/maven2/%s/", path)

	var resp *http.Response
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = httpClient.Get(u)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`href="([^"]+)/"`)
	matches := re.FindAllStringSubmatch(string(bodyBytes), -1)

	var dirs []string
	for _, m := range matches {
		name := m[1]
		if name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, name)
	}
	return dirs, nil
}

// --- deps.dev lookup ---

func firstPublishInfo(groupID string) (publishInfo, error) {
	path := strings.ReplaceAll(groupID, ".", "/")

	var artifacts []string
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		artifacts, err = listSubgroups(path)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		return publishInfo{}, fmt.Errorf("listing artifacts for %s: %w", groupID, err)
	}

	artCount := len(artifacts)
	tried := 0
	for _, art := range artifacts {
		if len(art) == 0 {
			continue
		}
		if tried >= 5 {
			break
		}
		tried++

		pkg := fmt.Sprintf("%s:%s", groupID, art)
		u := fmt.Sprintf("https://api.deps.dev/v3alpha/systems/maven/packages/%s",
			strings.ReplaceAll(pkg, ":", "%3A"))

		var resp *http.Response
		for attempt := 0; attempt < 3; attempt++ {
			resp, err = httpClient.Get(u)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
		if err != nil {
			slog.Debug("deps.dev request failed", "pkg", pkg, "error", err)
			continue
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			slog.Warn("deps.dev unexpected status", "pkg", pkg, "status", resp.StatusCode)
			continue
		}

		var result depsDevResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if len(result.Versions) > 0 && result.Versions[0].PublishedAt != "" {
			first, err := time.Parse(time.RFC3339, result.Versions[0].PublishedAt)
			if err != nil {
				continue
			}
			last := first
			lastV := result.Versions[len(result.Versions)-1]
			if lastV.PublishedAt != "" {
				if t, err := time.Parse(time.RFC3339, lastV.PublishedAt); err == nil {
					last = t
				}
			}
			info := publishInfo{
				FirstPublished: first,
				LastUpdated:    last,
				FirstArtifact:  art,
				ArtifactCount:  artCount,
				TotalVersions:  len(result.Versions),
			}

			// Fetch version detail for latest version (license, source repo)
			latestVersion := lastV.VersionKey.Version
			if latestVersion != "" {
				detail, err := fetchVersionDetail(groupID, art, latestVersion)
				if err == nil {
					if len(detail.Licenses) > 0 {
						info.License = detail.Licenses[0]
					}
					info.SourceRepo = extractSourceRepo(detail)
				}
			}

			return info, nil
		}
	}
	return publishInfo{}, fmt.Errorf("no publish date found for %s (tried %d of %d artifacts)", groupID, tried, artCount)
}

// --- Cache I/O ---

func loadGroupsCache() map[string][]groupInfo {
	data, err := os.ReadFile(groupsCacheFile)
	if err != nil {
		return make(map[string][]groupInfo)
	}
	var cached map[string][]groupInfo
	if err := json.Unmarshal(data, &cached); err != nil {
		return make(map[string][]groupInfo)
	}
	return cached
}

func writeGroupsCache(groups map[string][]groupInfo) {
	cleaned := make(map[string][]groupInfo)
	for month, gs := range groups {
		cleaned[month] = dedup(gs)
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return
	}
	os.WriteFile(groupsCacheFile, b, 0644)
}

type scanProgressData struct {
	CompletedPrefixes map[string]int `json:"completed_prefixes"`
}

func loadScanProgress() scanProgressData {
	data, err := os.ReadFile(scanProgressFile)
	if err != nil {
		return scanProgressData{CompletedPrefixes: make(map[string]int)}
	}
	var sp scanProgressData
	if err := json.Unmarshal(data, &sp); err != nil {
		return scanProgressData{CompletedPrefixes: make(map[string]int)}
	}
	if sp.CompletedPrefixes == nil {
		sp.CompletedPrefixes = make(map[string]int)
	}
	return sp
}

func saveScanProgress(sp scanProgressData) {
	b, err := json.Marshal(sp)
	if err != nil {
		return
	}
	os.WriteFile(scanProgressFile, b, 0644)
}

func loadNewCache() ([]newMonthCount, bool) {
	data, err := os.ReadFile(newCacheFile)
	if err != nil {
		return nil, false
	}
	var cached struct {
		Months []newMonthCount `json:"months"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}
	if len(cached.Months) == 0 {
		return nil, false
	}
	return cached.Months, true
}

func writeNewCache(months []newMonthCount) {
	out := struct {
		Months []newMonthCount `json:"months"`
	}{months}
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	os.WriteFile(newCacheFile, b, 0644)
}

func saveNewMonths(monthCounts map[string]int) {
	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0)
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)

	var months []newMonthCount
	for m := start; m.Before(now); m = m.AddDate(0, 1, 0) {
		if m.Year() == now.Year() && m.Month() == now.Month() && now.Day() < 30 {
			continue
		}
		key := m.Format("2006-01")
		months = append(months, newMonthCount{
			Month:     key,
			NewGroups: monthCounts[key],
		})
	}
	writeNewCache(months)
}

// --- Main scan loop ---

func StartNewFetch() {
	go fetchNewOnce.Do(func() {
		defer close(fetchNewDone)
		fetchNewGroups()
	})
}

func fetchNewGroups() {
	scanStatus.mu.Lock()
	scanStatus.Scanning = true
	scanStatus.mu.Unlock()
	defer func() {
		scanStatus.mu.Lock()
		scanStatus.Scanning = false
		scanStatus.mu.Unlock()
	}()

	progress := loadScanProgress()
	monthGroups := loadGroupsCache()

	// Rebuild monthCounts from cached group data
	monthCounts := make(map[string]int)
	for month, groups := range monthGroups {
		monthCounts[month] = len(dedup(groups))
	}

	// Count total groups found so far
	totalFound := 0
	for _, gs := range monthGroups {
		totalFound += len(dedup(gs))
	}

	scanStatus.mu.Lock()
	scanStatus.CompletedCount = len(progress.CompletedPrefixes)
	scanStatus.TotalGroupsFound = totalFound
	scanStatus.mu.Unlock()

	// Check if all prefixes are done
	allDone := true
	for _, p := range prefixes {
		if _, ok := progress.CompletedPrefixes[p]; !ok {
			allDone = false
			break
		}
	}
	if allDone {
		slog.Info("all prefixes already scanned", "prefixes", len(prefixes), "total_groups", totalFound)
		saveNewMonths(monthCounts)
		return
	}

	// Build global set of already-cached group IDs
	cachedIDs := make(map[string]bool)
	for _, gs := range monthGroups {
		for _, g := range gs {
			cachedIDs[g.GroupID] = true
		}
	}

	for _, prefix := range prefixes {
		if _, done := progress.CompletedPrefixes[prefix]; done {
			slog.Info("skipping completed prefix", "prefix", prefix)
			continue
		}

		prefixStart := time.Now()
		slog.Info("scanning prefix", "prefix", prefix)

		subgroups, err := listSubgroups(prefix)
		if err != nil {
			slog.Warn("failed to list subgroups", "prefix", prefix, "error", err)
			continue
		}

		scanStatus.mu.Lock()
		scanStatus.CurrentPrefix = prefix
		scanStatus.PrefixTotal = len(subgroups)
		scanStatus.PrefixDone = 0
		scanStatus.PrefixSkipped = 0
		scanStatus.mu.Unlock()

		slog.Info("prefix has subgroups", "prefix", prefix, "count", len(subgroups))

		newInPrefix := 0
		for i, sg := range subgroups {
			groupID := prefix + "." + sg

			scanStatus.mu.Lock()
			scanStatus.PrefixDone = i + 1
			scanStatus.mu.Unlock()

			if cachedIDs[groupID] {
				scanStatus.mu.Lock()
				scanStatus.PrefixSkipped++
				scanStatus.mu.Unlock()
				continue
			}

			info, err := firstPublishInfo(groupID)
			if err != nil {
				slog.Debug("group skipped", "group", groupID, "error", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			key := info.FirstPublished.Format("2006-01")
			monthCounts[key]++
			monthGroups[key] = append(monthGroups[key], groupInfo{
				GroupID:        groupID,
				FirstArtifact:  info.FirstArtifact,
				FirstPublished: info.FirstPublished.Format("2006-01-02"),
				ArtifactCount:  info.ArtifactCount,
				LastUpdated:    info.LastUpdated.Format("2006-01-02"),
				TotalVersions:  info.TotalVersions,
				License:        info.License,
				SourceRepo:     info.SourceRepo,
			})
			cachedIDs[groupID] = true
			newInPrefix++

			scanStatus.mu.Lock()
			scanStatus.TotalGroupsFound++
			scanStatus.mu.Unlock()

			slog.Info("new group found", "group", groupID, "artifact", info.FirstArtifact, "published", info.FirstPublished.Format("2006-01-02"))

			// Write cache frequently: every 25 new groups
			if newInPrefix%25 == 0 {
				writeGroupsCache(monthGroups)
				slog.Info("cache saved", "prefix", prefix, "new_in_prefix", newInPrefix, "subgroup", i+1, "of", len(subgroups))
			}

			time.Sleep(100 * time.Millisecond)
		}

		// Always save at end of prefix
		writeGroupsCache(monthGroups)
		progress.CompletedPrefixes[prefix] = len(subgroups)
		saveScanProgress(progress)

		scanStatus.mu.Lock()
		scanStatus.CompletedCount = len(progress.CompletedPrefixes)
		scanStatus.mu.Unlock()

		elapsed := time.Since(prefixStart)
		slog.Info("prefix complete", "prefix", prefix, "subgroups", len(subgroups), "new_groups", newInPrefix, "elapsed", elapsed.Round(time.Second))
	}

	slog.Info("scan complete", "total_groups", scanStatus.TotalGroupsFound)
	saveNewMonths(monthCounts)
}

// --- HTTP handlers ---

func MavenNew(w http.ResponseWriter, r *http.Request) {
	groups := loadGroupsCache()
	if len(groups) == 0 {
		http.Error(w, "data not yet available — fetch in progress", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0)
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)

	var months []newMonthCount
	for m := start; m.Before(now); m = m.AddDate(0, 1, 0) {
		if m.Year() == now.Year() && m.Month() == now.Month() && now.Day() < 30 {
			continue
		}
		key := m.Format("2006-01")
		months = append(months, newMonthCount{
			Month:     key,
			NewGroups: len(dedup(groups[key])),
		})
	}

	out := struct {
		Months []newMonthCount `json:"months"`
	}{months}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func MavenNewGroups(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	groups := loadGroupsCache()

	if month != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dedup(groups[month]))
		return
	}

	cleaned := make(map[string][]groupInfo)
	for m, gs := range groups {
		cleaned[m] = dedup(gs)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cleaned)
}
