package handler

import "net/http"

const contributorsChartHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central — Contributors</title>
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
  .stat-card .value.red { color: #ef4444; }
  .stat-card .value.blue { color: #3b82f6; }
  .stat-card .value.green { color: #10b981; }
  .stat-card .value.amber { color: #f59e0b; }
  .stat-card .label { font-size: 0.8rem; color: #64748b; }
</style>
</head>
<body>
<div class="chart-wrap">
  <h1>Solo vs Team — Contributor Counts</h1>
  <p id="status">Loading GitHub enrichment data...</p>
  <p id="sub">How many contributors does each new group's repo have? Single-committer groups (red) vs team projects (blue). New-to-GitHub authors shown as a trend line. Has AI democratised coding?</p>
  <div id="chart"></div>
</div>
<div class="summary" id="summary"></div>
<script>
const chart = echarts.init(document.getElementById('chart'), 'dark');

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
  fetch('/api/contributors')
    .then(r => {
      if (r.status === 503) throw new Error('not ready');
      return r.json();
    })
    .then(data => {
      if (!data || data.length === 0) throw new Error('empty');

      const months = data.map(d => d.month);
      const single = data.map(d => d.single_committer);
      const multi = data.map(d => d.multi_committer);
      const newAuth = data.map(d => d.new_authors);
      const pctSolo = data.map(d => d.total > 0 ? Math.round(d.single_committer / d.total * 100) : 0);

      // Linear regression on % solo
      const n = pctSolo.length;
      let sumX = 0, sumY = 0, sumXY = 0, sumX2 = 0;
      for (let i = 0; i < n; i++) {
        sumX += i; sumY += pctSolo[i]; sumXY += i * pctSolo[i]; sumX2 += i * i;
      }
      const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX * sumX);
      const intercept = (sumY - slope * sumX) / n;
      const trend = pctSolo.map((_, i) => Math.round(slope * i + intercept));

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
          type: 'value', name: '% solo',
          nameTextStyle: { color: '#64748b' },
          axisLabel: { color: '#64748b', formatter: '{value}%' },
          splitLine: { show: false },
          min: 0, max: 100,
        }],
        series: [{
          name: 'Solo committer',
          type: 'bar',
          stack: 'contribs',
          data: single,
          itemStyle: { color: '#ef4444' },
          markLine: { symbol: 'none', data: markLines, silent: true },
          barMaxWidth: 30,
        }, {
          name: 'Team (2+ contributors)',
          type: 'bar',
          stack: 'contribs',
          data: multi,
          itemStyle: { color: '#3b82f6' },
          barMaxWidth: 30,
        }, {
          name: 'New-to-GitHub authors',
          type: 'bar',
          data: newAuth,
          itemStyle: { color: '#10b981', opacity: 0.5 },
          barMaxWidth: 15,
        }, {
          name: '% solo trend',
          type: 'line',
          yAxisIndex: 1,
          data: trend,
          symbol: 'none',
          lineStyle: { color: '#f59e0b', width: 2 },
          z: 10,
        }],
        grid: { top: 50, bottom: 60, left: 50, right: 60 },
      });

      const totalSolo = single.reduce((s, v) => s + v, 0);
      const totalAll = data.reduce((s, d) => s + d.total, 0);
      const totalNew = newAuth.reduce((s, v) => s + v, 0);
      const soloPct = totalAll > 0 ? Math.round(totalSolo / totalAll * 100) : 0;

      document.getElementById('status').textContent =
        totalAll.toLocaleString() + ' groups with GitHub data';
      document.getElementById('summary').innerHTML =
        '<div class="stat-card"><div class="value red">' + soloPct + '%</div><div class="label">Solo committer rate</div></div>' +
        '<div class="stat-card"><div class="value red">' + totalSolo.toLocaleString() + '</div><div class="label">Solo committer groups</div></div>' +
        '<div class="stat-card"><div class="value blue">' + (totalAll - totalSolo).toLocaleString() + '</div><div class="label">Team projects</div></div>' +
        '<div class="stat-card"><div class="value green">' + totalNew.toLocaleString() + '</div><div class="label">New-to-GitHub authors</div></div>';

      setTimeout(update, 120000);
    })
    .catch(() => {
      document.getElementById('status').textContent = 'Waiting for GitHub enrichment data... (requires GITHUB_TOKEN)';
      setTimeout(update, 15000);
    });
}

update();
window.addEventListener('resize', () => chart.resize());
</script>
</body>
</html>`

func ContributorsChart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(contributorsChartHTML))
}
