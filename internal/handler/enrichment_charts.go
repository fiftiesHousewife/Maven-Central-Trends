package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pippanewbold/maven-central-trends/internal/store"
)

// dateRange returns the start month (4 years ago) and current month for filtering.
func dateRange() (start, current string) {
	now := time.Now().UTC()
	return now.AddDate(-4, 0, 0).Format("2006-01"), now.Format("2006-01")
}

// inRange returns true if month is within the last 4 years and not the current partial month.
func inRange(month string) bool {
	start, current := dateRange()
	return month >= start && month != current
}

// writeJSON encodes data as JSON to the response.
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// LicenseTrends returns license distribution per month.
func LicenseTrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.LicensesByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.LicenseTrend
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// OneAndDone returns one-version vs multi-version group counts per month.
func OneAndDone(w http.ResponseWriter, r *http.Request) {
	data, err := store.OneAndDoneByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.OneAndDoneStat
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// CVETrends returns CVE stats per month from OSV-enriched groups.
func CVETrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.CVEsByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.CVETrendStat
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// SourceRepoTrends returns source repo presence per month.
func SourceRepoTrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.SourceRepoByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.SourceRepoStat
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// PopularityData returns popularity distribution and top groups.
func PopularityData(w http.ResponseWriter, r *http.Request) {
	dist, err := store.PopularityDistribution()
	if err != nil || len(dist) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	top, _ := store.TopGroupsByDependents(25)
	writeJSON(w, map[string]any{"distribution": dist, "top_groups": top})
}

// SizeData returns group size distribution by month.
func SizeData(w http.ResponseWriter, r *http.Request) {
	byMonth, err := store.SizeDistributionByMonth()
	if err != nil || len(byMonth) == 0 {
		http.Error(w, "data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.SizeByMonthStat
	for _, d := range byMonth {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	overall, _ := store.SizeDistribution()
	writeJSON(w, map[string]any{"by_month": filtered, "overall": overall})
}

// ContributorData returns contributor stats per month.
func ContributorData(w http.ResponseWriter, r *http.Request) {
	data, err := store.ContributorsByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "GitHub enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.ContributorStat
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// GroupsByPrefix returns new groups per month broken down by top-level prefix.
func GroupsByPrefix(w http.ResponseWriter, r *http.Request) {
	data, err := store.GroupsByMonthAndPrefix()
	if err != nil || len(data) == 0 {
		http.Error(w, "data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.PrefixMonthCount
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// VersionTrends returns version publish counts by actual publish month.
func VersionTrends(w http.ResponseWriter, r *http.Request) {
	data, err := store.MonthlyVersionActivity()
	if err != nil || len(data) == 0 {
		http.Error(w, "enrichment data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.MonthCount
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}

// GrowthData returns new artifacts per month.
func GrowthData(w http.ResponseWriter, r *http.Request) {
	data, err := store.GrowthByMonth()
	if err != nil || len(data) == 0 {
		http.Error(w, "data not yet available", http.StatusServiceUnavailable)
		return
	}
	var filtered []store.GrowthStat
	for _, d := range data {
		if inRange(d.Month) {
			filtered = append(filtered, d)
		}
	}
	writeJSON(w, filtered)
}
