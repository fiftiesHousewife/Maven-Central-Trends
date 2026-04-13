# Maven Central Publishing Trends

A Go service that visualises Maven Central publishing activity over time, with a focus on whether AI coding tools have increased the rate of new library creation.

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Index page with links to all charts and API endpoints |
| `GET /health` | Health check |
| `GET /publishes-per-month` | Stacked area chart — version publishes per month by group |
| `GET /new-groups-per-month` | Bar chart — new Maven Central groups created per month (clickable) |
| `GET /api/publishes` | JSON data behind the versions chart |
| `GET /api/new-groups` | JSON data behind the new groups chart |
| `GET /api/new-groups/details?month=2025-07` | List of groups created in a specific month |
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

The server starts on `:8080`. On startup it begins background fetches to populate the data caches. The first run takes some time (see data collection below); subsequent runs resume from cached data.

## How It Works

### Chart 1: Version Publishes Per Month (stacked area)

Tracks how many versions 10 selected namespaces publish each month over 3 years.

**Data sources:**
- **repo1.maven.org** — directory listing scraped to enumerate all artifacts under each namespace (e.g. `repo1.maven.org/maven2/io/quarkus/` lists ~191 artifacts)
- **api.deps.dev** (Google Open Source Insights) — called per artifact to retrieve every published version with its `publishedAt` timestamp

**Process:**
1. For each namespace (e.g. `io.quarkus`), scrape repo1 to list all artifact names
2. For each artifact, call `GET https://api.deps.dev/v3alpha/systems/maven/packages/{group}%3A{artifact}` to get all versions with dates
3. Bucket every version's publish date into months
4. Cache results to `maven_series_cache.json`; on restart, skip namespaces that already have data

**Selected namespaces:** AWS SDK, Quarkus, Google Cloud, Spring Boot, Eclipse Jetty, LangChain4j, OpenTelemetry, Testcontainers, Azure SDK, Micronaut

### Chart 2: New Groups Per Month (bar chart)

Counts how many entirely new group namespaces (e.g. `ai.agentcord`) first appeared on Maven Central each month. This measures whether AI coding tools are driving more people to publish new libraries.

**Data sources:** Same as above (repo1 + deps.dev)

**Process:**
1. Scrape repo1 top-level directories for 26 prefixes (`ai/`, `com/`, `dev/`, `io/`, `org/`, etc.) to enumerate ~23,000 group namespaces
2. For each group, list its artifacts on repo1, pick the first one, and call deps.dev to find the earliest version's `publishedAt` date — this is when the group first appeared
3. Bucket each group's creation date into months
4. Store both counts (`maven_new_cache.json`) and group details with artifact names (`maven_new_groups_cache.json`)
5. The chart is interactive — click a bar to see the actual groups created that month

**Colour coding:** Blue bars = pre-2024, green bars = 2024 onwards (AI coding tools era). A vertical annotation marks the boundary.

### Partial Month Handling

The current month is excluded from both charts unless it's the 30th or 31st, to avoid showing misleadingly low bars for incomplete months.

### Cache & Resume

Both fetchers write intermediate results to JSON cache files. On restart:
- The versions fetcher skips namespaces that already have data
- The new groups fetcher skips entirely if the cache is populated
- Cache files: `maven_series_cache.json`, `maven_new_cache.json`, `maven_new_groups_cache.json`

## Other Approaches Attempted

Several data sources were tried before settling on the deps.dev + repo1 combination.

### Maven Central Solr Search API (`search.maven.org/solrsearch/select`)

The original approach. Supports time-range queries on the `gav` core using epoch millisecond timestamps:

```
GET https://search.maven.org/solrsearch/select?q=timestamp:[START TO END] AND g:org.apache.*&core=gav&rows=0&wt=json
```

**Problems:**
- The Solr index is **severely stale for most groups after June 2025**. Only `org.apache.*` artifacts are indexed up to date. All other namespaces (Google Cloud, Spring, Eclipse, etc.) stop at mid-2025.
- The `ga` core's `timestamp` field reflects the last update, not creation date, making it useless for finding when artifacts were first published.
- The `ga` core doesn't support filtering by `versionCount` to find new artifacts.
- Rate limiting returns HTML instead of JSON, causing silent failures (counts appear as 0 instead of errors).
- Maximum 20 rows per page, making it impractical to page through large result sets for deduplication.

**Verdict:** Reliable for historical data (pre-June 2025) for all groups, and current data for Apache-prefixed groups only. Not viable as a sole data source.

### Sonatype Central Portal API (`central.sonatype.com/api/internal/browse/`)

The newer Maven Central portal has an undocumented internal API.

**Components endpoint** (`POST /api/internal/browse/components`):
- Supports namespace filtering and sort by `publishedDate`
- Returns the **latest version** timestamp per component, not historical data
- Cannot answer "what was published in October 2024" because it only knows each component's current state
- Max page size of 20, aggressive rate limiting (429 after ~100 rapid requests, 60-90s cooldown)

**Versions endpoint** (`GET /api/internal/browse/component/versions`):
- Returns individual versions with `publishedEpochMillis`
- Sort by `publishedDate` is **broken** — results come back in random order regardless of sort parameters
- Namespace-only queries also return random-ordered results

**Verdict:** Has current data but cannot query historical time ranges. Rate limits make it impractical for large-scale enumeration. Being an internal API, it could break without notice.

### Hybrid Approach (Solr + Central Portal)

Attempted using Solr for pre-June 2025 data and Central Portal for recent months. The cutoff was set at June 2025 based on when each group's Solr index stopped updating.

**Problems:**
- Central Portal can only count components whose **latest** version was published in the target month — for historical months, all components have newer timestamps so the count is always 0
- Only works for the current month, creating a ~9-month gap between Solr coverage and Central Portal usefulness

### mvnrepository.com

Returns HTTP 403 for all programmatic requests. No public API exists.

### libraries.io

Requires API key authentication. Maven data historically lags behind and does not reliably track publish dates at the version level.

### deps.dev + repo1 (current approach)

Chosen because:
- **Complete data**: deps.dev has every version ever published with precise timestamps, right up to today
- **No authentication**: Both APIs are free and keyless
- **No rate limits observed**: deps.dev handles rapid sequential requests without throttling
- **Accurate timestamps**: `publishedAt` in RFC 3339 format, not epoch milliseconds subject to timezone bugs
- **Simple enumeration**: repo1 directory listings provide a reliable source of truth for what exists

**Trade-offs:**
- Requires one HTTP call per artifact to deps.dev (~934 calls for `com.google.cloud` alone)
- repo1 directory entries sometimes include parent directories that aren't real artifacts (produce 404s from deps.dev)
- First full scan of ~23,000 groups takes significant time; cached results serve instantly
