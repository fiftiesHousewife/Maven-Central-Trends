package handler

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

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

type popularityJSON struct {
	GroupID        string `json:"group_id"`
	DependentCount int    `json:"dependent_count"`
	AppCount       int    `json:"app_count"`
	OrgCount       int    `json:"org_count"`
}

func GroupPopularity(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		http.Error(w, "namespace parameter required", http.StatusBadRequest)
		return
	}

	// Check if already enriched in the DB
	g, err := store.GroupByID(ns)
	if err == nil && g.EnrichedPortal {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		json.NewEncoder(w).Encode(popularityJSON{
			GroupID:        ns,
			DependentCount: g.DependentCount,
			AppCount:       g.AppCount,
			OrgCount:       g.OrgCount,
		})
		return
	}

	// Fetch on-demand from Central Portal
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

	info := popularityJSON{GroupID: ns}
	if len(result.Components) > 0 {
		c := result.Components[0]
		info.DependentCount = c.DependentOnCount
		info.AppCount = c.NsPopularityAppCount
		info.OrgCount = c.NsPopularityOrgCount
	}

	// Store in DB for future use
	store.UpdatePortalEnrichment(ns, info.DependentCount, info.AppCount, info.OrgCount, 0)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(info)
}
