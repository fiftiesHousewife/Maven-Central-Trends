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
		contributor_count INTEGER NOT NULL DEFAULT 0,
		primary_author TEXT NOT NULL DEFAULT '',
		author_github_created TEXT NOT NULL DEFAULT '',
		enriched_depsdev INTEGER NOT NULL DEFAULT 0,
		enriched_osv INTEGER NOT NULL DEFAULT 0,
		enriched_portal INTEGER NOT NULL DEFAULT 0,
		enriched_github INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS contributors (
		group_id TEXT NOT NULL,
		login TEXT NOT NULL,
		contributions INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (group_id, login)
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
	QualityScore        float64
	ContributorCount    int
	PrimaryAuthor       string
	AuthorGithubCreated string
	EnrichedDepsDev     bool
	EnrichedOSV         bool
	EnrichedPortal      bool
	EnrichedGithub      bool
}

// UpsertGroup inserts or updates a group row.
func UpsertGroup(g Group) error {
	_, err := db.Exec(`
		INSERT INTO groups (group_id, first_artifact, first_published, artifact_count, last_updated,
			total_versions, license, source_repo, cve_count, max_cve_severity,
			dependent_count, app_count, org_count, quality_score,
			contributor_count, primary_author, author_github_created,
			enriched_depsdev, enriched_osv, enriched_portal, enriched_github)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			contributor_count = excluded.contributor_count,
			primary_author = excluded.primary_author,
			author_github_created = excluded.author_github_created,
			enriched_depsdev = excluded.enriched_depsdev,
			enriched_osv = excluded.enriched_osv,
			enriched_portal = excluded.enriched_portal,
			enriched_github = excluded.enriched_github`,
		g.GroupID, g.FirstArtifact, g.FirstPublished, g.ArtifactCount, g.LastUpdated,
		g.TotalVersions, g.License, g.SourceRepo, g.CVECount, g.MaxCVESeverity,
		g.DependentCount, g.AppCount, g.OrgCount, g.QualityScore,
		g.ContributorCount, g.PrimaryAuthor, g.AuthorGithubCreated,
		boolToInt(g.EnrichedDepsDev), boolToInt(g.EnrichedOSV), boolToInt(g.EnrichedPortal), boolToInt(g.EnrichedGithub))
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

const groupSelectCols = `group_id, first_artifact, first_published, artifact_count, last_updated,
	total_versions, license, source_repo, cve_count, max_cve_severity,
	dependent_count, app_count, org_count, quality_score,
	contributor_count, primary_author, author_github_created,
	enriched_depsdev, enriched_osv, enriched_portal, enriched_github`

func scanGroup(scanner interface{ Scan(...any) error }) (Group, error) {
	var g Group
	var ed, eo, ep, eg int
	err := scanner.Scan(
		&g.GroupID, &g.FirstArtifact, &g.FirstPublished, &g.ArtifactCount, &g.LastUpdated,
		&g.TotalVersions, &g.License, &g.SourceRepo, &g.CVECount, &g.MaxCVESeverity,
		&g.DependentCount, &g.AppCount, &g.OrgCount, &g.QualityScore,
		&g.ContributorCount, &g.PrimaryAuthor, &g.AuthorGithubCreated,
		&ed, &eo, &ep, &eg)
	if err != nil {
		return Group{}, err
	}
	g.EnrichedDepsDev = ed == 1
	g.EnrichedOSV = eo == 1
	g.EnrichedPortal = ep == 1
	g.EnrichedGithub = eg == 1
	return g, nil
}

// GroupByID returns a single group by its ID.
func GroupByID(groupID string) (Group, error) {
	return scanGroup(db.QueryRow(`SELECT `+groupSelectCols+` FROM groups WHERE group_id = ?`, groupID))
}

// GroupExists checks whether a group_id is already in the database.
func GroupExists(groupID string) bool {
	var n int
	err := db.QueryRow("SELECT 1 FROM groups WHERE group_id = ?", groupID).Scan(&n)
	return err == nil
}

// GroupsByMonth returns groups bucketed by their first_published month (YYYY-MM).
func GroupsByMonth() (map[string][]Group, error) {
	rows, err := db.Query(`SELECT ` + groupSelectCols + ` FROM groups ORDER BY first_published`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Group)
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		if len(g.FirstPublished) >= 7 {
			result[g.FirstPublished[:7]] = append(result[g.FirstPublished[:7]], g)
		}
	}
	return result, rows.Err()
}

// GroupsForMonth returns all groups first published in a given month.
func GroupsForMonth(month string) ([]Group, error) {
	rows, err := db.Query(`SELECT `+groupSelectCols+` FROM groups WHERE first_published LIKE ? ORDER BY first_published`, month+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
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
	case "github":
		col = "enriched_github"
	}
	rows, err := db.Query(fmt.Sprintf(`SELECT `+groupSelectCols+` FROM groups WHERE %s = 0`, col))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
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

// AllGroupIDs returns all group IDs in the database.
func AllGroupIDs() ([]string, error) {
	rows, err := db.Query("SELECT group_id FROM groups ORDER BY group_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
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

// UpsertContributor stores a contributor for a group.
func UpsertContributor(groupID, login string, contributions int) error {
	_, err := db.Exec(`
		INSERT INTO contributors (group_id, login, contributions)
		VALUES (?, ?, ?)
		ON CONFLICT(group_id, login) DO UPDATE SET contributions = excluded.contributions`,
		groupID, login, contributions)
	return err
}

// UniqueContributorsByMonth returns unique new contributor counts per month.
// A contributor is "new" in the month of the earliest group they contributed to.
func UniqueContributorsByMonth() ([]MonthCount, error) {
	rows, err := db.Query(`
		SELECT substr(g.first_published, 1, 7) AS month, COUNT(DISTINCT c.login) AS cnt
		FROM contributors c
		JOIN groups g ON c.group_id = g.group_id
		WHERE g.first_published != ''
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

// UpdateGithubEnrichment updates contributor data for a group.
func UpdateGithubEnrichment(groupID string, contributorCount int, primaryAuthor, authorCreated string) error {
	_, err := db.Exec(`
		UPDATE groups SET contributor_count = ?, primary_author = ?, author_github_created = ?, enriched_github = 1
		WHERE group_id = ?`,
		contributorCount, primaryAuthor, authorCreated, groupID)
	return err
}

// ContributorStat holds per-month contributor data.
type ContributorStat struct {
	Month           string `json:"month"`
	Total           int    `json:"total"`
	SingleCommitter int    `json:"single_committer"`
	MultiCommitter  int    `json:"multi_committer"`
	NewAuthors      int    `json:"new_authors"`
}

// ContributorsByMonth returns contributor stats per month from GitHub-enriched groups.
// "New author" = GitHub account created within 12 months of the group's first publish date.
func ContributorsByMonth() ([]ContributorStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			COUNT(*) AS total,
			SUM(CASE WHEN contributor_count <= 1 THEN 1 ELSE 0 END) AS single,
			SUM(CASE WHEN contributor_count > 1 THEN 1 ELSE 0 END) AS multi,
			SUM(CASE WHEN author_github_created != '' AND
				substr(author_github_created, 1, 4) >= substr(first_published, 1, 4) - 1
				THEN 1 ELSE 0 END) AS new_authors
		FROM groups
		WHERE enriched_github = 1 AND first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ContributorStat
	for rows.Next() {
		var s ContributorStat
		if err := rows.Scan(&s.Month, &s.Total, &s.SingleCommitter, &s.MultiCommitter, &s.NewAuthors); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// CVETrendStat holds per-month CVE summary.
type CVETrendStat struct {
	Month      string         `json:"month"`
	WithCVEs   int            `json:"with_cves"`
	WithoutCVEs int           `json:"without_cves"`
	TotalCVEs  int            `json:"total_cves"`
	Severities map[string]int `json:"severities"`
}

// CVEsByMonth returns CVE stats per month from OSV-enriched groups.
func CVEsByMonth() ([]CVETrendStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			SUM(CASE WHEN cve_count > 0 THEN 1 ELSE 0 END) AS with_cves,
			SUM(CASE WHEN cve_count = 0 THEN 1 ELSE 0 END) AS without_cves,
			SUM(cve_count) AS total_cves
		FROM groups
		WHERE enriched_osv = 1 AND first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CVETrendStat
	for rows.Next() {
		var s CVETrendStat
		if err := rows.Scan(&s.Month, &s.WithCVEs, &s.WithoutCVEs, &s.TotalCVEs); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Second query for severity breakdown per month
	sevRows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month, max_cve_severity, COUNT(*) AS cnt
		FROM groups
		WHERE enriched_osv = 1 AND cve_count > 0 AND first_published != ''
		GROUP BY month, max_cve_severity
		ORDER BY month`)
	if err != nil {
		return result, nil // return what we have
	}
	defer sevRows.Close()

	sevByMonth := make(map[string]map[string]int)
	for sevRows.Next() {
		var month, sev string
		var cnt int
		if err := sevRows.Scan(&month, &sev, &cnt); err != nil {
			continue
		}
		if sevByMonth[month] == nil {
			sevByMonth[month] = make(map[string]int)
		}
		sevByMonth[month][sev] = cnt
	}

	for i := range result {
		result[i].Severities = sevByMonth[result[i].Month]
		if result[i].Severities == nil {
			result[i].Severities = make(map[string]int)
		}
	}
	return result, nil
}

// SourceRepoStat holds per-month source repo presence.
type SourceRepoStat struct {
	Month   string `json:"month"`
	WithRepo int   `json:"with_repo"`
	Without  int   `json:"without_repo"`
	Total    int   `json:"total"`
}

// SourceRepoByMonth returns % of groups with a linked source repo per month.
func SourceRepoByMonth() ([]SourceRepoStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			SUM(CASE WHEN source_repo != '' THEN 1 ELSE 0 END) AS with_repo,
			SUM(CASE WHEN source_repo = '' THEN 1 ELSE 0 END) AS without_repo,
			COUNT(*) AS total
		FROM groups
		WHERE enriched_depsdev = 1 AND first_published != ''
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SourceRepoStat
	for rows.Next() {
		var s SourceRepoStat
		if err := rows.Scan(&s.Month, &s.WithRepo, &s.Without, &s.Total); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// PopularityStat holds popularity distribution data.
type PopularityStat struct {
	Bucket    string `json:"bucket"`
	Count     int    `json:"count"`
}

// PopularityDistribution returns groups bucketed by dependent count.
func PopularityDistribution() ([]PopularityStat, error) {
	rows, err := db.Query(`
		SELECT
			CASE
				WHEN dependent_count = 0 THEN '0'
				WHEN dependent_count BETWEEN 1 AND 10 THEN '1-10'
				WHEN dependent_count BETWEEN 11 AND 100 THEN '11-100'
				WHEN dependent_count BETWEEN 101 AND 1000 THEN '101-1K'
				ELSE '1K+'
			END AS bucket,
			COUNT(*) AS cnt
		FROM groups
		WHERE enriched_portal = 1
		GROUP BY bucket
		ORDER BY MIN(dependent_count)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PopularityStat
	for rows.Next() {
		var s PopularityStat
		if err := rows.Scan(&s.Bucket, &s.Count); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// TopGroupsByDependents returns the top N groups by dependent count.
type TopGroup struct {
	GroupID        string `json:"group_id"`
	DependentCount int    `json:"dependent_count"`
	AppCount       int    `json:"app_count"`
	ArtifactCount  int    `json:"artifact_count"`
	License        string `json:"license"`
}

func TopGroupsByDependents(limit int) ([]TopGroup, error) {
	rows, err := db.Query(`
		SELECT group_id, dependent_count, app_count, artifact_count, license
		FROM groups
		WHERE enriched_portal = 1 AND dependent_count > 0
		ORDER BY dependent_count DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TopGroup
	for rows.Next() {
		var g TopGroup
		if err := rows.Scan(&g.GroupID, &g.DependentCount, &g.AppCount, &g.ArtifactCount, &g.License); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

// SizeDistribution returns groups bucketed by artifact count.
type SizeStat struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

func SizeDistribution() ([]SizeStat, error) {
	rows, err := db.Query(`
		SELECT
			CASE
				WHEN artifact_count <= 1 THEN '1'
				WHEN artifact_count BETWEEN 2 AND 5 THEN '2-5'
				WHEN artifact_count BETWEEN 6 AND 10 THEN '6-10'
				WHEN artifact_count BETWEEN 11 AND 50 THEN '11-50'
				WHEN artifact_count BETWEEN 51 AND 200 THEN '51-200'
				ELSE '200+'
			END AS bucket,
			COUNT(*) AS cnt
		FROM groups
		WHERE artifact_count > 0
		GROUP BY bucket
		ORDER BY MIN(artifact_count)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SizeStat
	for rows.Next() {
		var s SizeStat
		if err := rows.Scan(&s.Bucket, &s.Count); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// SizeByMonth returns artifact count stats per creation month.
type SizeByMonthStat struct {
	Month      string `json:"month"`
	Single     int    `json:"single"`
	Small      int    `json:"small"`
	Medium     int    `json:"medium"`
	Large      int    `json:"large"`
}

func SizeDistributionByMonth() ([]SizeByMonthStat, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			SUM(CASE WHEN artifact_count <= 1 THEN 1 ELSE 0 END) AS single,
			SUM(CASE WHEN artifact_count BETWEEN 2 AND 5 THEN 1 ELSE 0 END) AS small,
			SUM(CASE WHEN artifact_count BETWEEN 6 AND 50 THEN 1 ELSE 0 END) AS medium,
			SUM(CASE WHEN artifact_count > 50 THEN 1 ELSE 0 END) AS large
		FROM groups
		WHERE first_published != '' AND artifact_count > 0
		GROUP BY month
		ORDER BY month`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SizeByMonthStat
	for rows.Next() {
		var s SizeByMonthStat
		if err := rows.Scan(&s.Month, &s.Single, &s.Small, &s.Medium, &s.Large); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// PrefixMonthCount holds per-month new group counts by top-level prefix.
type PrefixMonthCount struct {
	Month    string         `json:"month"`
	Prefixes map[string]int `json:"prefixes"`
}

// GroupsByMonthAndPrefix returns new groups per month broken down by top-level prefix.
func GroupsByMonthAndPrefix() ([]PrefixMonthCount, error) {
	rows, err := db.Query(`
		SELECT substr(first_published, 1, 7) AS month,
			substr(group_id, 1, instr(group_id, '.') - 1) AS prefix,
			COUNT(*) AS cnt
		FROM groups
		WHERE first_published != ''
		GROUP BY month, prefix
		ORDER BY month, cnt DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byMonth := make(map[string]map[string]int)
	var months []string
	for rows.Next() {
		var month, prefix string
		var cnt int
		if err := rows.Scan(&month, &prefix, &cnt); err != nil {
			return nil, err
		}
		if byMonth[month] == nil {
			byMonth[month] = make(map[string]int)
			months = append(months, month)
		}
		byMonth[month][prefix] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var result []PrefixMonthCount
	for _, m := range months {
		result = append(result, PrefixMonthCount{Month: m, Prefixes: byMonth[m]})
	}
	return result, nil
}

// --- Helpers ---

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
