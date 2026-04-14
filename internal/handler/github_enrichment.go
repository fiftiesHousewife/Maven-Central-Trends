package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

var githubOwnerRepo = regexp.MustCompile(`github\.com[:/]([^/]+)/([^/\s.]+?)(?:\.git)?(?:/.*)?$`)

func parseGithubRepo(sourceRepo string) (owner, repo string, ok bool) {
	m := githubOwnerRepo.FindStringSubmatch(sourceRepo)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

type githubRepoInfo struct {
	ContributorCount int
	OpenIssues       int
	Stars            int
	Forks            int
	Size             int
	Language         string
}

type githubUserInfo struct {
	Login     string `json:"login"`
	CreatedAt string `json:"created_at"`
	Type      string `json:"type"`
}

func githubGet(url, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return httpClient.Do(req)
}

func checkGithubRateLimit(resp *http.Response) time.Duration {
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "" {
		return 0
	}
	rem, _ := strconv.Atoi(remaining)
	if rem < 100 {
		resetStr := resp.Header.Get("X-RateLimit-Reset")
		resetEpoch, _ := strconv.ParseInt(resetStr, 10, 64)
		wait := time.Until(time.Unix(resetEpoch, 0))
		if wait > 0 {
			return wait + 5*time.Second
		}
	}
	return 0
}

func enrichWithGithub() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		slog.Info("GITHUB_TOKEN not set, skipping GitHub enrichment")
		return
	}

	unenriched, err := store.UnenrichedGroups("github")
	if err != nil {
		slog.Error("failed to query unenriched groups for github", "error", err)
		return
	}

	// Filter to groups with parseable GitHub URLs
	var toEnrich []store.Group
	for _, g := range unenriched {
		if _, _, ok := parseGithubRepo(g.SourceRepo); ok {
			toEnrich = append(toEnrich, g)
		} else {
			// Mark non-GitHub groups as enriched so we don't retry them
			store.UpdateGithubEnrichment(g.GroupID, 0, "", "")
		}
	}

	total := len(toEnrich)
	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = "GitHub"
	scanStatus.EnrichmentTotal = total
	scanStatus.EnrichmentDone = 0
	scanStatus.mu.Unlock()

	slog.Info("GitHub enrichment starting", "github_repos", total)

	enriched := 0
	seen := make(map[string]bool) // dedupe owner/repo

	for i, g := range toEnrich {
		owner, repo, _ := parseGithubRepo(g.SourceRepo)
		key := strings.ToLower(owner + "/" + repo)

		if seen[key] {
			// Same repo as another group — copy the data
			store.UpdateGithubEnrichment(g.GroupID, 0, "", "")
			scanStatus.mu.Lock()
			scanStatus.EnrichmentDone = i + 1
			scanStatus.mu.Unlock()
			continue
		}
		seen[key] = true

		// Fetch contributors list (up to 100)
		contribURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?per_page=100&anon=false", owner, repo)
		resp, err := githubGet(contribURL, token)
		if err != nil {
			time.Sleep(time.Second)
			store.UpdateGithubEnrichment(g.GroupID, 0, "", "")
			continue
		}

		if wait := checkGithubRateLimit(resp); wait > 0 {
			slog.Info("GitHub rate limit approaching, waiting", "wait", wait.Round(time.Second))
			resp.Body.Close()
			time.Sleep(wait)
			i-- // retry this group
			continue
		}

		contributorCount := 0
		primaryAuthor := ""
		if resp.StatusCode == 200 {
			var contribs []struct {
				Login         string `json:"login"`
				Contributions int    `json:"contributions"`
			}
			json.NewDecoder(resp.Body).Decode(&contribs)
			contributorCount = len(contribs)

			// Check Link header for repos with 100+ contributors
			link := resp.Header.Get("Link")
			if strings.Contains(link, `rel="last"`) {
				re := regexp.MustCompile(`page=(\d+)>; rel="last"`)
				if m := re.FindStringSubmatch(link); m != nil {
					lastPage, _ := strconv.Atoi(m[1])
					contributorCount = (lastPage - 1) * 100 // approximate
				}
			}

			if len(contribs) > 0 {
				primaryAuthor = contribs[0].Login
			}

			// Store all contributors
			for _, c := range contribs {
				store.UpsertContributor(g.GroupID, c.Login, c.Contributions)
			}
		}
		resp.Body.Close()

		// Fetch owner/author creation date
		authorCreated := ""
		lookupUser := primaryAuthor
		if lookupUser == "" {
			lookupUser = owner
		}
		resp3, err := githubGet(fmt.Sprintf("https://api.github.com/users/%s", lookupUser), token)
		if err == nil && resp3.StatusCode == 200 {
			var user githubUserInfo
			json.NewDecoder(resp3.Body).Decode(&user)
			if user.CreatedAt != "" {
				authorCreated = user.CreatedAt[:10] // "2024-01-15T..." -> "2024-01-15"
			}
		}
		if resp3 != nil {
			resp3.Body.Close()
		}

		if err := store.UpdateGithubEnrichment(g.GroupID, contributorCount, primaryAuthor, authorCreated); err != nil {
			slog.Warn("failed to update github enrichment", "group", g.GroupID, "error", err)
		}

		enriched++
		scanStatus.mu.Lock()
		scanStatus.EnrichmentDone = i + 1
		scanStatus.mu.Unlock()

		if (i+1)%100 == 0 {
			slog.Info("GitHub enrichment progress", "done", i+1, "of", total, "enriched", enriched)
		}

		// ~1 request per second to stay well under 5000/hr limit
		time.Sleep(1200 * time.Millisecond)
	}

	scanStatus.mu.Lock()
	scanStatus.EnrichmentPhase = ""
	scanStatus.EnrichmentDone = 0
	scanStatus.EnrichmentTotal = 0
	scanStatus.mu.Unlock()

	slog.Info("GitHub enrichment complete", "total", total, "enriched", enriched)
}
