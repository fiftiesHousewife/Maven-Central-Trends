# Maven Central Publishing Trends

A Go service that visualises Maven Central publishing activity over time, with a focus on whether AI coding tools have increased the rate of new library creation.

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Index page with links to all charts and API endpoints |
| `GET /health` | Health check |
| `GET /new-groups-per-month` | New groups per month with one-and-done overlay |
| `GET /publishes-per-month` | New groups by prefix (stacked area, top 10 + Other) |
| `GET /license-trends` | License distribution per month |
| `GET /artifact-trends` | New artifacts per month with trend line |
| `GET /version-trends` | Version counts per month with trend line |
| `GET /cve-trends` | CVEs by severity per month |
| `GET /source-repos` | Source repo presence over time |
| `GET /popularity` | Popularity distribution + top 25 groups |
| `GET /size-distribution` | Group size (artifact count) distribution over time |
| `GET /api/new-groups` | New groups per month JSON |
| `GET /api/new-groups/details?month=2025-07` | Groups created in a specific month |
| `GET /api/scan-progress` | Live scan and enrichment progress |
| `GET /api/groups-by-prefix` | New groups by prefix per month |
| `GET /api/license-trends` | License distribution per month |
| `GET /api/one-and-done` | Single-version vs multi-version per month |
| `GET /api/growth` | New artifacts per month |
| `GET /api/version-trends` | Version counts per month |
| `GET /api/cve-trends` | CVE stats per month |
| `GET /api/source-repos` | Source repo presence per month |
| `GET /api/popularity` | Popularity distribution + top groups |
| `GET /api/size-distribution` | Group size distribution by month |
| `GET /api/group-popularity?namespace=com.foo` | On-demand popularity for a namespace |
| `GET /api/new-artifacts-today` | New artifacts today (Solr) |

## Running

```bash
# Install Go
brew install go

# Run directly
make run

# Or build and run in background
make build
nohup ./bin/server > server.log 2>&1 &
```

The server starts on `:8080` (override with `PORT` env var). Set `LOG_LEVEL=debug` for verbose logging. On startup it begins background fetches to populate the SQLite database. The first run takes some time; subsequent runs resume from stored data.

## Testing

### Go tests

```bash
make test

# With coverage
go test ./... -cover
```

Coverage: config 100%, middleware 100%, store 84%, handler 19% (handler includes background goroutines that call external APIs).

### Playwright E2E tests

The server must be running on `:8080` before running Playwright tests.

```bash
npm install
npx playwright install chromium

npx playwright test
npx playwright test --headed  # debug with visible browser
```

## Architecture

### Data storage

All data is stored in a SQLite database at `data/maven.db` (WAL mode). Legacy JSON cache files are automatically migrated to SQLite on first run.

| Table | Purpose |
|-------|---------|
| `groups` | All discovered Maven Central namespaces with enrichment data |
| `scan_progress` | Completed prefix scans for resume support |

### Background tasks

Two goroutines start on boot:

1. **New groups scan** — Enumerate 26 top-level prefixes on repo1, discover groups, call deps.dev for first-publish dates. Tracks completed prefixes for resume
2. **Enrichment pipeline** — Runs after the scan completes, three sequential phases:
   - **deps.dev detail**: Fetch license, source repo, total version count (100ms throttle)
   - **OSV CVEs**: Query api.osv.dev for vulnerability data across up to 5 artifacts per group (100ms throttle)
   - **Central Portal**: Fetch popularity metrics from Sonatype (3s throttle, 60s backoff on 429)

### Charts

All charts use a dark theme and include AI tool milestone annotations. All are clickable — click any bar to drill into the groups created that month.

| Chart | Data source |
|-------|-------------|
| New groups per month | All 26 prefixes, with one-and-done stacked overlay |
| New groups by prefix | Stacked area, top 10 prefixes + Other |
| License trends | deps.dev enrichment (top 8 licenses + Other) |
| Artifact trends | All groups, outliers capped at 500 |
| Version trends | deps.dev enrichment, outliers capped at 500 |
| CVE trends | OSV enrichment, severity breakdown per month |
| Source repo presence | deps.dev enrichment, with/without repo stacked |
| Popularity distribution | Portal enrichment, log-scale histogram + top 25 |
| Group size distribution | Artifact count buckets per month |

All charts use ECharts with a dark theme, AI tool milestone annotations, and click-to-detail drill-down panels.

## Data Sources

### repo1.maven.org (Maven Central Repository)

The actual Maven Central repository at `https://repo1.maven.org/maven2/`. Provides HTML directory listings of all groups, sub-groups, and artifacts. Used for **enumeration** — discovering what exists.

- **URL pattern**: `https://repo1.maven.org/maven2/{group/path}/`
- **No authentication** or rate limits
- **Always current** — this IS the repository
- **Limitations**: No search, no timestamps, HTML only (requires scraping)

### api.deps.dev (Google Open Source Insights)

Google's open-source dependency database at `https://api.deps.dev/v3alpha/`. Returns complete version history with precise publish timestamps. Used for **timestamping** and **enrichment**.

- **URL pattern**: `https://api.deps.dev/v3alpha/systems/maven/packages/{groupId}%3A{artifactId}`
- **No authentication** or rate limits observed
- **Complete history** — every version with `publishedAt` in RFC 3339 format
- **Limitations**: No namespace enumeration (must know exact groupId:artifactId)

### api.osv.dev (OSV Vulnerability Database)

Google's open-source vulnerability database. Returns CVE data for Maven packages.

- **Endpoint**: `POST https://api.osv.dev/v1/query`
- **Fields used**: CVE count, maximum severity (CRITICAL, HIGH, MODERATE, LOW)
- **No authentication** required

### central.sonatype.com (Sonatype Central Portal)

Popularity metrics from the Sonatype Central Portal's internal browse API.

- **Endpoint**: `POST https://central.sonatype.com/api/internal/browse/components`
- **Fields used**: `dependentOnCount`, `nsPopularityAppCount`, `nsPopularityOrgCount`
- **Rate limited** — returns 429 after ~100 rapid requests
- **Undocumented internal API** — could change without notice

## How It Works

### New Groups Per Month (bar chart with one-and-done overlay)

Counts how many new group namespaces first appeared each month. Bars are stacked: red = one-and-done (single version only), blue = active (2+ versions), grey = not yet enriched.

**Process:**
1. Scrape repo1 top-level directories for 26 prefixes to enumerate ~19,000 group namespaces
2. For each group, call deps.dev to find the earliest version's `publishedAt` date
3. Bucket by month; enrichment adds version counts for one-and-done classification
4. Click any bar to see groups created that month, grouped by prefix, with lazy-loaded popularity data

### Enrichment charts

Built from the three enrichment phases. All feature milestone annotations and click-to-detail:

- **Groups by prefix**: Stacked area showing top 10 prefixes (com, org, io, dev, etc.) + Other
- **License trends**: Top 8 licenses stacked per month, click any segment to filter
- **Artifact trends**: Total artifacts per month across all groups (outliers capped at 500)
- **Version trends**: Total versions per month from deps.dev enrichment (outliers capped at 500)
- **CVE trends**: Groups with known vulnerabilities by severity, % affected trend line
- **Source repo presence**: % of groups linking to a source repository over time
- **Popularity distribution**: Log-scale histogram of dependent counts + top 25 groups
- **Group size distribution**: Artifact count buckets (1 / 2-5 / 6-50 / 50+) stacked per month

### Partial Month Handling

The current month is excluded from all charts unless it's the 30th or 31st, to avoid misleadingly low bars for incomplete months.

### Resume & Progress

Data is stored in SQLite with prefix-level checkpointing. On restart:
- Version fetcher skips namespaces already in the DB
- Group scanner resumes from the last incomplete prefix
- Enrichment resumes from unenriched groups (`enriched_*` flags)
- The `/api/scan-progress` endpoint exposes live scan and enrichment state

## Other Approaches Attempted

Several data sources were evaluated before settling on deps.dev + repo1. All had significant limitations:

| Source | Data Current? | Historical? | Namespace Filter? | Rate Limits | Verdict |
|--------|:---:|:---:|:---:|:---:|---------|
| **Solr Search API** (`search.maven.org`) | Only for `org.apache.*` | Yes (pre-June 2025) | Wildcard `g:prefix.*` | Aggressive (returns HTML) | Index stale for most groups after June 2025 |
| **Central Portal Components** (`central.sonatype.com`) | Yes | No (latest version only) | Exact namespace | 429 after ~100 requests | Cannot query historical time ranges |
| **Central Portal Versions** | Yes | Broken sort | Exact namespace | Same as above | Sort by `publishedDate` returns random order |
| **Hybrid Solr + Central Portal** | Partial | Partial | Both | Both | 9-month gap between Solr coverage and Portal usefulness |
| **mvnrepository.com** | Yes (website) | Yes (website) | N/A | HTTP 403 for all API calls | No programmatic access |
| **libraries.io** | Lags behind | Unreliable timestamps | Requires API key | Auth required | Maven data quality too low |
| **repo1.maven.org alone** | Yes | No timestamps | Manual path traversal | None | No publish dates, only directory listings |
| **deps.dev alone** | Yes | Yes | No enumeration | None observed | Cannot discover what exists, only look up known packages |
| **deps.dev + repo1** | **Yes** | **Yes** | **Via repo1 scraping** | **None observed** | **Chosen approach** |
