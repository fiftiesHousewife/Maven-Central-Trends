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
