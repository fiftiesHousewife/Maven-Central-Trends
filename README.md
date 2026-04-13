# Maven Central Publishing Trends

A Go service that visualises Maven Central publishing activity over time, with a focus on whether AI coding tools have increased the rate of new library creation.

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Index page with links to all charts and API endpoints |
| `GET /health` | Health check |
| `GET /publishes-per-month` | Stacked area chart — version publishes per month by group |
| `GET /new-groups-per-month` | Bar chart — new groups per month with AI milestone annotations |
| `GET /api/publishes` | JSON data behind the versions chart |
| `GET /api/new-groups` | JSON data behind the new groups chart |
| `GET /api/new-groups/details?month=2025-07` | Groups created in a specific month |
| `GET /api/scan-progress` | Live scan progress (during background fetch) |
| `GET /api/group-popularity?namespace=com.foo` | Popularity metrics for a namespace |
| `GET /api/new-artifacts-today` | Count of new artifacts published today |

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

The server starts on `:8080`. On startup it begins background fetches to populate the data caches in `data/`. The first run takes some time (see data collection below); subsequent runs resume from cached data.

## Testing

### Go tests

```bash
make test
```

### Playwright E2E tests

The server must be running on `:8080` before running Playwright tests.

```bash
# Install (first time only)
npm install
npx playwright install chromium

# Run all E2E tests
npx playwright test

# Run a specific test
npx playwright test tests/new-chart.spec.ts

# Run with headed browser (useful for debugging)
npx playwright test --headed
```

## Data Source: deps.dev + repo1.maven.org

Both charts use the same two data sources working together:

### repo1.maven.org (Maven Central Repository)

The actual Maven Central repository at `https://repo1.maven.org/maven2/`. Provides HTML directory listings of all groups, sub-groups, and artifacts. Used for **enumeration** — discovering what exists.

- **URL pattern**: `https://repo1.maven.org/maven2/{group/path}/` (dots replaced with slashes)
- **Example**: `https://repo1.maven.org/maven2/com/google/cloud/` lists 933 artifacts
- **No authentication** required
- **No rate limits** observed
- **Always current** — this IS the repository, not an index
- **Limitations**: No search, no timestamps, HTML only (requires scraping)

### api.deps.dev (Google Open Source Insights)

Google's open-source dependency database at `https://api.deps.dev/v3alpha/`. Returns complete version history with precise publish timestamps for any Maven package. Used for **timestamping** — knowing when things were published.

- **URL pattern**: `https://api.deps.dev/v3alpha/systems/maven/packages/{groupId}%3A{artifactId}`
- **Example**: Calling with `com.google.cloud%3Agoogle-cloud-storage` returns all 312 versions with `publishedAt` in RFC 3339 format (`2026-03-25T14:22:56Z`)
- **No authentication** required
- **No rate limits** observed — handles rapid sequential requests without throttling
- **Complete history** — every version ever published, right up to today
- **CORS enabled** with 1-hour cache headers
- **Limitations**: No search or namespace enumeration (must know exact groupId:artifactId), returns 404 for parent directories that aren't real artifacts

### Why this combination?

repo1 tells us **what exists** (all groups, all artifacts), deps.dev tells us **when it was published** (precise timestamps per version). Neither can do both: repo1 has no timestamps, deps.dev has no enumeration. Together they provide complete, current, and accurate data.

### Popularity metrics: central.sonatype.com

For the interactive group drilldown, we lazy-load popularity data from the Sonatype Central Portal's internal browse API:

- **Endpoint**: `POST https://central.sonatype.com/api/internal/browse/components`
- **Fields used**: `dependentOnCount` (how many packages depend on this), `nsPopularityAppCount` (apps using this namespace)
- **Fetched on demand** when a user expands a group in the drilldown, not during the scan
- **Cached to disk** in `data/maven_popularity_cache.json` to avoid re-fetching
- **Rate limited** — the portal returns 429 after ~100 rapid requests, so lazy loading is essential
- **Undocumented internal API** — could change without notice

## How It Works

### Chart 1: Version Publishes Per Month (stacked area)

Tracks how many versions 10 selected namespaces publish each month over 4 years.

**Process:**
1. For each namespace (e.g. `io.quarkus`), scrape repo1 to list all artifact names
2. For each artifact, call deps.dev to get all versions with `publishedAt` dates
3. Bucket every version's publish date into months
4. Cache results to `data/maven_series_cache.json`; on restart, skip namespaces that already have data

**Selected namespaces:** AWS SDK, Quarkus, Google Cloud, Spring Boot, Eclipse Jetty, LangChain4j, OpenTelemetry, Testcontainers, Azure SDK, Micronaut

### Chart 2: New Groups Per Month (bar chart, ECharts)

Counts how many entirely new group namespaces (e.g. `ai.agentcord`) first appeared on Maven Central each month. Measures whether AI coding tools are driving more people to publish new libraries.

**Process:**
1. Scrape repo1 top-level directories for 26 prefixes (`ai/`, `com/`, `dev/`, `io/`, `org/`, etc.) to enumerate ~23,000 group namespaces
2. For each group, list its artifacts on repo1, pick the first one, and call deps.dev to find the earliest version's `publishedAt` date — this is when the group first appeared
3. Bucket each group's creation date into months
4. Store counts and group details (name, first artifact, artifact count, last updated)
5. The chart is interactive — click a bar to see groups created that month, grouped by prefix, with lazy-loaded popularity data

**Milestone annotations** mark key AI model and tool releases (GPT-4, Claude models, Cursor, Windsurf, Claude Code, etc.) as vertical dashed lines with labels.

**3-month moving average trend line** shown in amber to highlight the overall trajectory.

### Partial Month Handling

The current month is excluded from both charts unless it's the 30th or 31st, to avoid showing misleadingly low bars for incomplete months.

### Cache & Resume

All data is cached in `data/` (gitignored). On restart:
- The versions fetcher skips namespaces that already have data in the cache
- The new groups fetcher tracks completed prefixes in `data/maven_scan_progress.json` and resumes from where it left off
- Already-cached group IDs are checked before making any API calls, so re-scanning a prefix is fast
- Cache is written every 25 new groups to minimise data loss on crashes
- The `/api/scan-progress` endpoint exposes live scan state

| Cache file | Purpose |
|------------|---------|
| `data/maven_series_cache.json` | Version counts per month per namespace |
| `data/maven_new_groups_cache.json` | Group details keyed by month |
| `data/maven_new_cache.json` | Aggregated month counts (derived from groups cache) |
| `data/maven_scan_progress.json` | Which prefixes have been fully scanned |
| `data/maven_popularity_cache.json` | Cached popularity metrics from Central Portal |

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

### Solr Search API details

The most promising initial approach. Supports time-range queries on the `gav` core:

```
GET https://search.maven.org/solrsearch/select?q=timestamp:[1743379200000 TO 1743984000000] AND g:org.apache.*&core=gav&rows=0&wt=json
```

Key issues discovered:
- The `timestamp` field uses epoch milliseconds (not ISO dates as the docs imply)
- The `ga` core's timestamp reflects last update, not creation — useless for finding new artifacts
- The index stopped being updated for most groups around June 2025 (only `org.apache.*` continues)
- Rate limiting returns the full Maven Central HTML page instead of JSON, causing silent parse failures where counts appear as 0 rather than errors
- Maximum 20 rows per page with no server-side grouping, making deduplication of ~50k GAVs per week impractical

### Central Portal API details

The newer Sonatype Central Portal at `central.sonatype.com` has an undocumented internal API:

- **Components endpoint** (`POST /api/internal/browse/components`): Returns components sorted by `publishedDate`, but this is the latest version's date — not the creation date. For a query "what was published in October 2024", all components have newer timestamps (they've been updated since), so the count is always 0 for historical months.
- **Versions endpoint** (`GET /api/internal/browse/component/versions`): Returns individual versions with `publishedEpochMillis`, but sort by `publishedDate` is broken — results come back in seemingly random order regardless of sort parameters.
- Both endpoints cap at 20 results per page and return 429 after approximately 100 rapid requests with a 60-90 second cooldown.
