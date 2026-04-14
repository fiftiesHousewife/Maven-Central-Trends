package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	dbMu sync.Mutex
)

// DB returns the singleton database connection.
func DB() *sql.DB {
	return db
}

// Open initialises the SQLite database at the given path, creating tables if needed.
// Safe to call multiple times; subsequent calls are no-ops if already open.
func Open(path string) error {
	dbMu.Lock()
	defer dbMu.Unlock()

	if db != nil {
		return nil
	}

	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)

	var err error
	db, err = sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)
	return migrate()
}

// Close shuts down the database connection.
func Close() error {
	dbMu.Lock()
	defer dbMu.Unlock()
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

func migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS groups (
		group_id TEXT PRIMARY KEY,
		first_artifact TEXT NOT NULL DEFAULT '',
		first_published TEXT NOT NULL DEFAULT '',
		artifact_count INTEGER NOT NULL DEFAULT 0,
		last_updated TEXT NOT NULL DEFAULT '',
		total_versions INTEGER NOT NULL DEFAULT 0,
		license TEXT NOT NULL DEFAULT '',
		source_repo TEXT NOT NULL DEFAULT '',
		cve_count INTEGER NOT NULL DEFAULT 0,
		max_cve_severity TEXT NOT NULL DEFAULT '',
		dependent_count INTEGER NOT NULL DEFAULT 0,
		app_count INTEGER NOT NULL DEFAULT 0,
		org_count INTEGER NOT NULL DEFAULT 0,
		quality_score REAL NOT NULL DEFAULT 0,
		enriched_depsdev INTEGER NOT NULL DEFAULT 0,
		enriched_osv INTEGER NOT NULL DEFAULT 0,
		enriched_portal INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS version_publishes (
		namespace TEXT NOT NULL,
		month TEXT NOT NULL,
		publish_count INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (namespace, month)
	);

	CREATE TABLE IF NOT EXISTS scan_progress (
		prefix TEXT PRIMARY KEY,
		subgroups_count INTEGER NOT NULL DEFAULT 0,
		completed_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE INDEX IF NOT EXISTS idx_groups_first_published ON groups(first_published);
	CREATE INDEX IF NOT EXISTS idx_groups_enriched ON groups(enriched_depsdev, enriched_osv, enriched_portal);
	`
	_, err := db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	slog.Info("database migrated")
	return nil
}

// --- Groups CRUD ---

type Group struct {
	GroupID        string
	FirstArtifact  string
	FirstPublished string
	ArtifactCount  int
	LastUpdated    string
	TotalVersions  int
	License        string
	SourceRepo     string
	CVECount       int
	MaxCVESeverity string
	DependentCount int
	AppCount       int
	OrgCount       int
	QualityScore   float64
	EnrichedDepsDev bool
	EnrichedOSV     bool
	EnrichedPortal  bool
}

// UpsertGroup inserts or updates a group row.
func UpsertGroup(g Group) error {
	_, err := db.Exec(`
		INSERT INTO groups (group_id, first_artifact, first_published, artifact_count, last_updated,
			total_versions, license, source_repo, cve_count, max_cve_severity,
			dependent_count, app_count, org_count, quality_score,
			enriched_depsdev, enriched_osv, enriched_portal)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id) DO UPDATE SET
			first_artifact = excluded.first_artifact,
			first_published = excluded.first_published,
			artifact_count = excluded.artifact_count,
			last_updated = excluded.last_updated,
			total_versions = excluded.total_versions,
			license = excluded.license,
			source_repo = excluded.source_repo,
			cve_count = excluded.cve_count,
			max_cve_severity = excluded.max_cve_severity,
			dependent_count = excluded.dependent_count,
			app_count = excluded.app_count,
			org_count = excluded.org_count,
			quality_score = excluded.quality_score,
			enriched_depsdev = excluded.enriched_depsdev,
			enriched_osv = excluded.enriched_osv,
			enriched_portal = excluded.enriched_portal`,
		g.GroupID, g.FirstArtifact, g.FirstPublished, g.ArtifactCount, g.LastUpdated,
		g.TotalVersions, g.License, g.SourceRepo, g.CVECount, g.MaxCVESeverity,
		g.DependentCount, g.AppCount, g.OrgCount, g.QualityScore,
		boolToInt(g.EnrichedDepsDev), boolToInt(g.EnrichedOSV), boolToInt(g.EnrichedPortal))
	return err
}

// UpdateDepsDevEnrichment updates enrichment fields from deps.dev for a single group.
func UpdateDepsDevEnrichment(groupID string, totalVersions int, license, sourceRepo string) error {
	_, err := db.Exec(`
		UPDATE groups SET total_versions = ?, license = ?, source_repo = ?, enriched_depsdev = 1
		WHERE group_id = ?`,
		totalVersions, license, sourceRepo, groupID)
	return err
}

// UpdateOSVEnrichment updates CVE fields for a single group.
func UpdateOSVEnrichment(groupID string, cveCount int, maxSeverity string) error {
	_, err := db.Exec(`
		UPDATE groups SET cve_count = ?, max_cve_severity = ?, enriched_osv = 1
		WHERE group_id = ?`,
		cveCount, maxSeverity, groupID)
	return err
}

// UpdatePortalEnrichment updates popularity fields for a single group.
func UpdatePortalEnrichment(groupID string, dependentCount, appCount, orgCount int, qualityScore float64) error {
	_, err := db.Exec(`
		UPDATE groups SET dependent_count = ?, app_count = ?, org_count = ?, quality_score = ?, enriched_portal = 1
		WHERE group_id = ?`,
		dependentCount, appCount, orgCount, qualityScore, groupID)
	return err
}

// GroupByID returns a single group by its ID.
func GroupByID(groupID string) (Group, error) {
	var g Group
	var ed, eo, ep int
	err := db.QueryRow(`
		SELECT group_id, first_artifact, first_published, artifact_count, last_updated,
			total_versions, license, source_repo, cve_count, max_cve_severity,
			dependent_count, app_count, org_count, quality_score,
			enriched_depsdev, enriched_osv, enriched_portal
		FROM groups WHERE group_id = ?`, groupID).Scan(
		&g.GroupID, &g.FirstArtifact, &g.FirstPublished, &g.ArtifactCount, &g.LastUpdated,
		&g.TotalVersions, &g.License, &g.SourceRepo, &g.CVECount, &g.MaxCVESeverity,
		&g.DependentCount, &g.AppCount, &g.OrgCount, &g.QualityScore,
		&ed, &eo, &ep)
	if err != nil {
		return Group{}, err
	}
	g.EnrichedDepsDev = ed == 1
	g.EnrichedOSV = eo == 1
	g.EnrichedPortal = ep == 1
	return g, nil
}

// GroupExists checks whether a group_id is already in the database.
func GroupExists(groupID string) bool {
	var n int
	err := db.QueryRow("SELECT 1 FROM groups WHERE group_id = ?", groupID).Scan(&n)
	return err == nil
}

// GroupsByMonth returns groups bucketed by their first_published month (YYYY-MM).
func GroupsByMonth() (map[string][]Group, error) {
	rows, err := db.Query(`
		SELECT group_id, first_artifact, first_published, artifact_count, last_updated,
			total_versions, license, source_repo, cve_count, max_cve_severity,
			dependent_count, app_count, org_count, quality_score,
			enriched_depsdev, enriched_osv, enriched_portal
		FROM groups
		ORDER BY first_published`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Group)
	for rows.Next() {
		var g Group
		var ed, eo, ep int
		if err := rows.Scan(&g.GroupID, &g.FirstArtifact, &g.FirstPublished, &g.ArtifactCount, &g.LastUpdated,
			&g.TotalVersions, &g.License, &g.SourceRepo, &g.CVECount, &g.MaxCVESeverity,
			&g.DependentCount, &g.AppCount, &g.OrgCount, &g.QualityScore,
			&ed, &eo, &ep); err != nil {
			return nil, err
		}
		g.EnrichedDepsDev = ed == 1
		g.EnrichedOSV = eo == 1
		g.EnrichedPortal = ep == 1

		month := g.FirstPublished[:7] // "2024-01-15" -> "2024-01"
		if len(g.FirstPublished) >= 7 {
			month = g.FirstPublished[:7]
		}
		result[month] = append(result[month], g)
	}
	return result, rows.Err()
}

// GroupsForMonth returns all groups first published in a given month.
func GroupsForMonth(month string) ([]Group, error) {
	rows, err := db.Query(`
		SELECT group_id, first_artifact, first_published, artifact_count, last_updated,
			total_versions, license, source_repo, cve_count, max_cve_severity,
			dependent_count, app_count, org_count, quality_score,
			enriched_depsdev, enriched_osv, enriched_portal
		FROM groups
		WHERE first_published LIKE ?
		ORDER BY first_published`,
		month+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Group
	for rows.Next() {
		var g Group
		var ed, eo, ep int
		if err := rows.Scan(&g.GroupID, &g.FirstArtifact, &g.FirstPublished, &g.ArtifactCount, &g.LastUpdated,
			&g.TotalVersions, &g.License, &g.SourceRepo, &g.CVECount, &g.MaxCVESeverity,
			&g.DependentCount, &g.AppCount, &g.OrgCount, &g.QualityScore,
			&ed, &eo, &ep); err != nil {
			return nil, err
		}
		g.EnrichedDepsDev = ed == 1
		g.EnrichedOSV = eo == 1
		g.EnrichedPortal = ep == 1
		result = append(result, g)
	}
	return result, rows.Err()
}

// NewGroupsPerMonth returns the count of new groups per month.
func NewGroupsPerMonth() ([]MonthCount, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month, COUNT(*) AS cnt
		FROM groups
		WHERE first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MonthCount
	for rows.Next() {
		var mc MonthCount
		if err := rows.Scan(&mc.Month, &mc.NewGroups); err != nil {
			return nil, err
		}
		result = append(result, mc)
	}
	return result, rows.Err()
}

type MonthCount struct {
	Month     string `json:"month"`
	NewGroups int    `json:"new_groups"`
}

// UnenrichedGroups returns groups where the given enrichment phase is incomplete.
func UnenrichedGroups(phase string) ([]Group, error) {
	col := "enriched_depsdev"
	switch phase {
	case "osv":
		col = "enriched_osv"
	case "portal":
		col = "enriched_portal"
	}
	rows, err := db.Query(fmt.Sprintf(`
		SELECT group_id, first_artifact, first_published, artifact_count, last_updated,
			total_versions, license, source_repo, cve_count, max_cve_severity,
			dependent_count, app_count, org_count, quality_score,
			enriched_depsdev, enriched_osv, enriched_portal
		FROM groups
		WHERE %s = 0`, col))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Group
	for rows.Next() {
		var g Group
		var ed, eo, ep int
		if err := rows.Scan(&g.GroupID, &g.FirstArtifact, &g.FirstPublished, &g.ArtifactCount, &g.LastUpdated,
			&g.TotalVersions, &g.License, &g.SourceRepo, &g.CVECount, &g.MaxCVESeverity,
			&g.DependentCount, &g.AppCount, &g.OrgCount, &g.QualityScore,
			&ed, &eo, &ep); err != nil {
			return nil, err
		}
		g.EnrichedDepsDev = ed == 1
		g.EnrichedOSV = eo == 1
		g.EnrichedPortal = ep == 1
		result = append(result, g)
	}
	return result, rows.Err()
}

// TotalGroups returns the total number of groups in the database.
func TotalGroups() int {
	var n int
	db.QueryRow("SELECT COUNT(*) FROM groups").Scan(&n)
	return n
}

// --- Version Publishes CRUD ---

// UpsertVersionPublish inserts or updates a version publish count.
func UpsertVersionPublish(namespace, month string, count int) error {
	_, err := db.Exec(`
		INSERT INTO version_publishes (namespace, month, publish_count)
		VALUES (?, ?, ?)
		ON CONFLICT(namespace, month) DO UPDATE SET publish_count = excluded.publish_count`,
		namespace, month, count)
	return err
}

// VersionPublishes returns all version publish data, keyed by month.
func VersionPublishes() ([]VersionPublishMonth, error) {
	rows, err := db.Query(`
		SELECT namespace, month, publish_count
		FROM version_publishes
		ORDER BY month, namespace`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byMonth := make(map[string]map[string]int)
	for rows.Next() {
		var ns, month string
		var count int
		if err := rows.Scan(&ns, &month, &count); err != nil {
			return nil, err
		}
		if byMonth[month] == nil {
			byMonth[month] = make(map[string]int)
		}
		byMonth[month][ns] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []VersionPublishMonth
	for month, groups := range byMonth {
		result = append(result, VersionPublishMonth{Month: month, Groups: groups})
	}
	return result, nil
}

type VersionPublishMonth struct {
	Month  string         `json:"month"`
	Groups map[string]int `json:"groups"`
}

// --- Scan Progress CRUD ---

// SetPrefixComplete marks a prefix scan as complete.
func SetPrefixComplete(prefix string, subgroupsCount int) error {
	_, err := db.Exec(`
		INSERT INTO scan_progress (prefix, subgroups_count)
		VALUES (?, ?)
		ON CONFLICT(prefix) DO UPDATE SET subgroups_count = excluded.subgroups_count, completed_at = datetime('now')`,
		prefix, subgroupsCount)
	return err
}

// CompletedPrefixes returns the map of completed prefixes and their subgroup counts.
func CompletedPrefixes() (map[string]int, error) {
	rows, err := db.Query("SELECT prefix, subgroups_count FROM scan_progress")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var prefix string
		var count int
		if err := rows.Scan(&prefix, &count); err != nil {
			return nil, err
		}
		result[prefix] = count
	}
	return result, rows.Err()
}

// --- Enrichment analytics queries ---

// LicenseTrend holds per-month license counts.
type LicenseTrend struct {
	Month    string         `json:"month"`
	Licenses map[string]int `json:"licenses"`
}

// LicensesByMonth returns the top licenses bucketed by first_published month.
// Only includes groups where enriched_depsdev = 1 and license is non-empty.
func LicensesByMonth() ([]LicenseTrend, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month, license, COUNT(*) AS cnt
		FROM groups
		WHERE enriched_depsdev = 1 AND license != '' AND first_published != ''
		GROUP BY month, license
		ORDER BY month, cnt DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byMonth := make(map[string]map[string]int)
	var months []string
	for rows.Next() {
		var month, license string
		var cnt int
		if err := rows.Scan(&month, &license, &cnt); err != nil {
			return nil, err
		}
		if byMonth[month] == nil {
			byMonth[month] = make(map[string]int)
			months = append(months, month)
		}
		byMonth[month][license] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []LicenseTrend
	for _, m := range months {
		result = append(result, LicenseTrend{Month: m, Licenses: byMonth[m]})
	}
	return result, nil
}

// OneAndDoneStat holds per-month one-and-done vs active group counts.
type OneAndDoneStat struct {
	Month      string `json:"month"`
	OneVersion int    `json:"one_version"`
	Multiple   int    `json:"multiple"`
	Total      int    `json:"total"`
}

// OneAndDoneByMonth returns the count of single-version vs multi-version groups per month.
// Only includes groups where enriched_depsdev = 1 (so total_versions is meaningful).
func OneAndDoneByMonth() ([]OneAndDoneStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			SUM(CASE WHEN total_versions <= 1 THEN 1 ELSE 0 END) AS one_ver,
			SUM(CASE WHEN total_versions > 1 THEN 1 ELSE 0 END) AS multi,
			COUNT(*) AS total
		FROM groups
		WHERE enriched_depsdev = 1 AND first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []OneAndDoneStat
	for rows.Next() {
		var s OneAndDoneStat
		if err := rows.Scan(&s.Month, &s.OneVersion, &s.Multiple, &s.Total); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GrowthStat holds per-month artifact counts for cumulative growth.
type GrowthStat struct {
	Month          string `json:"month"`
	NewArtifacts   int    `json:"new_artifacts"`
	CumulArtifacts int    `json:"cumul_artifacts"`
}

// GrowthByMonth returns per-month new artifacts with running totals.
// Caps individual group contribution at 500 artifacts to exclude outliers
// like org.mvnpm (which repackages all of NPM).
func GrowthByMonth() ([]GrowthStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			SUM(MIN(artifact_count, 500)) AS new_artifacts
		FROM groups
		WHERE first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GrowthStat
	cumulA := 0
	for rows.Next() {
		var s GrowthStat
		if err := rows.Scan(&s.Month, &s.NewArtifacts); err != nil {
			return nil, err
		}
		cumulA += s.NewArtifacts
		s.CumulArtifacts = cumulA
		result = append(result, s)
	}
	return result, rows.Err()
}

// VersionTrendStat holds per-month version counts.
type VersionTrendStat struct {
	Month         string `json:"month"`
	NewVersions   int    `json:"new_versions"`
	CumulVersions int    `json:"cumul_versions"`
}

// VersionsByMonth returns total versions from enriched groups bucketed by first_published month.
// Caps individual groups at 500 versions to exclude outliers.
func VersionsByMonth() ([]VersionTrendStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			SUM(MIN(total_versions, 500)) AS new_versions
		FROM groups
		WHERE enriched_depsdev = 1 AND first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VersionTrendStat
	cumul := 0
	for rows.Next() {
		var s VersionTrendStat
		if err := rows.Scan(&s.Month, &s.NewVersions); err != nil {
			return nil, err
		}
		cumul += s.NewVersions
		s.CumulVersions = cumul
		result = append(result, s)
	}
	return result, rows.Err()
}

// --- Helpers ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
