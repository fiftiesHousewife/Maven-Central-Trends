# Maven Central Publishing Trends

A Go service that visualises Maven Central publishing activity over time, with a focus on whether AI coding tools have increased the rate of new library creation.

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Index page with links to all charts and API endpoints |
| `GET /health` | Health check |
| `GET /publishes-per-month` | Stacked area chart — version publishes per month for 10 selected namespaces |
| `GET /new-groups-per-month` | Bar chart — new groups per month with AI milestones and one-and-done overlay |
| `GET /license-trends` | Stacked bar — license distribution per month (from deps.dev enrichment) |
| `GET /artifact-trends` | Bar + cumulative line — new artifacts per month across all groups |
| `GET /version-trends` | Bar + cumulative line — version counts per month (from deps.dev enrichment) |
| `GET /api/publishes` | JSON data behind the versions chart |
| `GET /api/new-groups` | JSON data behind the new groups chart |
| `GET /api/new-groups/details?month=2025-07` | Groups created in a specific month |
| `GET /api/scan-progress` | Live scan and enrichment progress |
| `GET /api/group-popularity?namespace=com.foo` | Popularity metrics for a namespace |
| `GET /api/new-artifacts-today` | Count of new artifacts published today (Solr) |
| `GET /api/license-trends` | License distribution per month JSON |
| `GET /api/one-and-done` | Single-version vs multi-version group counts per month |
| `GET /api/growth` | New artifacts per month with cumulative totals |
| `GET /api/version-trends` | Version counts per month with cumulative totals |

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
| `version_publishes` | Monthly version counts for 10 selected namespaces |
| `scan_progress` | Completed prefix scans for resume support |

### Background tasks

Three goroutines start on boot:

1. **Version series fetch** — For 10 selected namespaces (AWS SDK, Spring Boot, etc.), scrape repo1 for artifacts, call deps.dev for version dates, bucket by month
2. **New groups scan** — Enumerate 26 top-level prefixes on repo1, discover groups, call deps.dev for first-publish dates. Tracks completed prefixes for resume
3. **Enrichment pipeline** — Runs after the scan completes, three sequential phases:
   - **deps.dev detail**: Fetch license, source repo, total version count (100ms throttle)
   - **OSV CVEs**: Query api.osv.dev for vulnerability data (100ms throttle)
   - **Central Portal**: Fetch popularity metrics from Sonatype (3s throttle, 60s backoff on 429)

### Charts

All charts use a dark theme and include AI tool milestone annotations. All are clickable — click any bar to drill into the groups created that month.

| Chart | Library | Data source |
|-------|---------|-------------|
| Version publishes (stacked area) | Chart.js | 10 selected namespaces via deps.dev |
| New groups per month | ECharts | All 26 prefixes, with one-and-done stacked overlay |
| License trends | ECharts | deps.dev enrichment (top 8 licenses + Other) |
| Artifact trends | ECharts | All groups, outliers capped at 500 artifacts |
| Version trends | ECharts | deps.dev enrichment, outliers capped at 500 versions |

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

### Chart 1: Version Publishes Per Month (stacked area)

Tracks how many versions 10 selected namespaces publish each month over 4 years.

**Process:**
1. For each namespace (e.g. `io.quarkus`), scrape repo1 to list all artifact names
2. For each artifact, call deps.dev to get all versions with `publishedAt` dates
3. Bucket every version's publish date into months, store in SQLite

**Selected namespaces:** AWS SDK, Quarkus, Google Cloud, Spring Boot, Eclipse Jetty, LangChain4j, OpenTelemetry, Testcontainers, Azure SDK, Micronaut

### Chart 2: New Groups Per Month (bar chart with one-and-done overlay)

Counts how many new group namespaces first appeared each month. Bars are stacked: red = one-and-done (single version only), blue = active (2+ versions), grey = not yet enriched.

**Process:**
1. Scrape repo1 top-level directories for 26 prefixes to enumerate ~19,000 group namespaces
2. For each group, call deps.dev to find the earliest version's `publishedAt` date
3. Bucket by month; enrichment adds version counts for one-and-done classification
4. Click any bar to see groups created that month, grouped by prefix, with lazy-loaded popularity data

### Charts 3-5: Enrichment charts

Built from the three enrichment phases:
- **License trends**: Groups with deps.dev license data, top 8 licenses stacked per month
- **Artifact trends**: Total artifacts per month across all groups (outliers capped at 500)
- **Version trends**: Total versions per month from deps.dev enrichment (outliers capped at 500)

All enrichment charts feature milestone annotations, 3-month moving averages, and click-to-detail.

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
