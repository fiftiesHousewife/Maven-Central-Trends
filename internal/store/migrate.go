package store

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// MigrateFromJSON imports existing JSON cache files into the SQLite database.
// It's idempotent — if data already exists in the DB, it skips the import.
func MigrateFromJSON(dataDir string) error {
	if TotalGroups() > 0 {
		slog.Info("database already has data, skipping JSON migration")
		return nil
	}

	slog.Info("migrating JSON caches to SQLite")

	if err := migrateGroups(filepath.Join(dataDir, "maven_new_groups_cache.json")); err != nil {
		slog.Warn("groups migration failed (may not exist yet)", "error", err)
	}

	if err := migrateScanProgress(filepath.Join(dataDir, "maven_scan_progress.json")); err != nil {
		slog.Warn("scan progress migration failed", "error", err)
	}

	if err := migrateVersionPublishes(filepath.Join(dataDir, "maven_series_cache.json")); err != nil {
		slog.Warn("version publishes migration failed", "error", err)
	}

	if err := migratePopularity(filepath.Join(dataDir, "maven_popularity_cache.json")); err != nil {
		slog.Warn("popularity migration failed (may not exist yet)", "error", err)
	}

	slog.Info("JSON migration complete", "total_groups", TotalGroups())
	return nil
}

func migrateGroups(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cached map[string][]struct {
		GroupID        string `json:"group_id"`
		FirstArtifact  string `json:"first_artifact"`
		FirstPublished string `json:"first_published"`
		ArtifactCount  int    `json:"artifact_count"`
		LastUpdated    string `json:"last_updated"`
		TotalVersions  int    `json:"total_versions"`
		License        string `json:"license"`
		SourceRepo     string `json:"source_repo"`
		CVECount       int    `json:"cve_count"`
		MaxCVESeverity string `json:"max_cve_severity"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return err
	}

	seen := make(map[string]bool)
	count := 0
	for _, groups := range cached {
		for _, g := range groups {
			if seen[g.GroupID] {
				continue
			}
			seen[g.GroupID] = true

			enrichedDeps := g.TotalVersions > 0
			enrichedOSV := g.CVECount > 0 || g.MaxCVESeverity != ""

			if err := UpsertGroup(Group{
				GroupID:         g.GroupID,
				FirstArtifact:   g.FirstArtifact,
				FirstPublished:  g.FirstPublished,
				ArtifactCount:   g.ArtifactCount,
				LastUpdated:     g.LastUpdated,
				TotalVersions:   g.TotalVersions,
				License:         g.License,
				SourceRepo:      g.SourceRepo,
				CVECount:        g.CVECount,
				MaxCVESeverity:  g.MaxCVESeverity,
				EnrichedDepsDev: enrichedDeps,
				EnrichedOSV:     enrichedOSV,
			}); err != nil {
				slog.Warn("failed to insert group", "group", g.GroupID, "error", err)
			}
			count++
		}
	}
	slog.Info("migrated groups", "count", count)
	return nil
}

func migrateScanProgress(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var sp struct {
		CompletedPrefixes map[string]int `json:"completed_prefixes"`
	}
	if err := json.Unmarshal(data, &sp); err != nil {
		return err
	}

	for prefix, count := range sp.CompletedPrefixes {
		if err := SetPrefixComplete(prefix, count); err != nil {
			slog.Warn("failed to insert prefix", "prefix", prefix, "error", err)
		}
	}
	slog.Info("migrated scan progress", "prefixes", len(sp.CompletedPrefixes))
	return nil
}

func migrateVersionPublishes(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cached struct {
		Months []struct {
			Month  string         `json:"month"`
			Groups map[string]int `json:"groups"`
		} `json:"months"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return err
	}

	count := 0
	for _, m := range cached.Months {
		for ns, c := range m.Groups {
			if err := UpsertVersionPublish(ns, m.Month, c); err != nil {
				slog.Warn("failed to insert version publish", "ns", ns, "month", m.Month, "error", err)
			}
			count++
		}
	}
	slog.Info("migrated version publishes", "rows", count)
	return nil
}

func migratePopularity(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var cached map[string]struct {
		DependentCount int     `json:"dependent_count"`
		AppCount       int     `json:"app_count"`
		OrgCount       int     `json:"org_count"`
		QualityScore   float64 `json:"quality_score"`
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return err
	}

	count := 0
	for groupID, info := range cached {
		if err := UpdatePortalEnrichment(groupID, info.DependentCount, info.AppCount, info.OrgCount, info.QualityScore); err != nil {
			slog.Warn("failed to update portal enrichment", "group", groupID, "error", err)
		}
		count++
	}
	slog.Info("migrated popularity data", "groups", count)
	return nil
}
