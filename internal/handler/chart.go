package handler

import "net/http"

const chartHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central — Versions Published Per Month</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
  h1 { font-size: 1.4rem; margin-bottom: 0.5rem; }
  p#status { color: #94a3b8; font-size: 0.9rem; margin-bottom: 1.5rem; }
  .chart-wrap { position: relative; width: 100%; max-width: 1200px; margin: 0 auto; }
  canvas { width: 100% !important; height: 500px !important; }
</style>
</head>
<body>
<div class="chart-wrap">
  <h1>Maven Central — Versions Published Per Month, Last 3 Years</h1>
  <p id="status">Loading data… fetching from deps.dev.</p>
  <canvas id="chart"></canvas>
</div>
<script>
const colors = [
  '#3b82f6', '#ef4444', '#10b981', '#f59e0b', '#8b5cf6',
  '#ec4899', '#06b6d4', '#f97316', '#84cc16', '#6366f1'
];

let chart = null;

function update() {
  fetch('/api/publishes')
    .then(r => {
      if (r.status === 503) throw new Error('still fetching');
      return r.json();
    })
    .then(data => {
      const { groups, months } = data;
      const groupsDone = groups.filter(g =>
        months.some(m => (m.groups[g.namespace] || 0) > 0)
      ).length;

      document.getElementById('status').textContent = groupsDone >= groups.length
        ? groups.length + ' groups loaded — data from deps.dev + repo1.maven.org'
        : 'Fetching… ' + groupsDone + '/' + groups.length + ' groups loaded';

      const labels = months.map(m => m.month);
      const datasets = groups.map((g, i) => ({
        label: g.label,
        data: months.map(m => Math.max(0, m.groups[g.namespace] || 0)),
        borderColor: colors[i % colors.length],
        backgroundColor: colors[i % colors.length] + '88',
        fill: true,
        pointRadius: 2,
        tension: 0.3,
        borderWidth: 1.5,
      }));

      if (!chart) {
        chart = new Chart(document.getElementById('chart'), {
          type: 'line',
          data: { labels, datasets },
          options: {
            responsive: true,
            animation: false,
            interaction: { intersect: false, mode: 'index' },
            scales: {
              x: {
                ticks: { color: '#64748b', maxTicksLimit: 18 },
                grid: { color: '#1e293b' },
              },
              y: {
                stacked: true,
                beginAtZero: true,
                ticks: { color: '#64748b' },
                grid: { color: '#1e293b' },
              }
            },
            plugins: {
              legend: {
                reverse: true,
                labels: { color: '#e2e8f0', usePointStyle: true, pointStyle: 'rect' }
              },
              tooltip: {
                itemSort: (a, b) => b.datasetIndex - a.datasetIndex,
                callbacks: {
                  label: ctx => ctx.dataset.label + ': ' + ctx.raw.toLocaleString() + ' versions'
                }
              }
            }
          }
        });
      } else {
        chart.data.labels = labels;
        groups.forEach((g, i) => {
          chart.data.datasets[i].data = months.map(m => Math.max(0, m.groups[g.namespace] || 0));
        });
        chart.update();
      }

      if (groupsDone < groups.length) setTimeout(update, 10000);
    })
    .catch(() => {
      setTimeout(update, 5000);
    });
}

update();
</script>
</body>
</html>`

func Chart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(chartHTML))
}
