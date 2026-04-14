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
  .card {
    display: block; text-decoration: none; color: inherit;
    background: #1e293b; border-radius: 8px; padding: 1.2rem 1.5rem;
    margin-bottom: 1rem; transition: background 0.15s;
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
  <p class="sub">Visualising Maven Central publishing activity and the impact of AI coding tools</p>

  <a class="card" href="/publishes-per-month">
    <h2>Version Publishes Per Month</h2>
    <p>Stacked area chart tracking 10 namespaces (AWS SDK, Quarkus, Google Cloud, Spring Boot, LangChain4j, etc.) over 3 years. Data from deps.dev + repo1.maven.org.</p>
  </a>

  <a class="card" href="/new-groups-per-month">
    <h2>New Groups Per Month</h2>
    <p>Bar chart counting brand-new Maven Central namespaces each month, with a one-and-done % overlay showing single-version abandonment rate. Click any bar to explore groups.</p>
  </a>

  <a class="card" href="/license-trends">
    <h2>License Trends</h2>
    <p>Stacked bar chart showing how license choices evolve over time. Top 8 licenses broken out, the rest grouped as Other. Powered by deps.dev enrichment.</p>
  </a>

  <a class="card" href="/artifact-trends">
    <h2>Maven Central Growth</h2>
    <p>Cumulative groups and artifacts over time with monthly additions. Click any bar to drill into that month's new groups, sorted by artifact count.</p>
  </a>

  <a class="card" href="/version-trends">
    <h2>Version Trends</h2>
    <p>Total versions published by all groups per month from deps.dev enrichment. Are groups publishing more or fewer versions over time? Click any bar to explore.</p>
  </a>

  <div class="api">
    <h2>API Endpoints</h2>
    <ul>
      <li><a href="/health">/health</a> <span>— health check</span></li>
      <li><a href="/api/publishes">/api/publishes</a> <span>— version publishes JSON</span></li>
      <li><a href="/api/new-groups">/api/new-groups</a> <span>— new groups per month JSON</span></li>
      <li><a href="/api/new-groups/details">/api/new-groups/details</a> <span>— all new group details</span></li>
      <li><a href="/api/new-groups/details?month=2026-03">/api/new-groups/details?month=2026-03</a> <span>— groups for a specific month</span></li>
    </ul>
  </div>
</div>
</body>
</html>`

func Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}
