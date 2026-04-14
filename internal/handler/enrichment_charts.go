package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

// LicenseTrends returns license distribution per month.
func LicenseTrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.LicensesByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.LicenseTrend
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// OneAndDone returns one-version vs multi-version group counts per month.
func OneAndDone(w http.ResponseWriter, r *http.Request) {
	data, err := store.OneAndDoneByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.OneAndDoneStat
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// CVETrends returns CVE stats per month from OSV-enriched groups.
func CVETrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.CVEsByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.CVETrendStat
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// SourceRepoTrends returns source repo presence per month.
func SourceRepoTrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.SourceRepoByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.SourceRepoStat
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// PopularityData returns popularity distribution and top groups.
func PopularityData(w http.ResponseWriter, r *http.Request) {
	dist, err := store.PopularityDistribution()
	if err != nil || len(dist) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	top, _ := store.TopGroupsByDependents(25)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"distribution": dist,
		"top_groups":   top,
	})
}

// SizeData returns group size distribution by month.
func SizeData(w http.ResponseWriter, r *http.Request) {
	byMonth, err := store.SizeDistributionByMonth()
	if err != nil || len(byMonth) == 0 {
		http.Error(w, "data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.SizeByMonthStat
	for _, d := range byMonth {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	overall, _ := store.SizeDistribution()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"by_month": filtered,
		"overall":  overall,
	})
}

// ContributorData returns contributor stats per month.
func ContributorData(w http.ResponseWriter, r *http.Request) {
	data, err := store.ContributorsByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "GitHub enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.ContributorStat
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// GroupsByPrefix returns new groups per month broken down by top-level prefix.
func GroupsByPrefix(w http.ResponseWriter, r *http.Request) {
	data, err := store.GroupsByMonthAndPrefix()
	if err != nil || len(data) == 0 {
		http.Error(w, "data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.PrefixMonthCount
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// VersionTrends returns version counts per month from enriched groups.
func VersionTrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.VersionsByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.VersionTrendStat
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// GrowthData returns cumulative groups and artifacts over time.
func GrowthData(w http.ResponseWriter, r *http.Request) {
	data, err := store.GrowthByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "data not yet available", http.StatusServiceUnavailable)
		return
	}

	now := time.Now().UTC()
	start := now.AddDate(-4, 0, 0).Format("2006-01")
	current := now.Format("2006-01")

	var filtered []store.GrowthStat
	for _, d := range data {
		if d.Month < start || d.Month == current {
			continue
		}
		filtered = append(filtered, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}
