import { test, expect } from '@playwright/test';
import { injectDashboardToken, collectConsoleErrors, assertNoErrorBoundary } from './auth.setup';

test.describe('Dashboard Pages', () => {
  let consoleErrors: string[];

  test.beforeEach(async ({ page }) => {
    injectDashboardToken(page);
    consoleErrors = collectConsoleErrors(page);
  });

  test('[Dashboard] Home page renders health dashboard', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByText('Command Center')).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Cookies page renders cookie panel', async ({ page }) => {
    await page.goto('/cookies');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    // Target the page heading specifically to avoid matching nav links
    await expect(page.getByRole('heading', { name: /Cookie/ })).toBeVisible({ timeout: 10000 });
    // CI-06: Assert at least one ClewdR instance card is visible (real data, not just heading)
    await expect(
      page.locator('[class*="instanceName"]').first()
    ).toBeVisible({ timeout: 20000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Models page renders model editor', async ({ page }) => {
    await page.goto('/models');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByRole('heading', { name: /Model/ })).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Logs page renders log viewer', async ({ page }) => {
    await page.goto('/logs');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByRole('heading', { name: /Log/ })).toBeVisible({ timeout: 10000 });
    // CI-07: Assert at least one real log entry exists (from smoke inference in global setup)
    const logTable = page.locator('table');
    await expect(logTable).toBeVisible({ timeout: 15000 });
    await expect(logTable.locator('tbody tr').first()).toBeVisible({ timeout: 15000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Ops page renders ops panel', async ({ page }) => {
    await page.goto('/ops');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    // Ops panel content area — look for any content within main
    await expect(page.locator('main').getByRole('button').first()).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Setup page renders setup wizard', async ({ page }) => {
    await page.goto('/setup');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByRole('heading', { name: /Provider|Setup/i })).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });
});
