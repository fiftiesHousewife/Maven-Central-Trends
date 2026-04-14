package handler

import "net/http"

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central Trends</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
  .wrap { max-width: 800px; margin: 0 auto; }
  h1 { font-size: 1.6rem; margin-bottom: 0.5rem; }
  p.sub { color: #94a3b8; margin-bottom: 2rem; }
  h3 { font-size: 0.85rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; margin: 1.5rem 0 0.5rem; }
  .card {
    display: block; text-decoration: none; color: inherit;
    background: #1e293b; border-radius: 8px; padding: 1.2rem 1.5rem;
    margin-bottom: 0.75rem; transition: background 0.15s;
  }
  .card:hover { background: #334155; }
  .card h2 { font-size: 1.1rem; color: #3b82f6; margin-bottom: 0.3rem; }
  .card p { color: #94a3b8; font-size: 0.9rem; }
  .api { margin-top: 2.5rem; }
  .api h2 { font-size: 1.1rem; margin-bottom: 0.75rem; }
  .api a { color: #3b82f6; text-decoration: none; font-size: 0.85rem; font-family: monospace; }
  .api a:hover { text-decoration: underline; }
  .api li { margin-bottom: 0.4rem; list-style: none; }
  .api li span { color: #64748b; font-size: 0.8rem; }
</style>
</head>
<body>
<div class="wrap">
  <h1>Maven Central Trends</h1>
  <p class="sub">Tracking ~19,000 Maven Central namespaces to measure how AI coding tools affect library creation. Data from repo1.maven.org, deps.dev, OSV, and Sonatype Central Portal.</p>

  <h3>Groups</h3>

  <a class="card" href="/new-groups-per-month">
    <h2>New Groups Per Month</h2>
    <p>New Maven Central namespaces each month. Bars split into one-and-done (single version, red) vs active (blue). Click any bar to explore the groups created that month.</p>
  </a>

  <a class="card" href="/publishes-per-month">
    <h2>New Groups By Prefix</h2>
    <p>Stacked area chart breaking down new groups by top-level prefix (com, org, io, dev, ai, etc.). Top 10 prefixes shown, the rest grouped as Other.</p>
  </a>

  <a class="card" href="/license-trends">
    <h2>License Trends</h2>
    <p>How license choices evolve over time. Top 8 licenses stacked per month. Click any segment to see the groups with that license.</p>
  </a>

  <h3>Artifacts</h3>

  <a class="card" href="/artifact-trends">
    <h2>Artifacts Per Month</h2>
    <p>New artifacts added to Maven Central each month across all groups, with a linear trend line. Outlier groups (500+ artifacts) are capped. Click any bar for details.</p>
  </a>

  <h3>Versions</h3>

  <a class="card" href="/version-trends">
    <h2>Versions Per Month</h2>
    <p>Total versions from deps.dev enrichment, bucketed by group creation month. Outlier groups capped at 500 versions. Click any bar to explore.</p>
  </a>

  <h3>Security &amp; Quality</h3>

  <a class="card" href="/cve-trends">
    <h2>Known CVEs</h2>
    <p>Groups with known vulnerabilities from the OSV database, stacked by severity (Critical, High, Moderate, Low). Shows what % of new groups ship with known CVEs.</p>
  </a>

  <a class="card" href="/source-repos">
    <h2>Source Repository Presence</h2>
    <p>What % of groups link to a source repo? Stacked bars with a trend line showing whether repo linkage is rising or falling over time.</p>
  </a>

  <a class="card" href="/popularity">
    <h2>Popularity Distribution</h2>
    <p>How many dependents do groups have? Log-scale histogram from Portal enrichment, plus the top 25 most-depended-on groups.</p>
  </a>

  <a class="card" href="/size-distribution">
    <h2>Group Size Distribution</h2>
    <p>How many artifacts does each group contain? Stacked bars per month showing the mix of single-artifact vs multi-artifact groups.</p>
  </a>

  <div class="api">
    <h2>API Endpoints</h2>
    <ul>
      <li><a href="/health">/health</a> <span>— health check</span></li>
      <li><a href="/api/scan-progress">/api/scan-progress</a> <span>— live scan and enrichment progress</span></li>
      <li><a href="/api/new-groups">/api/new-groups</a> <span>— new groups per month</span></li>
      <li><a href="/api/new-groups/details?month=2026-03">/api/new-groups/details?month=2026-03</a> <span>— groups for a specific month</span></li>
      <li><a href="/api/groups-by-prefix">/api/groups-by-prefix</a> <span>— new groups by prefix per month</span></li>
      <li><a href="/api/license-trends">/api/license-trends</a> <span>— license distribution per month</span></li>
      <li><a href="/api/one-and-done">/api/one-and-done</a> <span>— one-version vs multi-version per month</span></li>
      <li><a href="/api/growth">/api/growth</a> <span>— new artifacts per month</span></li>
      <li><a href="/api/version-trends">/api/version-trends</a> <span>— version counts per month</span></li>
    </ul>
  </div>
</div>
</body>
</html>`

func Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}
