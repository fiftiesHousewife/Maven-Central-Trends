package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// --- deps.dev version detail types ---

type depsDevVersionDetail struct {
	Licenses    []string `json:"licenses"`
	AdvisoryKeys []struct {
		ID string `json:"id"`
	} `json:"advisoryKeys"`
	Links []struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	} `json:"links"`
}

func extractSourceRepo(detail depsDevVersionDetail) string {
	for _, link := range detail.Links {
		if link.Label == "SOURCE_REPO" {
			return link.URL
		}
	}
	return ""
}

// fetchVersionDetail gets license, advisories, and links for a specific version.
func fetchVersionDetail(groupID, artifact, version string) (depsDevVersionDetail, error) {
	pkg := fmt.Sprintf("%s:%s", groupID, artifact)
	u := fmt.Sprintf("https://api.deps.dev/v3alpha/systems/maven/packages/%s/versions/%s",
		urlEncode(pkg), urlEncode(version))

	resp, err := httpClient.Get(u)
	if err != nil {
		return depsDevVersionDetail{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return depsDevVersionDetail{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	var detail depsDevVersionDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return depsDevVersionDetail{}, err
	}
	return detail, nil
}

func urlEncode(s string) string {
	// Simple URL encoding for : character
	result := ""
	for _, c := range s {
		if c == ':' {
			result += "%3A"
		} else {
			result += string(c)
		}
	}
	return result
}

// --- OSV types and functions ---

type osvResponse struct {
	Vulns []struct {
		ID               string `json:"id"`
		Aliases          []string `json:"aliases"`
		DatabaseSpecific struct {
			Severity string `json:"severity"`
		} `json:"database_specific"`
	} `json:"vulns"`
}

var severityRank = map[string]int{
	"":         0,
	"LOW":      1,
	"MODERATE": 2,
	"HIGH":     3,
	"CRITICAL": 4,
}

func extractOSVSummary(resp osvResponse) (count int, maxSeverity string) {
	count = len(resp.Vulns)
	maxRank := 0
	for _, v := range resp.Vulns {
		sev := v.DatabaseSpecific.Severity
		if rank, ok := severityRank[sev]; ok && rank > maxRank {
			maxRank = rank
			maxSeverity = sev
		}
	}
	return
}

// queryOSV queries the OSV API for vulnerabilities. baseURL can be overridden for testing.
func queryOSV(baseURL string, pkg string) (count int, maxSeverity string, err error) {
	body, _ := json.Marshal(map[string]any{
		"package": map[string]string{
			"name":      pkg,
			"ecosystem": "Maven",
		},
	})

	resp, err := httpClient.Post(baseURL+"/v1/query", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, "", fmt.Errorf("OSV status %d", resp.StatusCode)
	}

	var osvResp osvResponse
	if err := json.NewDecoder(resp.Body).Decode(&osvResp); err != nil {
		return 0, "", err
	}

	count, maxSeverity = extractOSVSummary(osvResp)
	return
}

// --- Background enrichment ---

// StartEnrichment runs background enrichment after the main scan completes.
// It enriches cached groups with OSV data and slowly fetches Central Portal popularity.
func StartEnrichment() {
	go func() {
		// Wait for the main scan to finish
		<-fetchNewDone

		slog.Info("starting background enrichment")
		enrichWithOSV()
		enrichWithPortal()
	}()
}

func enrichWithOSV() {
	groups := loadGroupsCache()
	total := 0
	enriched := 0

	for _, gs := range groups {
		total += len(gs)
	}

	slog.Info("OSV enrichment starting", "total_groups", total)

	updated := false
	count := 0
	for month, gs := range groups {
		for i, g := range gs {
			count++
			// Skip if already enriched
			if g.CVECount > 0 || g.MaxCVESeverity != "" {
				continue
			}

			pkg := g.GroupID + ":" + g.FirstArtifact
			cveCount, maxSev, err := queryOSV("https://api.osv.dev", pkg)
			if err != nil {
				slog.Debug("OSV query failed", "pkg", pkg, "error", err)
				time.Sleep(200 * time.Millisecond)
				continue
			}

			if cveCount > 0 {
				groups[month][i].CVECount = cveCount
				groups[month][i].MaxCVESeverity = maxSev
				updated = true
				enriched++
				slog.Info("CVEs found", "group", g.GroupID, "cves", cveCount, "severity", maxSev)
			}

			if count%100 == 0 {
				slog.Info("OSV enrichment progress", "done", count, "of", total, "enriched", enriched)
				if updated {
					writeGroupsCache(groups)
					updated = false
				}
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	if updated {
		writeGroupsCache(groups)
	}
	slog.Info("OSV enrichment complete", "total", total, "with_cves", enriched)
}

func enrichWithPortal() {
	groups := loadGroupsCache()

	// Collect unique group IDs
	seen := make(map[string]bool)
	var groupIDs []string
	for _, gs := range groups {
		for _, g := range gs {
			if !seen[g.GroupID] {
				seen[g.GroupID] = true
				groupIDs = append(groupIDs, g.GroupID)
			}
		}
	}

	// Check which are already in popularity cache
	popMu.RLock()
	remaining := make([]string, 0)
	for _, id := range groupIDs {
		if _, ok := popCache[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	popMu.RUnlock()

	if len(remaining) == 0 {
		slog.Info("Central Portal enrichment: all groups already cached")
		return
	}

	slog.Info("Central Portal enrichment starting", "remaining", len(remaining), "total", len(groupIDs))

	for i, id := range remaining {
		body, _ := json.Marshal(map[string]any{
			"sortField":     "dependentOnCount",
			"sortDirection": "desc",
			"page":          0,
			"size":          1,
			"searchTerm":    "namespace:" + id,
		})

		resp, err := httpClient.Post(
			"https://central.sonatype.com/api/internal/browse/components",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			slog.Debug("Portal rate limited, backing off")
			time.Sleep(60 * time.Second)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		var result centralBrowseResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		info := popularityInfo{GroupID: id}
		if len(result.Components) > 0 {
			c := result.Components[0]
			info.DependentCount = c.DependentOnCount
			info.AppCount = c.NsPopularityAppCount
			info.OrgCount = c.NsPopularityOrgCount
		}

		popMu.Lock()
		popCache[id] = info
		popMu.Unlock()

		if (i+1)%50 == 0 {
			savePopularityCache()
			slog.Info("Portal enrichment progress", "done", i+1, "of", len(remaining))
		}

		time.Sleep(3 * time.Second)
	}

	savePopularityCache()
	slog.Info("Central Portal enrichment complete", "enriched", len(remaining))
}
