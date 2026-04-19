import { test, expect } from '@playwright/test';

test.describe('New Groups Per Month chart', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/new-groups-per-month');
    await page.waitForFunction(() => {
      const text = document.getElementById('status')?.textContent ?? '';
      return text.includes('new groups') && !text.includes('Loading');
    }, { timeout: 15000 });
    await page.waitForTimeout(500);
  });

  test('chart renders with data', async ({ page }) => {
    await expect(page.locator('#chart')).toBeVisible();
    await expect(page.locator('#status')).toContainText('new groups');
  });

  test('clicking a bar opens hierarchical drill-down', async ({ page }) => {
    const barInfo = await page.evaluate(() => {
      const chartDom = document.getElementById('chart');
      const instance = (window as any).echarts.getInstanceByDom(chartDom);
      const seriesModel = instance.getModel().getSeriesByIndex(0);
      const data = seriesModel.getData();
      const bars: { label: string; cssX: number; cssY: number }[] = [];
      for (let i = 0; i < data.count(); i++) {
        const layout = data.getItemLayout(i);
        if (!layout) continue;
        bars.push({ label: data.getName(i), cssX: layout.x + layout.width / 2, cssY: layout.y + layout.height / 2 });
      }
      return bars;
    });

    expect(barInfo.length).toBeGreaterThan(0);
    const box = await page.locator('#chart').boundingBox();
    if (!box) return;

    // Click a recent bar
    const bar = barInfo[barInfo.length - 2];
    await page.mouse.click(box.x + bar.cssX, box.y + bar.cssY);
    await page.waitForSelector('#groups-panel[style*="display: block"]', { timeout: 5000 });

    // Should show hierarchical view with prefixes
    const title = await page.locator('#groups-title').textContent();
    expect(title).toContain(bar.label);

    // Should have group items with depth indicators
    const items = await page.locator('.group-item').count();
    expect(items).toBeGreaterThan(0);
  });

  test('filter toggle changes chart data', async ({ page }) => {
    // Get initial Y-axis max
    const allMax = await page.evaluate(() => {
      const inst = (window as any).echarts.getInstanceByDom(document.getElementById('chart'));
      return inst.getModel().getComponent('yAxis', 0).axis.scale.getExtent()[1];
    });

    // Click "Truly new namespaces"
    await page.click('#btn-new');
    await page.waitForTimeout(3000);

    const newMax = await page.evaluate(() => {
      const inst = (window as any).echarts.getInstanceByDom(document.getElementById('chart'));
      return inst.getModel().getComponent('yAxis', 0).axis.scale.getExtent()[1];
    });

    // Truly new should have a smaller Y-axis
    expect(newMax).toBeLessThan(allMax);

    // Click "Extensions of existing"
    await page.click('#btn-ext');
    await page.waitForTimeout(3000);

    const extMax = await page.evaluate(() => {
      const inst = (window as any).echarts.getInstanceByDom(document.getElementById('chart'));
      return inst.getModel().getComponent('yAxis', 0).axis.scale.getExtent()[1];
    });

    // Extensions should be between new and all
    expect(extMax).toBeLessThan(allMax);
    expect(extMax).toBeGreaterThan(newMax);
  });
});

test.describe('All chart pages load', () => {
  const charts = [
    { path: '/new-groups-per-month', title: 'New Maven Central Groups' },
    { path: '/publishes-per-month', title: 'New Groups Per Month By Prefix' },
    { path: '/license-trends', title: 'License Distribution' },
    { path: '/artifact-trends', title: 'New Artifacts Per Month' },
    { path: '/cve-trends', title: 'Security' },
    { path: '/source-repos', title: 'Source Repository' },
    { path: '/size-distribution', title: 'Group Size' },
  ];

  for (const { path, title } of charts) {
    test(`${path} renders`, async ({ page }) => {
      await page.goto(path);
      await expect(page.locator('#chart')).toBeVisible();
      await expect(page.locator('h1')).toContainText(title);

      // No JS errors
      const errors: string[] = [];
      page.on('pageerror', (err) => errors.push(err.message));
      await page.waitForTimeout(2000);
      expect(errors).toHaveLength(0);
    });
  }
});

test.describe('Index page', () => {
  test('all chart links present', async ({ page }) => {
    await page.goto('/');
    const links = await page.locator('a.card').count();
    expect(links).toBeGreaterThanOrEqual(10);
  });
});
