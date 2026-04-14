package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

type depsDevResponse struct {
	Versions []struct {
		VersionKey struct {
			Version string `json:"version"`
		} `json:"versionKey"`
		PublishedAt string `json:"publishedAt"`
	} `json:"versions"`
}

// Top-level prefixes to scan for new groups.
var prefixes = []string{
	"ai", "app", "cc", "cloud", "co", "com", "de", "dev",
	"eu", "gg", "id", "io", "it", "me", "net", "nl",
	"org", "pl", "run", "se", "sh", "so", "tech", "top", "uk", "xyz",
}


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

type publishInfo struct {
	FirstPublished time.Time
	LastUpdated    time.Time
	FirstArtifact  string
	ArtifactCount  int
	TotalVersions  int
	License        string
	SourceRepo     string
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

// deepenGroups discovers 3+ level groupIds by checking existing groups for
// deeper subgroups. Runs after the initial scan.
func deepenGroups() {
	slog.Info("starting deep group scan")

	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = "deep scan"
	scanStatus.mu.Unlock()

	existing, err := store.AllGroupIDs()
	if err != nil {
		slog.Error("failed to load groups for deep scan", "error", err)
		return
	}

	scanStatus.mu.Lock()
	scanStatus.EnrichmentTotal = len(existing)
	scanStatus.EnrichmentDone = 0
	scanStatus.mu.Unlock()

	newFound := 0
	for i, groupID := range existing {
		path := strings.ReplaceAll(groupID, ".", "/")
		entries, err := listSubgroups(path)
		if err != nil {
			scanStatus.mu.Lock()
			scanStatus.EnrichmentDone = i + 1
			scanStatus.mu.Unlock()
			continue
		}

		// Check each entry: if it's NOT an artifact (no version-like subdirs), it's a deeper group
		for _, entry := range entries {
			subpath := path + "/" + entry
			deepGroupID := groupID + "." + entry

			if store.GroupExists(deepGroupID) {
				continue
			}

			// Peek inside to see if this contains version directories (= artifact) or more dirs (= namespace)
			subEntries, err := listSubgroups(subpath)
			if err != nil || len(subEntries) == 0 {
				continue
			}

			// If first entry starts with a digit, this is an artifact dir, skip
			hasVersion := false
			for _, se := range subEntries {
				if len(se) > 0 && se[0] >= '0' && se[0] <= '9' {
					hasVersion = true
					break
				}
			}
			if hasVersion {
				continue
			}

			// This is a deeper namespace — register it as a group
			info, err := firstPublishInfo(deepGroupID)
			if err != nil {
				continue
			}

			enrichedDeps := info.TotalVersions > 0
			if err := store.UpsertGroup(store.Group{
				GroupID:         deepGroupID,
				FirstArtifact:   info.FirstArtifact,
				FirstPublished:  info.FirstPublished.Format("2006-01-02"),
				ArtifactCount:   info.ArtifactCount,
				LastUpdated:     info.LastUpdated.Format("2006-01-02"),
				TotalVersions:   info.TotalVersions,
				License:         info.License,
				SourceRepo:      info.SourceRepo,
				EnrichedDepsDev: enrichedDeps,
			}); err != nil {
				continue
			}

			newFound++
			scanStatus.mu.Lock()
			scanStatus.TotalGroupsFound++
			scanStatus.mu.Unlock()

			slog.Info("deep group found", "group", deepGroupID, "artifact", info.FirstArtifact)
			time.Sleep(100 * time.Millisecond)
		}

		scanStatus.mu.Lock()
		scanStatus.EnrichmentDone = i + 1
		scanStatus.mu.Unlock()

		if (i+1)%100 == 0 {
			slog.Info("deep scan progress", "done", i+1, "of", len(existing), "new", newFound)
		}

		time.Sleep(50 * time.Millisecond)
	}

	slog.Info("deep scan complete", "new_groups", newFound)
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
			urlEncode(pkg))

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

	completedPrefixes, err := store.CompletedPrefixes()
	if err != nil {
		slog.Error("failed to load scan progress", "error", err)
		completedPrefixes = make(map[string]int)
	}

	totalFound := store.TotalGroups()

	scanStatus.mu.Lock()
	scanStatus.CompletedCount = len(completedPrefixes)
	scanStatus.TotalGroupsFound = totalFound
	scanStatus.mu.Unlock()

	// Check if all prefixes are done
	allDone := true
	for _, p := range prefixes {
		if _, ok := completedPrefixes[p]; !ok {
			allDone = false
			break
		}
	}
	if allDone {
		slog.Info("all prefixes already scanned", "prefixes", len(prefixes), "total_groups", totalFound)
		return
	}

	for _, prefix := range prefixes {
		if _, done := completedPrefixes[prefix]; done {
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

			if store.GroupExists(groupID) {
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

			enrichedDeps := info.TotalVersions > 0
			if err := store.UpsertGroup(store.Group{
				GroupID:         groupID,
				FirstArtifact:   info.FirstArtifact,
				FirstPublished:  info.FirstPublished.Format("2006-01-02"),
				ArtifactCount:   info.ArtifactCount,
				LastUpdated:     info.LastUpdated.Format("2006-01-02"),
				TotalVersions:   info.TotalVersions,
				License:         info.License,
				SourceRepo:      info.SourceRepo,
				EnrichedDepsDev: enrichedDeps,
			}); err != nil {
				slog.Warn("failed to save group", "group", groupID, "error", err)
				continue
			}

			newInPrefix++

			scanStatus.mu.Lock()
			scanStatus.TotalGroupsFound++
			scanStatus.mu.Unlock()

			slog.Info("new group found", "group", groupID, "artifact", info.FirstArtifact, "published", info.FirstPublished.Format("2006-01-02"))

			if newInPrefix%25 == 0 {
				slog.Info("progress", "prefix", prefix, "new_in_prefix", newInPrefix, "subgroup", i+1, "of", len(subgroups))
			}

			time.Sleep(100 * time.Millisecond)
		}

		store.SetPrefixComplete(prefix, len(subgroups))

		scanStatus.mu.Lock()
		scanStatus.CompletedCount++
		scanStatus.mu.Unlock()

		elapsed := time.Since(prefixStart)
		slog.Info("prefix complete", "prefix", prefix, "subgroups", len(subgroups), "new_groups", newInPrefix, "elapsed", elapsed.Round(time.Second))
	}

	slog.Info("scan complete", "total_groups", store.TotalGroups())
}

// --- HTTP handlers ---

// groupJSON is the JSON representation of a group for the API.
type groupJSON struct {
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

func groupToJSON(g store.Group) groupJSON {
	return groupJSON{
		GroupID:        g.GroupID,
		FirstArtifact:  g.FirstArtifact,
		FirstPublished: g.FirstPublished,
		ArtifactCount:  g.ArtifactCount,
		LastUpdated:    g.LastUpdated,
		TotalVersions:  g.TotalVersions,
		License:        g.License,
		SourceRepo:     g.SourceRepo,
		CVECount:       g.CVECount,
		MaxCVESeverity: g.MaxCVESeverity,
	}
}

func MavenNew(w http.ResponseWriter, r *http.Request) {
	counts, err := store.NewGroupsPerMonth()
	if err != nil || len(counts) == 0 {
		http.Error(w, "data not yet available — fetch in progress", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	currentMonth := now.Format("2006-01")

	// Filter to last 4 years, exclude current partial month
	start := now.AddDate(-4, 0, 0)
	startMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")

	var months []store.MonthCount
	for _, mc := range counts {
		if mc.Month < startMonth {
			continue
		}
		if mc.Month == currentMonth && now.Day() < 30 {
			continue
		}
		months = append(months, mc)
	}

	out := struct {
		Months []store.MonthCount `json:"months"`
	}{months}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func MavenNewGroups(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")

	if month != "" {
		groups, err := store.GroupsForMonth(month)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		result := make([]groupJSON, len(groups))
		for i, g := range groups {
			result[i] = groupToJSON(g)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	byMonth, err := store.GroupsByMonth()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	result := make(map[string][]groupJSON)
	for m, gs := range byMonth {
		result[m] = make([]groupJSON, len(gs))
		for i, g := range gs {
			result[m][i] = groupToJSON(g)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
