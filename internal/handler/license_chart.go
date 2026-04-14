package handler

import "net/http"

const licenseChartHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central — License Trends</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
  h1 { font-size: 1.4rem; margin-bottom: 0.5rem; }
  p#status { color: #94a3b8; font-size: 0.9rem; margin-bottom: 1rem; }
  .chart-wrap { position: relative; width: 100%; max-width: 1200px; margin: 0 auto; }
  #chart { width: 100%; height: 550px; }
  .summary { max-width: 1200px; margin: 1.5rem auto 0; display: flex; gap: 1rem; flex-wrap: wrap; }
  .stat-card {
    background: #1e293b; border-radius: 8px; padding: 0.8rem 1.2rem; flex: 1; min-width: 150px;
  }
  .stat-card .value { font-size: 1.5rem; font-weight: 700; color: #3b82f6; }
  .stat-card .label { font-size: 0.8rem; color: #64748b; }
  #detail-panel {
    margin-top: 1.5rem; max-width: 1200px; margin-left: auto; margin-right: auto;
    display: none;
  }
  #detail-panel h2 { font-size: 1.1rem; margin-bottom: 0.75rem; }
  #detail-panel .close { float: right; cursor: pointer; color: #94a3b8; font-size: 1.2rem; }
  #detail-panel .close:hover { color: #e2e8f0; }
  .detail-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 0.5rem; }
  .detail-item {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 0.6rem 0.8rem; display: flex; flex-direction: column; gap: 0.2rem;
    transition: border-color 0.15s;
  }
  .detail-item:hover { border-color: #3b82f6; }
  .detail-item .top { display: flex; justify-content: space-between; align-items: center; }
  .detail-item .name {
    color: #e2e8f0; font-weight: 600; font-size: 0.85rem;
    text-decoration: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .detail-item .name:hover { color: #3b82f6; }
  .detail-item .badge {
    display: inline-block; font-size: 0.7rem; padding: 1px 6px;
    border-radius: 3px; background: #1e3a5f; color: #60a5fa; white-space: nowrap;
  }
  .detail-item .meta { font-size: 0.75rem; color: #94a3b8; }
</style>
</head>
<body>
<div class="chart-wrap">
  <h1>License Distribution — New Groups Per Month</h1>
  <p id="status">Loading enrichment data...</p>
  <div id="chart"></div>
</div>
<div class="summary" id="summary"></div>
<div id="detail-panel">
  <span class="close" onclick="document.getElementById('detail-panel').style.display='none'">&times;</span>
  <h2 id="detail-title"></h2>
  <div id="detail-list" class="detail-list"></div>
</div>
<script>
const chart = echarts.init(document.getElementById('chart'), 'dark');

const palette = [
  '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6',
  '#06b6d4', '#ec4899', '#84cc16', '#f97316', '#6366f1',
  '#14b8a6', '#e879f9', '#a3e635', '#fb923c', '#818cf8',
];

let topLicensesList = [];

function showDetail(month, license) {
  fetch('/api/new-groups/details?month=' + month)
    .then(r => r.json())
    .then(groups => {
      if (!groups || groups.length === 0) return;
      // Filter by license if clicked on a specific segment
      let filtered = groups;
      let title = month;
      if (license && license !== 'Other') {
        filtered = groups.filter(g => g.license === license);
        title += ' — ' + license;
      } else if (license === 'Other') {
        const topSet = new Set(topLicensesList);
        filtered = groups.filter(g => g.license && !topSet.has(g.license));
        title += ' — Other licenses';
      }
      filtered.sort((a, b) => a.group_id.localeCompare(b.group_id));
      document.getElementById('detail-title').textContent =
        title + ' — ' + filtered.length + ' groups';
      document.getElementById('detail-list').innerHTML = filtered.map(g => {
        const ns = g.group_id.replace(/\./g, '/');
        const url = 'https://repo1.maven.org/maven2/' + ns + '/';
        return '<div class="detail-item">' +
          '<div class="top"><a class="name" href="' + url + '" target="_blank">' + g.group_id + '</a>' +
          (g.license ? '<span class="badge">' + g.license + '</span>' : '') + '</div>' +
          '<div class="meta">' + g.first_artifact + ' — ' + g.first_published +
          (g.artifact_count > 0 ? ' — ' + g.artifact_count + ' artifacts' : '') + '</div>' +
          '</div>';
      }).join('');
      document.getElementById('detail-panel').style.display = 'block';
      document.getElementById('detail-panel').scrollIntoView({ behavior: 'smooth' });
    });
}

chart.on('click', function(params) {
  if (params.componentType === 'series') {
    showDetail(params.name, params.seriesName);
  }
});

function update() {
  fetch('/api/license-trends')
    .then(r => {
      if (r.status === 503) throw new Error('not ready');
      return r.json();
    })
    .then(data => {
      if (!data || data.length === 0) throw new Error('empty');

      const totals = {};
      data.forEach(m => {
        Object.entries(m.licenses).forEach(([lic, cnt]) => {
          totals[lic] = (totals[lic] || 0) + cnt;
        });
      });

      const sorted = Object.entries(totals).sort((a, b) => b[1] - a[1]);
      const topN = 8;
      const topLicenses = sorted.slice(0, topN).map(e => e[0]);
      topLicensesList = topLicenses;
      const topSet = new Set(topLicenses);

      const months = data.map(m => m.month);

      const seriesData = {};
      topLicenses.forEach(l => { seriesData[l] = []; });
      seriesData['Other'] = [];

      data.forEach(m => {
        let otherCount = 0;
        topLicenses.forEach(l => {
          seriesData[l].push(m.licenses[l] || 0);
        });
        Object.entries(m.licenses).forEach(([lic, cnt]) => {
          if (!topSet.has(lic)) otherCount += cnt;
        });
        seriesData['Other'].push(otherCount);
      });

      const allKeys = [...topLicenses, 'Other'];
      const series = allKeys.map((name, i) => ({
        name: name,
        type: 'bar',
        stack: 'licenses',
        data: seriesData[name],
        itemStyle: { color: palette[i % palette.length] },
        emphasis: { focus: 'series' },
      }));

      chart.setOption({
        backgroundColor: 'transparent',
        tooltip: {
          trigger: 'axis',
          axisPointer: { type: 'shadow' },
        },
        legend: {
          show: true,
          top: 5,
          right: 20,
          textStyle: { color: '#94a3b8', fontSize: 11 },
        },
        xAxis: {
          type: 'category',
          data: months,
          axisLabel: { rotate: 45, color: '#64748b', fontSize: 11 },
          axisLine: { lineStyle: { color: '#1e293b' } },
        },
        yAxis: {
          type: 'value',
          axisLabel: { color: '#64748b' },
          splitLine: { lineStyle: { color: '#1e293b' } },
        },
        series: series,
        grid: { top: 50, bottom: 60, left: 50, right: 20 },
      });

      const grandTotal = Object.values(totals).reduce((s, v) => s + v, 0);
      const topLicense = sorted[0];
      const uniqueCount = sorted.length;
      document.getElementById('status').textContent =
        grandTotal.toLocaleString() + ' enriched groups with license data — click any bar segment for details';
      document.getElementById('summary').innerHTML =
        '<div class="stat-card"><div class="value">' + topLicense[0] + '</div><div class="label">Most common license (' + Math.round(topLicense[1]/grandTotal*100) + '%)</div></div>' +
        '<div class="stat-card"><div class="value">' + uniqueCount + '</div><div class="label">Distinct licenses</div></div>' +
        '<div class="stat-card"><div class="value">' + grandTotal.toLocaleString() + '</div><div class="label">Groups with license data</div></div>';

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

func LicenseChart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(licenseChartHTML))
}
