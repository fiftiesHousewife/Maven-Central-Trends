package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/pippanewbold/maven-central-trends/internal/store"
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
	u := fmt.Sprintf("%s/systems/maven/packages/%s/versions/%s",
		depsDevBaseURL, urlEncode(pkg), urlEncode(version))

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
	return url.PathEscape(s)
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
		enrichWithDepsDevDetail()
		enrichWithOSV()
		enrichWithPortal()
		enrichWithGithub()
	}()
}

func enrichWithDepsDevDetail() {
	unenriched, err := store.UnenrichedGroups("depsdev")
	if err != nil {
		slog.Error("failed to query unenriched groups", "error", err)
		return
	}

	total := len(unenriched)
	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = "deps.dev detail"
	scanStatus.EnrichmentTotal = total
	scanStatus.EnrichmentDone = 0
	scanStatus.mu.Unlock()

	slog.Info("deps.dev detail enrichment starting", "unenriched", total)

	enriched := 0
	for i, g := range unenriched {
		if g.FirstArtifact == "" {
			continue
		}

		pkg := fmt.Sprintf("%s:%s", g.GroupID, g.FirstArtifact)
		u := fmt.Sprintf("%s/systems/maven/packages/%s",
			depsDevBaseURL, urlEncode(pkg))

		resp, err := httpClient.Get(u)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		var result depsDevResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		totalVersions := len(result.Versions)
		license := ""
		sourceRepo := ""

		if len(result.Versions) > 0 {
			latest := result.Versions[len(result.Versions)-1]
			if latest.VersionKey.Version != "" {
				detail, err := fetchVersionDetail(g.GroupID, g.FirstArtifact, latest.VersionKey.Version)
				if err == nil {
					if len(detail.Licenses) > 0 {
						license = detail.Licenses[0]
					}
					sourceRepo = extractSourceRepo(detail)
				}
			}
		}

		if err := store.UpdateDepsDevEnrichment(g.GroupID, totalVersions, license, sourceRepo); err != nil {
			slog.Warn("failed to update depsdev enrichment", "group", g.GroupID, "error", err)
		}
		enriched++

		scanStatus.mu.Lock()
		scanStatus.EnrichmentDone = i + 1
		scanStatus.mu.Unlock()

		if (i+1)%100 == 0 {
			slog.Info("deps.dev detail progress", "done", i+1, "of", total, "enriched", enriched)
		}

		time.Sleep(100 * time.Millisecond)
	}

	slog.Info("deps.dev detail enrichment complete", "total", total, "enriched", enriched)
}

func enrichWithOSV() {
	unenriched, err := store.UnenrichedGroups("osv")
	if err != nil {
		slog.Error("failed to query unenriched groups for OSV", "error", err)
		return
	}

	total := len(unenriched)
	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = "OSV CVEs"
	scanStatus.EnrichmentTotal = total
	scanStatus.EnrichmentDone = 0
	scanStatus.mu.Unlock()

	slog.Info("OSV enrichment starting", "unenriched", total)

	enriched := 0
	for i, g := range unenriched {
		// Skip namespace-only groups with no artifacts
		if g.FirstArtifact == "" {
			store.UpdateOSVEnrichment(g.GroupID, 0, "")
			scanStatus.mu.Lock()
			scanStatus.EnrichmentDone = i + 1
			scanStatus.mu.Unlock()
			continue
		}

		// List all artifacts for the group from repo1 and query each for CVEs.
		path := strings.ReplaceAll(g.GroupID, ".", "/")
		artifacts, listErr := listSubgroups(path)
		if listErr != nil || len(artifacts) == 0 {
			artifacts = []string{g.FirstArtifact}
		}

		totalCVEs := 0
		bestSev := ""

		for _, art := range artifacts {
			// Skip entries that are likely namespace dirs, not artifacts
			if !strings.Contains(art, "-") && len(art) < 4 {
				continue
			}
			pkg := g.GroupID + ":" + art
			count, sev, err := queryOSV("https://api.osv.dev", pkg)
			if err != nil {
				continue
			}
			if count > 0 {
				totalCVEs += count
				if severityRank[sev] > severityRank[bestSev] {
					bestSev = sev
				}
			}
		}

		if err := store.UpdateOSVEnrichment(g.GroupID, totalCVEs, bestSev); err != nil {
			slog.Warn("failed to update OSV enrichment", "group", g.GroupID, "error", err)
		}

		if totalCVEs > 0 {
			enriched++
			slog.Info("CVEs found", "group", g.GroupID, "cves", totalCVEs, "severity", bestSev)
		}

		scanStatus.mu.Lock()
		scanStatus.EnrichmentDone = i + 1
		scanStatus.mu.Unlock()

		if (i+1)%100 == 0 {
			slog.Info("OSV enrichment progress", "done", i+1, "of", total, "enriched", enriched)
		}

		time.Sleep(100 * time.Millisecond)
	}

	slog.Info("OSV enrichment complete", "total", total, "with_cves", enriched)
}

func enrichWithPortal() {
	unenriched, err := store.UnenrichedGroups("portal")
	if err != nil {
		slog.Error("failed to query unenriched groups for portal", "error", err)
		return
	}

	if len(unenriched) == 0 {
		slog.Info("Central Portal enrichment: all groups already enriched")
		return
	}

	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = "Central Portal"
	scanStatus.EnrichmentTotal = len(unenriched)
	scanStatus.EnrichmentDone = 0
	scanStatus.mu.Unlock()

	slog.Info("Central Portal enrichment starting", "unenriched", len(unenriched))

	for i, g := range unenriched {
		body, _ := json.Marshal(map[string]any{
			"sortField":     "dependentOnCount",
			"sortDirection": "desc",
			"page":          0,
			"size":          1,
			"searchTerm":    "namespace:" + g.GroupID,
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

		depCount, appCount, orgCount := 0, 0, 0
		if len(result.Components) > 0 {
			c := result.Components[0]
			depCount = c.DependentOnCount
			appCount = c.NsPopularityAppCount
			orgCount = c.NsPopularityOrgCount
		}

		if err := store.UpdatePortalEnrichment(g.GroupID, depCount, appCount, orgCount, 0); err != nil {
			slog.Warn("failed to update portal enrichment", "group", g.GroupID, "error", err)
		}

		scanStatus.mu.Lock()
		scanStatus.EnrichmentDone = i + 1
		scanStatus.mu.Unlock()

		if (i+1)%50 == 0 {
			slog.Info("Portal enrichment progress", "done", i+1, "of", len(unenriched))
		}

		time.Sleep(3 * time.Second)
	}

	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = ""
	scanStatus.EnrichmentDone = 0
	scanStatus.EnrichmentTotal = 0
	scanStatus.mu.Unlock()

	slog.Info("Central Portal enrichment complete", "enriched", len(unenriched))
}
