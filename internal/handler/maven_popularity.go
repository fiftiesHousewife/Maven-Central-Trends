package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

type popularityInfo struct {
	GroupID        string `json:"group_id"`
	DependentCount int    `json:"dependent_count"`
	AppCount       int    `json:"app_count"`
	OrgCount       int    `json:"org_count"`
}

type centralBrowseResponse struct {
	Components []struct {
		Namespace            string `json:"namespace"`
		Name                 string `json:"name"`
		DependentOnCount     int    `json:"dependentOnCount"`
		NsPopularityAppCount int    `json:"nsPopularityAppCount"`
		NsPopularityOrgCount int    `json:"nsPopularityOrgCount"`
	} `json:"components"`
	TotalResultCount int `json:"totalResultCount"`
}

const popularityCacheFile = "maven_popularity_cache.json"

var (
	popMu    sync.RWMutex
	popCache map[string]popularityInfo
)

func init() {
	popCache = loadPopularityCache()
}

func loadPopularityCache() map[string]popularityInfo {
	data, err := os.ReadFile(popularityCacheFile)
	if err != nil {
		return make(map[string]popularityInfo)
	}
	var cached map[string]popularityInfo
	if err := json.Unmarshal(data, &cached); err != nil {
		return make(map[string]popularityInfo)
	}
	return cached
}

func savePopularityCache() {
	popMu.RLock()
	b, err := json.Marshal(popCache)
	popMu.RUnlock()
	if err != nil {
		return
	}
	os.WriteFile(popularityCacheFile, b, 0644)
}

func GroupPopularity(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		http.Error(w, "namespace parameter required", http.StatusBadRequest)
		return
	}

	// Check cache first
	popMu.RLock()
	if info, ok := popCache[ns]; ok {
		popMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		json.NewEncoder(w).Encode(info)
		return
	}
	popMu.RUnlock()

	// Fetch from Central Portal
	body, _ := json.Marshal(map[string]any{
		"sortField":     "dependentOnCount",
		"sortDirection": "desc",
		"page":          0,
		"size":          1,
		"searchTerm":    "namespace:" + ns,
	})

	resp, err := httpClient.Post(
		"https://central.sonatype.com/api/internal/browse/components",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		http.Error(w, "failed to query central portal", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "central portal error", http.StatusBadGateway)
		return
	}

	var result centralBrowseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "failed to parse response", http.StatusInternalServerError)
		return
	}

	info := popularityInfo{GroupID: ns}
	if len(result.Components) > 0 {
		c := result.Components[0]
		info.DependentCount = c.DependentOnCount
		info.AppCount = c.NsPopularityAppCount
		info.OrgCount = c.NsPopularityOrgCount
	}

	// Cache and persist
	popMu.Lock()
	popCache[ns] = info
	popMu.Unlock()
	savePopularityCache()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(info)
}
