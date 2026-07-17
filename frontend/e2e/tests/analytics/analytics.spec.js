import { test, expect } from '../../fixtures/test.js';
import { ROUTES, SLEEPS } from '../../config.js';
import { collectPageErrors } from '../../helpers/diagnostics.js';

test.describe('Analytics Dashboard (US2)', () => {
  test('analytics page is accessible via nav', async ({ page, libraryPage }) => {
    await test.step('Navigate to analytics via the Quality section and verify the URL', async () => {
      await libraryPage.open();
      await libraryPage.nav('Quality');
      await libraryPage.nav('Analytics');
      await expect(page).toHaveURL(ROUTES.ANALYTICS);
    });
  });

  test('analytics dashboard shows summary cards', async ({ page, api }) => {
    await test.step('Seed a run result with a recent start_time so the dashboard has data', async () => {
      // Analytics counts run_results by start_time; an API-seeded result defaults to
      // a zero start_time (filtered out), so set it explicitly. Cards hide on empty.
      const folder = await api.createFolder(`Analytics ${Date.now()}`);
      const tc = await api.createTest(`Analytics TC ${Date.now()}`, folder.id);
      const run = await api.createRun(`Analytics Run ${Date.now()}`);
      await api.addRunResult(run.id, tc.id, { status: 'PASS', start_time: new Date().toISOString() });
    });

    await test.step('Open the analytics page and verify the summary cards are visible', async () => {
      await page.goto(ROUTES.ANALYTICS);
      await expect(page.getByText('Total Runs', { exact: true })).toBeVisible();
      await expect(page.getByText('Pass Rate', { exact: true })).toBeVisible();
    });
  });

  test('analytics page renders without errors on empty data', async ({ page }) => {
    let errors;
    await test.step('Open the analytics page and capture any page errors', async () => {
      await page.goto(ROUTES.ANALYTICS);
      errors = collectPageErrors(page);
      await page.waitForTimeout(SLEEPS.PAGEERROR_OBSERVE);
    });
    await test.step('Verify no page errors were thrown', async () => {
      expect(errors).toHaveLength(0);
    });
  });
});
