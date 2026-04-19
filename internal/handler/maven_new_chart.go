package handler

import "net/http"

const newChartHTML2 = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maven Central — New Groups Per Month</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; padding: 2rem; }
  h1 { font-size: 1.4rem; margin-bottom: 0.5rem; }
  p#status { color: #94a3b8; font-size: 0.9rem; margin-bottom: 0.25rem; }
  p#scan-progress { color: #475569; font-size: 0.75rem; margin-bottom: 1rem; }
  .toggle-bar { display: flex; gap: 0.5rem; margin-bottom: 1rem; }
  .toggle-btn {
    padding: 0.35rem 0.8rem; border-radius: 4px; border: 1px solid #334155;
    background: #1e293b; color: #94a3b8; font-size: 0.8rem; cursor: pointer;
    transition: all 0.15s;
  }
  .toggle-btn:hover { border-color: #3b82f6; color: #e2e8f0; }
  .toggle-btn.active { background: #3b82f6; border-color: #3b82f6; color: #fff; }
  .chart-wrap { position: relative; width: 100%; max-width: 1200px; margin: 0 auto; }
  #chart { width: 100%; height: 550px; }
  #groups-panel {
    margin-top: 1.5rem; max-width: 1200px; margin-left: auto; margin-right: auto;
    display: none;
  }
  #groups-panel h2 { font-size: 1.1rem; margin-bottom: 0.75rem; }
  #groups-panel .close { float: right; cursor: pointer; color: #94a3b8; font-size: 1.2rem; }
  #groups-panel .close:hover { color: #e2e8f0; }
  .group-list { display: flex; flex-direction: column; gap: 0.5rem; }
  .prefix-header {
    display: flex; justify-content: space-between; align-items: center;
    background: #1e293b; border-radius: 6px; padding: 0.6rem 1rem;
    cursor: pointer; user-select: none;
  }
  .prefix-header:hover { background: #334155; }
  .prefix-name { color: #e2e8f0; font-weight: 600; font-size: 0.95rem; }
  .prefix-items {
    display: none; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 0.5rem; padding: 0.5rem 0 0.5rem 1rem;
  }
  .prefix-items.expanded { display: grid; }
  .prefix-count { color: #64748b; font-size: 0.85rem; }
  .group-item {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 0.6rem 0.8rem; display: flex; flex-direction: column; gap: 0.25rem;
    transition: border-color 0.15s;
  }
  .group-item:hover { border-color: #3b82f6; }
  .group-item .top-row {
    display: flex; justify-content: space-between; align-items: center;
  }
  .group-item .name {
    color: #e2e8f0; font-weight: 600; font-size: 0.85rem;
    text-decoration: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .group-item .name:hover { color: #3b82f6; }
  .group-item .date {
    color: #64748b; font-size: 0.72rem; white-space: nowrap; margin-left: 0.5rem;
  }
  .group-item .meta {
    display: flex; gap: 0.75rem; flex-wrap: wrap;
    font-size: 0.75rem; color: #94a3b8;
  }
  .group-item .meta .label { color: #64748b; }
  .group-item .badges { display: flex; gap: 0.35rem; flex-wrap: wrap; }
  .group-item .badge {
    display: inline-block; font-size: 0.7rem; padding: 1px 6px;
    border-radius: 3px; background: #334155; color: #94a3b8;
    text-decoration: none; white-space: nowrap;
  }
  .group-item .badge.license { background: #1e3a5f; color: #60a5fa; }
  .group-item .badge.cve-high { background: #7f1d1d; color: #fca5a5; }
  .group-item .badge.cve-mod { background: #78350f; color: #fcd34d; }
  .group-item .badge.repo { background: #1e293b; color: #64748b; border: 1px solid #334155; }
  .group-item .badge.repo:hover { color: #e2e8f0; border-color: #3b82f6; }
  .group-item .meta .sep { color: #475569; margin: 0 0.15rem; }
  .group-item .popularity { color: #10b981; font-size: 0.75rem; min-height: 1em; }
</style>
</head>
<body>
<div class="chart-wrap">
  <h1>New Maven Central Groups Created Per Month</h1>
  <p id="status">Loading data… scanning Maven Central namespaces.</p>
  <p id="scan-progress"></p>
  <div class="toggle-bar">
    <button class="toggle-btn active" id="btn-all" onclick="setFilter('all')">All</button>
    <button class="toggle-btn" id="btn-new" onclick="setFilter('new')">Truly new namespaces</button>
    <button class="toggle-btn" id="btn-ext" onclick="setFilter('extensions')">Extensions of existing</button>
  </div>
  <div id="chart"></div>
</div>
<div id="groups-panel">
  <span class="close" onclick="document.getElementById('groups-panel').style.display='none'">&times;</span>
  <h2 id="groups-title"></h2>
  <div id="groups-list" class="group-list"></div>
</div>
<script>
const chart = echarts.init(document.getElementById('chart'), 'dark');

const milestones = [
  { month: '2022-06', label: 'GitHub Copilot GA', color: '#06b6d4' },
  { month: '2023-03', label: 'GPT-4', color: '#f59e0b' },
  { month: '2023-07', label: 'Claude 2', color: '#8b5cf6' },
  { month: '2024-03', label: 'Claude 3 Opus / Devin', color: '#8b5cf6' },
  { month: '2024-06', label: 'Claude 3.5 Sonnet', color: '#8b5cf6' },
  { month: '2024-08', label: 'Cursor AI', color: '#06b6d4' },
  { month: '2024-10', label: 'Claude 3.5 Sonnet v2', color: '#8b5cf6' },
  { month: '2025-01', label: 'Windsurf', color: '#06b6d4' },
  { month: '2025-02', label: 'Claude 3.7 Sonnet', color: '#8b5cf6' },
  { month: '2025-05', label: 'Sonnet 4 / Opus 4', color: '#8b5cf6' },
  { month: '2025-08', label: 'Opus 4.1', color: '#8b5cf6' },
  { month: '2025-09', label: 'Sonnet 4.5', color: '#8b5cf6' },
  { month: '2025-10', label: 'Haiku 4.5', color: '#8b5cf6' },
  { month: '2025-11', label: 'Opus 4.5 / Claude Code', color: '#8b5cf6' },
  { month: '2026-03', label: 'Opus 4.6 / Sonnet 4.6', color: '#8b5cf6' },
];

const popCache = {};

function renderGroup(g) {
  const ns = g.group_id.replace(/\./g, '/');
  const url = 'https://repo1.maven.org/maven2/' + ns + '/';

  // Badges row
  const badges = [];
  if (g.total_versions > 0) badges.push('<span class="badge">' + g.total_versions + ' versions</span>');
  if (g.license) badges.push('<span class="badge license">' + g.license + '</span>');
  if (g.cve_count > 0) {
    const sevClass = (g.max_cve_severity === 'CRITICAL' || g.max_cve_severity === 'HIGH') ? 'cve-high' : 'cve-mod';
    badges.push('<span class="badge ' + sevClass + '">' + g.cve_count + ' CVE' + (g.cve_count > 1 ? 's' : '') + ' (' + g.max_cve_severity + ')</span>');
  }
  if (g.source_repo) {
    const repoName = g.source_repo.replace('https://github.com/', '');
    badges.push('<a class="badge repo" href="' + g.source_repo + '" target="_blank">' + repoName + '</a>');
  }

  // Meta row
  const meta = [];
  meta.push('<span>' + g.first_artifact + '</span>');
  if (g.artifact_count > 0) meta.push('<span>' + g.artifact_count + ' artifacts</span>');
  if (g.last_updated && g.last_updated !== g.first_published) {
    meta.push('<span>updated ' + g.last_updated + '</span>');
  }

  return '<div class="group-item" data-group-id="' + g.group_id + '">' +
    '<div class="top-row">' +
      '<a class="name" href="' + url + '" target="_blank">' + g.group_id + '</a>' +
      '<span class="date">' + g.first_published + '</span>' +
    '</div>' +
    (badges.length ? '<div class="badges">' + badges.join('') + '</div>' : '') +
    '<div class="meta">' + meta.join('<span class="sep">&middot;</span>') + '</div>' +
    '<span class="popularity"></span>' +
  '</div>';
}

function fetchPopularity(groupId) {
  if (popCache[groupId]) return Promise.resolve(popCache[groupId]);
  return fetch('/api/group-popularity?namespace=' + groupId)
    .then(r => r.json())
    .then(d => { popCache[groupId] = d; return d; })
    .catch(() => null);
}

// IntersectionObserver to lazy-load popularity when groups scroll into view
const popObserver = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      const groupId = entry.target.dataset.groupId;
      if (groupId) {
        fetchPopularity(groupId).then(d => {
          const el = entry.target.querySelector('.popularity');
          if (!d || !el || el.textContent) return;
          const parts = [];
          if (d.dependent_count > 0) parts.push(d.dependent_count.toLocaleString() + ' dependents');
          if (d.app_count > 0) parts.push(d.app_count.toLocaleString() + ' apps');
          if (parts.length) el.textContent = ' — ' + parts.join(', ');
        });
      }
      popObserver.unobserve(entry.target);
    }
  });
}, { rootMargin: '200px' });

// Build a hierarchy tree from flat group list relative to a parent path
function buildTree(groups, parentPath) {
  const depth = parentPath ? parentPath.split('.').length : 0;
  const children = {};

  groups.forEach(g => {
    const parts = g.group_id.split('.');
    if (parts.length <= depth) return;
    const nextPart = parts[depth];
    const childKey = parentPath ? parentPath + '.' + nextPart : nextPart;
    if (!children[childKey]) children[childKey] = { key: childKey, label: nextPart, groups: [], totalArtifacts: 0 };
    children[childKey].groups.push(g);
    children[childKey].totalArtifacts += g.artifact_count || 0;
  });

  return Object.values(children).sort((a, b) => b.groups.length - a.groups.length);
}

let currentMonth = '';
let allGroupsForMonth = [];

function renderLevel(parentPath) {
  const nodes = buildTree(allGroupsForMonth, parentPath);
  const title = parentPath
    ? parentPath + '.* — ' + nodes.reduce((s, n) => s + n.groups.length, 0) + ' groups'
    : currentMonth + ' — ' + allGroupsForMonth.length + ' new groups';
  document.getElementById('groups-title').textContent = title;

  // Breadcrumb
  let breadcrumb = '';
  if (parentPath) {
    const parts = parentPath.split('.');
    breadcrumb = '<div style="margin-bottom:0.75rem;font-size:0.8rem">';
    breadcrumb += '<span style="cursor:pointer;color:#3b82f6" onclick="renderLevel(\'\')">' + currentMonth + '</span>';
    for (let i = 0; i < parts.length; i++) {
      const path = parts.slice(0, i + 1).join('.');
      breadcrumb += ' <span style="color:#475569">/</span> ';
      if (i < parts.length - 1) {
        breadcrumb += '<span style="cursor:pointer;color:#3b82f6" onclick="renderLevel(\'' + path + '\')">' + parts[i] + '</span>';
      } else {
        breadcrumb += '<span style="color:#e2e8f0">' + parts[i] + '</span>';
      }
    }
    breadcrumb += '</div>';
  }

  document.getElementById('groups-list').innerHTML = breadcrumb + nodes.map(node => {
    // Check if this node has deeper children or is a leaf
    const hasChildren = node.groups.some(g => g.group_id !== node.key && g.group_id.startsWith(node.key + '.'));
    const directMatch = node.groups.find(g => g.group_id === node.key);
    const subgroupCount = node.groups.filter(g => g.group_id !== node.key).length;

    const ns = node.key.replace(/\./g, '/');
    const url = 'https://repo1.maven.org/maven2/' + ns + '/';

    // Depth distribution within this node
    const depthCounts = {};
    node.groups.forEach(g => {
      const d = g.group_id.split('.').length;
      depthCounts[d] = (depthCounts[d] || 0) + 1;
    });
    const depthColors = { 2: '#3b82f6', 3: '#10b981', 4: '#f59e0b', 5: '#ef4444', 6: '#8b5cf6' };
    const depthLabels = { 2: 'Level 2', 3: 'Level 3', 4: 'Level 4', 5: 'Level 5', 6: 'Level 6' };
    const depthBar = Object.keys(depthCounts).sort().map(d => {
      const pct = Math.max(2, Math.round(depthCounts[d] / node.groups.length * 100));
      return '<span title="Depth ' + d + ': ' + depthCounts[d] + ' groups" style="display:inline-block;height:6px;width:' + pct + '%;background:' + (depthColors[d] || '#64748b') + ';border-radius:2px"></span>';
    }).join('');
    const depthLegend = Object.keys(depthCounts).sort().map(d =>
      '<span style="color:' + (depthColors[d] || '#64748b') + ';font-size:0.65rem">' + (depthLabels[d] || 'L'+d) + ':' + depthCounts[d] + '</span>'
    ).join(' ');

    // Stats line
    const stats = [];
    if (directMatch && directMatch.first_published) stats.push(directMatch.first_published);
    if (subgroupCount > 0) stats.push(subgroupCount + ' subgroup' + (subgroupCount !== 1 ? 's' : ''));
    stats.push(node.totalArtifacts + ' artifact' + (node.totalArtifacts !== 1 ? 's' : ''));
    if (directMatch && directMatch.license) stats.push(directMatch.license);

    // Badges
    const badges = [];
    if (directMatch && directMatch.cve_count > 0) {
      const sevClass = (directMatch.max_cve_severity === 'CRITICAL' || directMatch.max_cve_severity === 'HIGH') ? 'cve-high' : 'cve-mod';
      badges.push('<span class="badge ' + sevClass + '">' + directMatch.cve_count + ' CVEs</span>');
    }
    if (directMatch && directMatch.total_versions > 0) {
      badges.push('<span class="badge">' + directMatch.total_versions + ' ver</span>');
    }

    const clickable = hasChildren || subgroupCount > 0;
    const onclick = clickable ? ' onclick="renderLevel(\'' + node.key + '\')" style="cursor:pointer"' : '';

    return '<div class="group-item"' + onclick + '>' +
      '<div class="top-row">' +
        '<span class="name"' + (clickable ? ' style="color:#3b82f6"' : '') + '>' +
          node.key + (clickable ? ' >' : '') +
        '</span>' +
        '<span class="date">' + node.groups.length + ' group' + (node.groups.length !== 1 ? 's' : '') + '</span>' +
      '</div>' +
      (node.groups.length > 1 ? '<div style="display:flex;gap:1px;margin:2px 0;border-radius:3px;overflow:hidden;max-width:120px">' + depthBar + '</div>' +
        '<div style="display:flex;gap:0.5rem">' + depthLegend + '</div>' : '') +
      (badges.length ? '<div class="badges">' + badges.join('') + '</div>' : '') +
      '<div class="meta">' + stats.join('<span class="sep"> &middot; </span>') + '</div>' +
    '</div>';
  }).join('');
}

function showGroups(month) {
  fetch('/api/new-groups/details?month=' + month)
    .then(r => r.json())
    .then(groups => {
      if (!groups || groups.length === 0) {
        document.getElementById('groups-title').textContent = month + ' — no group data yet';
        document.getElementById('groups-list').innerHTML = '<p style="color:#94a3b8">Data still loading…</p>';
      } else {
        currentMonth = month;
        allGroupsForMonth = groups;
        renderLevel('');
      }
      document.getElementById('groups-panel').style.display = 'block';
      document.getElementById('groups-panel').scrollIntoView({ behavior: 'smooth' });
    });
}

chart.on('click', function(params) {
  if (params.componentType === 'series') {
    showGroups(params.name);
  }
});

let currentFilter = 'all';

function setFilter(f) {
  currentFilter = f;
  ['all', 'new', 'ext'].forEach(id => {
    document.getElementById('btn-' + id).className = 'toggle-btn' + (
      (id === 'all' && f === 'all') || (id === 'new' && f === 'new') || (id === 'ext' && f === 'extensions')
      ? ' active' : '');
  });
  update();
}

function update() {
  const filterParam = currentFilter === 'new' ? '?filter=new' : '';
  fetch('/api/new-groups' + filterParam)
    .then(r => {
      if (r.status === 503) throw new Error('still fetching');
      return r.json();
    })
    .then(data => {
      const { months } = data;
      const total = months.reduce((s, m) => s + m.new_groups, 0);

      document.getElementById('status').textContent = total > 0
        ? total.toLocaleString() + ' new groups discovered'
        : 'Loading data…';

      fetch('/api/scan-progress').then(r => r.json()).then(sp => {
        if (sp.scanning) {
          const pct = sp.prefix_total > 0 ? Math.round(sp.prefix_done / sp.prefix_total * 100) : 0;
          document.getElementById('scan-progress').textContent =
            'Scanning ' + sp.current_prefix + '/* — ' +
            sp.prefix_done.toLocaleString() + '/' + sp.prefix_total.toLocaleString() +
            ' (' + pct + '%) — ' +
            sp.completed_prefixes + '/' + sp.total_prefixes + ' prefixes complete';
        } else if (sp.enrichment_phase) {
          const pct = sp.enrichment_total > 0 ? Math.round(sp.enrichment_done / sp.enrichment_total * 100) : 0;
          document.getElementById('scan-progress').textContent =
            'Enriching: ' + sp.enrichment_phase + ' — ' +
            sp.enrichment_done.toLocaleString() + '/' + sp.enrichment_total.toLocaleString() +
            ' (' + pct + '%)';
        } else {
          document.getElementById('scan-progress').textContent = '';
        }
      }).catch(() => {});

      const labels = months.map(m => m.month);
      const values = months.map(m => m.new_groups);

      const markLines = milestones
        .filter(m => labels.includes(m.month))
        .map((m, i) => ({
          xAxis: m.month,
          label: {
            formatter: m.label,
            position: 'end',
            rotate: 0,
            fontSize: 11,
            fontWeight: 'bold',
            color: m.color,
            backgroundColor: '#0f172a',
            padding: [2, 4],
            borderRadius: 3,
            offset: [0, (i % 3) * 16],
          },
          lineStyle: { color: m.color, type: 'dashed', width: 1, opacity: 0.4 },
        }));

      // 3-month moving average for trend line
      const window = 3;
      const trend = values.map((v, i) => {
        if (i < window - 1) return null;
        let sum = 0;
        for (let j = i - window + 1; j <= i; j++) sum += values[j];
        return Math.round(sum / window);
      });

      // Fetch one-and-done data to split bars — only for 'all' view
      const oadPromise = currentFilter === 'all'
        ? fetch('/api/one-and-done').then(r => r.ok ? r.json() : null)
        : Promise.resolve(null);
      oadPromise.then(oad => {
        let series, tooltipFmt;

        if (oad && oad.length > 0) {
          // Build lookup: month -> { one_version, multiple }
          const oadMap = {};
          oad.forEach(d => { oadMap[d.month] = d; });

          // Split each bar: one-and-done (red bottom) + active (blue top)
          // For months without enrichment data, show full bar in blue
          const oneVer = labels.map((m, i) => {
            const d = oadMap[m];
            return d ? d.one_version : 0;
          });
          const active = labels.map((m, i) => {
            const d = oadMap[m];
            return d ? d.multiple : values[i];
          });
          // Unenriched remainder (total bar minus enriched portion)
          const unenriched = labels.map((m, i) => {
            const d = oadMap[m];
            return d ? Math.max(0, values[i] - d.one_version - d.multiple) : 0;
          });

          tooltipFmt = params => {
            const month = params[0].name;
            let total = 0;
            params.forEach(p => { if (p.value != null && p.seriesName !== '3-month trend') total += p.value; });
            let html = month + ' — ' + total.toLocaleString() + ' new groups';
            const d = oadMap[month];
            if (d && d.total > 0) {
              const pct = Math.round(d.one_version / d.total * 100);
              html += ' (' + pct + '% one-and-done)';
            }
            params.forEach(p => {
              if (p.value != null && p.value > 0) {
                html += '<br/>' + p.marker + ' ' + p.seriesName + ': ' + p.value.toLocaleString();
              }
            });
            return html;
          };

          series = [{
            name: 'One-and-done',
            type: 'bar',
            stack: 'groups',
            data: oneVer,
            itemStyle: { color: '#ef4444' },
            emphasis: { focus: 'series' },
            markLine: { symbol: 'none', data: markLines, silent: true },
            barMaxWidth: 30,
          }, {
            name: 'Active (2+ versions)',
            type: 'bar',
            stack: 'groups',
            data: active,
            itemStyle: { color: '#3b82f6' },
            emphasis: { focus: 'series' },
            barMaxWidth: 30,
          }, {
            name: 'Not yet enriched',
            type: 'bar',
            stack: 'groups',
            data: unenriched,
            itemStyle: { color: '#334155' },
            barMaxWidth: 30,
          }, {
            name: '3-month trend',
            type: 'line',
            data: trend,
            smooth: true,
            symbol: 'none',
            lineStyle: { color: '#f59e0b', width: 2 },
            z: 10,
          }];
        } else {
          // No enrichment data yet — plain blue bars
          tooltipFmt = params => {
            let html = params[0].name;
            params.forEach(p => {
              if (p.value != null) html += '<br/>' + p.marker + ' ' + p.seriesName + ': ' + p.value.toLocaleString();
            });
            return html;
          };
          series = [{
            name: 'New groups',
            type: 'bar',
            data: values,
            itemStyle: { color: '#3b82f6' },
            markLine: { symbol: 'none', data: markLines, silent: true },
            barMaxWidth: 30,
          }, {
            name: '3-month trend',
            type: 'line',
            data: trend,
            smooth: true,
            symbol: 'none',
            lineStyle: { color: '#f59e0b', width: 2 },
            z: 10,
          }];
        }

        chart.setOption({
          backgroundColor: 'transparent',
          tooltip: { trigger: 'axis', formatter: tooltipFmt },
          xAxis: {
            type: 'category',
            data: labels,
            axisLabel: { rotate: 45, color: '#64748b', fontSize: 11 },
            axisLine: { lineStyle: { color: '#1e293b' } },
          },
          yAxis: {
            type: 'value',
            axisLabel: { color: '#64748b' },
            splitLine: { lineStyle: { color: '#1e293b' } },
          },
          series: series,
          legend: {
            show: true,
            top: 5,
            right: 20,
            textStyle: { color: '#94a3b8', fontSize: 11 },
          },
          grid: { top: 50, bottom: 60, left: 50, right: 20 },
        }, true); // notMerge=true to fully replace series on filter change
      }).catch(() => {
        // Fall back without one-and-done overlay
        chart.setOption({
          backgroundColor: 'transparent',
          tooltip: { trigger: 'axis' },
          xAxis: { type: 'category', data: labels, axisLabel: { rotate: 45, color: '#64748b', fontSize: 11 }, axisLine: { lineStyle: { color: '#1e293b' } } },
          yAxis: { type: 'value', axisLabel: { color: '#64748b' }, splitLine: { lineStyle: { color: '#1e293b' } } },
          series: [{ name: 'New groups', type: 'bar', data: values, itemStyle: { color: '#3b82f6' }, markLine: { symbol: 'none', data: markLines, silent: true }, barMaxWidth: 30 },
                   { name: '3-month trend', type: 'line', data: trend, smooth: true, symbol: 'none', lineStyle: { color: '#f59e0b', width: 2 }, z: 10 }],
          legend: { show: true, top: 5, right: 20, textStyle: { color: '#94a3b8', fontSize: 11 } },
          grid: { top: 50, bottom: 60, left: 50, right: 20 },
        }, true);
      });

      setTimeout(update, 60000);
    })
    .catch(() => {
      setTimeout(update, 10000);
    });
}

update();
window.addEventListener('resize', () => chart.resize());
</script>
</body>
</html>`

func NewChart2(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(newChartHTML2))
}
