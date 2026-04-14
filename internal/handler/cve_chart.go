package handler

import "net/http"

const cveChartHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central — Security / CVEs</title>
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
  .stat-card .value.amber { color: #f59e0b; }
  .stat-card .value.green { color: #10b981; }
  .stat-card .value.blue { color: #3b82f6; }
  .stat-card .label { font-size: 0.8rem; color: #64748b; }
</style>
</head>
<body>
<div class="chart-wrap">
  <h1>Security — Known CVEs in New Groups</h1>
  <p id="status">Loading enrichment data...</p>
  <p id="sub">Groups with known vulnerabilities from the OSV database, broken down by maximum severity. Based on the group's primary artifact.</p>
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
      const withCVEs = groups.filter(g => g.cve_count > 0).sort((a, b) => b.cve_count - a.cve_count);
      document.getElementById('detail-title').textContent =
        month + ' — ' + withCVEs.length + ' groups with CVEs (of ' + groups.length + ' total)';
      document.getElementById('detail-list').innerHTML = (withCVEs.length > 0 ? withCVEs : groups.slice(0, 50)).map(g => {
        const ns = g.group_id.replace(/\./g, '/');
        const url = 'https://repo1.maven.org/maven2/' + ns + '/';
        const sev = g.max_cve_severity || '';
        const sevColor = sev === 'CRITICAL' || sev === 'HIGH' ? '#fca5a5' : '#fcd34d';
        return '<div style="background:#1e293b;border:1px solid #334155;border-radius:8px;padding:0.6rem 0.8rem">' +
          '<div style="display:flex;justify-content:space-between"><a href="' + url + '" target="_blank" style="color:#e2e8f0;font-weight:600;font-size:0.85rem;text-decoration:none">' + g.group_id + '</a>' +
          (g.cve_count > 0 ? '<span style="color:' + sevColor + ';font-size:0.8rem;font-weight:600">' + g.cve_count + ' CVEs (' + sev + ')</span>' : '<span style="color:#10b981;font-size:0.8rem">Clean</span>') +
          '</div></div>';
      }).join('');
      document.getElementById('detail-panel').style.display = 'block';
      document.getElementById('detail-panel').scrollIntoView({ behavior: 'smooth' });
    });
}

chart.on('click', function(params) {
  if (params.componentType === 'series') showDetail(params.name);
});

const sevColors = {
  'CRITICAL': '#dc2626',
  'HIGH': '#ef4444',
  'MODERATE': '#f59e0b',
  'LOW': '#facc15',
};

function update() {
  fetch('/api/cve-trends')
    .then(r => {
      if (r.status === 503) throw new Error('not ready');
      return r.json();
    })
    .then(data => {
      if (!data || data.length === 0) throw new Error('empty');

      const months = data.map(d => d.month);
      const sevKeys = ['CRITICAL', 'HIGH', 'MODERATE', 'LOW'];

      const series = sevKeys.map(sev => ({
        name: sev,
        type: 'bar',
        stack: 'cves',
        data: data.map(d => d.severities[sev] || 0),
        itemStyle: { color: sevColors[sev] },
        emphasis: { focus: 'series' },
        barMaxWidth: 30,
      }));

      // % with CVEs trend line
      const pctWithCVEs = data.map(d => {
        const total = d.with_cves + d.without_cves;
        return total > 0 ? Math.round(d.with_cves / total * 1000) / 10 : 0;
      });

      series.push({
        name: '% with CVEs',
        type: 'line',
        yAxisIndex: 1,
        data: pctWithCVEs,
        symbol: 'none',
        lineStyle: { color: '#e2e8f0', width: 2 },
        z: 10,
      });

      chart.setOption({
        backgroundColor: 'transparent',
        tooltip: {
          trigger: 'axis',
          formatter: params => {
            let html = params[0].name;
            params.forEach(p => {
              if (p.value != null && p.value > 0) {
                const suffix = p.seriesName.includes('%') ? '%' : '';
                html += '<br/>' + p.marker + ' ' + p.seriesName + ': ' + p.value + suffix;
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
          type: 'value', name: 'Groups with CVEs',
          nameTextStyle: { color: '#64748b' },
          axisLabel: { color: '#64748b' },
          splitLine: { lineStyle: { color: '#1e293b' } },
        }, {
          type: 'value', name: '% affected',
          nameTextStyle: { color: '#64748b' },
          axisLabel: { color: '#64748b', formatter: '{value}%' },
          splitLine: { show: false },
          min: 0,
        }],
        series: series,
        grid: { top: 50, bottom: 60, left: 60, right: 60 },
      });

      const totalWithCVEs = data.reduce((s, d) => s + d.with_cves, 0);
      const totalGroups = data.reduce((s, d) => s + d.with_cves + d.without_cves, 0);
      const totalCVEs = data.reduce((s, d) => s + d.total_cves, 0);
      const overallPct = totalGroups > 0 ? (totalWithCVEs / totalGroups * 100).toFixed(1) : 0;

      document.getElementById('status').textContent =
        totalGroups.toLocaleString() + ' groups scanned for vulnerabilities';
      document.getElementById('summary').innerHTML =
        '<div class="stat-card"><div class="value red">' + totalWithCVEs.toLocaleString() + '</div><div class="label">Groups with CVEs</div></div>' +
        '<div class="stat-card"><div class="value amber">' + totalCVEs.toLocaleString() + '</div><div class="label">Total CVEs found</div></div>' +
        '<div class="stat-card"><div class="value blue">' + overallPct + '%</div><div class="label">% groups affected</div></div>' +
        '<div class="stat-card"><div class="value green">' + (totalGroups - totalWithCVEs).toLocaleString() + '</div><div class="label">Clean groups</div></div>';

      setTimeout(update, 120000);
    })
    .catch(() => {
      document.getElementById('status').textContent = 'Waiting for OSV enrichment data...';
      setTimeout(update, 15000);
    });
}

update();
window.addEventListener('resize', () => chart.resize());
</script>
</body>
</html>`

func CVEChart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(cveChartHTML))
}
