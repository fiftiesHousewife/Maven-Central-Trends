package handler

import "net/http"

const repoChartHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central — Source Repo Presence</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
  h1 { font-size: 1.4rem; margin-bottom: 0.5rem; }
  p#status { color: #94a3b8; font-size: 0.9rem; margin-bottom: 0.25rem; }
  p#sub { color: #475569; font-size: 0.8rem; margin-bottom: 1rem; }
  .chart-wrap { position: relative; width: 100%; max-width: 1200px; margin: 0 auto; }
  #chart { width: 100%; height: 550px; }
  .summary { max-width: 1200px; margin: 1.5rem auto 0; display: flex; gap: 1rem; flex-wrap: wrap; }
  .stat-card {
    background: #1e293b; border-radius: 8px; padding: 0.8rem 1.2rem; flex: 1; min-width: 150px;
  }
  .stat-card .value { font-size: 1.5rem; font-weight: 700; }
  .stat-card .value.blue { color: #3b82f6; }
  .stat-card .value.green { color: #10b981; }
  .stat-card .value.grey { color: #64748b; }
  .stat-card .label { font-size: 0.8rem; color: #64748b; }
</style>
</head>
<body>
<div class="chart-wrap">
  <h1>Source Repository Presence</h1>
  <p id="status">Loading enrichment data...</p>
  <p id="sub">What percentage of new groups link to a source repository (typically GitHub)? Stacked bars show groups with vs without a repo link from deps.dev metadata.</p>
  <div id="chart"></div>
</div>
<div class="summary" id="summary"></div>
<div id="detail-panel" style="display:none; margin-top:1.5rem; max-width:1200px; margin-left:auto; margin-right:auto;">
  <span style="float:right;cursor:pointer;color:#94a3b8;font-size:1.2rem" onclick="document.getElementById('detail-panel').style.display='none'">&times;</span>
  <h2 id="detail-title" style="font-size:1.1rem;margin-bottom:0.75rem"></h2>
  <div id="detail-list" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(340px,1fr));gap:0.5rem"></div>
</div>
<script>
const chart = echarts.init(document.getElementById('chart'), 'dark');

function showDetail(month) {
  fetch('/api/new-groups/details?month=' + month)
    .then(r => r.json())
    .then(groups => {
      if (!groups) return;
      groups.sort((a, b) => (b.source_repo ? 1 : 0) - (a.source_repo ? 1 : 0));
      const withRepo = groups.filter(g => g.source_repo).length;
      document.getElementById('detail-title').textContent =
        month + ' — ' + withRepo + '/' + groups.length + ' with source repo';
      document.getElementById('detail-list').innerHTML = groups.map(g => {
        const ns = g.group_id.replace(/\./g, '/');
        const url = 'https://repo1.maven.org/maven2/' + ns + '/';
        const repoLink = g.source_repo ? '<a href="' + g.source_repo + '" target="_blank" style="color:#3b82f6;font-size:0.75rem;text-decoration:none">' + g.source_repo.replace('https://github.com/', '') + '</a>' : '<span style="color:#64748b;font-size:0.75rem">No repo linked</span>';
        return '<div style="background:#1e293b;border:1px solid #334155;border-radius:8px;padding:0.6rem 0.8rem">' +
          '<div style="display:flex;justify-content:space-between"><a href="' + url + '" target="_blank" style="color:#e2e8f0;font-weight:600;font-size:0.85rem;text-decoration:none">' + g.group_id + '</a></div>' +
          '<div>' + repoLink + '</div></div>';
      }).join('');
      document.getElementById('detail-panel').style.display = 'block';
      document.getElementById('detail-panel').scrollIntoView({ behavior: 'smooth' });
    });
}

chart.on('click', function(params) {
  if (params.componentType === 'series') showDetail(params.name);
});

const milestones = [
  { month: '2022-06', label: 'Copilot GA', color: '#06b6d4' },
  { month: '2023-03', label: 'GPT-4', color: '#f59e0b' },
  { month: '2024-03', label: 'Claude 3 / Devin', color: '#8b5cf6' },
  { month: '2024-08', label: 'Cursor AI', color: '#06b6d4' },
  { month: '2025-05', label: 'Claude 4', color: '#8b5cf6' },
  { month: '2025-11', label: 'Opus 4.5 / CC', color: '#8b5cf6' },
  { month: '2026-03', label: 'Opus 4.6', color: '#8b5cf6' },
];

function update() {
  fetch('/api/source-repos')
    .then(r => {
      if (r.status === 503) throw new Error('not ready');
      return r.json();
    })
    .then(data => {
      if (!data || data.length === 0) throw new Error('empty');

      const months = data.map(d => d.month);
      const withRepo = data.map(d => d.with_repo);
      const without = data.map(d => d.without_repo);
      const pct = data.map(d => d.total > 0 ? Math.round(d.with_repo / d.total * 100) : 0);

      // Linear regression on %
      const n = pct.length;
      let sumX = 0, sumY = 0, sumXY = 0, sumX2 = 0;
      for (let i = 0; i < n; i++) {
        sumX += i; sumY += pct[i]; sumXY += i * pct[i]; sumX2 += i * i;
      }
      const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX * sumX);
      const intercept = (sumY - slope * sumX) / n;
      const trend = pct.map((_, i) => Math.round(slope * i + intercept));

      const markLines = milestones
        .filter(m => months.includes(m.month))
        .map((m, i) => ({
          xAxis: m.month,
          label: {
            formatter: m.label, position: 'end', rotate: 0,
            fontSize: 10, fontWeight: 'bold', color: m.color,
            backgroundColor: '#0f172a', padding: [2, 4], borderRadius: 3,
            offset: [0, (i % 3) * 14],
          },
          lineStyle: { color: m.color, type: 'dashed', width: 1, opacity: 0.4 },
        }));

      chart.setOption({
        backgroundColor: 'transparent',
        tooltip: {
          trigger: 'axis',
          formatter: params => {
            let html = params[0].name;
            params.forEach(p => {
              if (p.value != null) {
                const suffix = p.seriesName.includes('%') ? '%' : '';
                html += '<br/>' + p.marker + ' ' + p.seriesName + ': ' + p.value.toLocaleString() + suffix;
              }
            });
            return html;
          },
        },
        legend: {
          show: true, top: 5, right: 20,
          textStyle: { color: '#94a3b8', fontSize: 11 },
        },
        xAxis: {
          type: 'category', data: months,
          axisLabel: { rotate: 45, color: '#64748b', fontSize: 11 },
          axisLine: { lineStyle: { color: '#1e293b' } },
        },
        yAxis: [{
          type: 'value',
          axisLabel: { color: '#64748b' },
          splitLine: { lineStyle: { color: '#1e293b' } },
        }, {
          type: 'value', name: '% with repo',
          nameTextStyle: { color: '#64748b' },
          axisLabel: { color: '#64748b', formatter: '{value}%' },
          splitLine: { show: false },
          min: 0, max: 100,
        }],
        series: [{
          name: 'With source repo',
          type: 'bar',
          stack: 'repos',
          data: withRepo,
          itemStyle: { color: '#3b82f6' },
          markLine: { symbol: 'none', data: markLines, silent: true },
          barMaxWidth: 30,
        }, {
          name: 'No repo linked',
          type: 'bar',
          stack: 'repos',
          data: without,
          itemStyle: { color: '#334155' },
          barMaxWidth: 30,
        }, {
          name: '% with repo',
          type: 'line',
          yAxisIndex: 1,
          data: pct,
          symbol: 'none',
          lineStyle: { color: '#10b981', width: 1, type: 'dotted' },
          z: 10,
        }, {
          name: 'Trend',
          type: 'line',
          yAxisIndex: 1,
          data: trend,
          symbol: 'none',
          lineStyle: { color: '#f59e0b', width: 2 },
          z: 10,
        }],
        grid: { top: 50, bottom: 60, left: 50, right: 60 },
      });

      const totalWith = withRepo.reduce((s, v) => s + v, 0);
      const totalAll = data.reduce((s, d) => s + d.total, 0);
      const overallPct = Math.round(totalWith / totalAll * 100);
      const recentPct = Math.round(data.slice(-6).reduce((s, d) => s + (d.total > 0 ? d.with_repo / d.total * 100 : 0), 0) / Math.min(6, data.length));

      document.getElementById('status').textContent =
        totalAll.toLocaleString() + ' enriched groups analysed';
      document.getElementById('summary').innerHTML =
        '<div class="stat-card"><div class="value blue">' + totalWith.toLocaleString() + '</div><div class="label">Groups with source repo</div></div>' +
        '<div class="stat-card"><div class="value green">' + overallPct + '%</div><div class="label">Overall with repo</div></div>' +
        '<div class="stat-card"><div class="value green">' + recentPct + '%</div><div class="label">Last 6 months</div></div>' +
        '<div class="stat-card"><div class="value grey">' + (totalAll - totalWith).toLocaleString() + '</div><div class="label">No repo linked</div></div>';

      setTimeout(update, 120000);
    })
    .catch(() => {
      document.getElementById('status').textContent = 'Waiting for enrichment data...';
      setTimeout(update, 15000);
    });
}

update();
window.addEventListener('resize', () => chart.resize());
</script>
</body>
</html>`

func SourceRepoChart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(repoChartHTML))
}
