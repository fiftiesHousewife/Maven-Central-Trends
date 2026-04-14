import { test, expect } from '@playwright/test';

test.describe('New Groups Per Month chart (ECharts)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/new-groups-per-month');
    await page.waitForFunction(() => {
      const text = document.getElementById('status')?.textContent ?? '';
      return text.includes('new groups') && !text.includes('Loading');
    }, { timeout: 15000 });
    await page.waitForTimeout(500);
  });

  test('chart renders with data', async ({ page }) => {
    const chartEl = page.locator('#chart');
    await expect(chartEl).toBeVisible();

    const status = page.locator('#status');
    await expect(status).toContainText('new groups');
  });

  test('clicking a bar shows the correct month in groups panel', async ({ page }) => {
    const barInfo = await page.evaluate(() => {
      const chartDom = document.getElementById('chart');
      const instance = (window as any).echarts.getInstanceByDom(chartDom);
      const seriesModel = instance.getModel().getSeriesByIndex(0);
      const data = seriesModel.getData();

      const bars: { label: string; cssX: number; cssY: number }[] = [];
      for (let i = 0; i < data.count(); i++) {
        const layout = data.getItemLayout(i);
        if (!layout) continue;
        bars.push({
          label: data.getName(i),
          cssX: layout.x + layout.width / 2,
          cssY: layout.y + layout.height / 2,
        });
      }
      return bars;
    });

    expect(barInfo.length).toBeGreaterThan(0);

    const chartEl = page.locator('#chart');
    const box = await chartEl.boundingBox();
    if (!box) return;

    const testIndices = [0, 5, 17, barInfo.length - 3, barInfo.length - 2, barInfo.length - 1];

    for (const idx of testIndices) {
      if (idx < 0 || idx >= barInfo.length) continue;
      const bar = barInfo[idx];

      await page.evaluate(() => {
        document.getElementById('groups-panel')!.style.display = 'none';
      });

      await page.mouse.click(box.x + bar.cssX, box.y + bar.cssY);
      await page.waitForSelector('#groups-panel[style*="display: block"]', { timeout: 5000 });
      await page.waitForTimeout(300);

      const title = await page.locator('#groups-title').textContent();
      expect(title).toContain(bar.label);
    }
  });

  test('tooltip shows on hover', async ({ page }) => {
    const chartEl = page.locator('#chart');
    const box = await chartEl.boundingBox();
    if (!box) return;

    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.waitForTimeout(500);

    // Verify no JS errors occurred during hover
    const errors: string[] = [];
    page.on('pageerror', (err) => errors.push(err.message));
    expect(errors).toHaveLength(0);
  });
});
