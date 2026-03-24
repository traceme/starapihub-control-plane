import { test, expect } from '@playwright/test';
import { injectNewApiToken, collectConsoleErrors } from './auth.setup';

test.describe('New-API Admin Pages', () => {
  let consoleErrors: string[];

  test.beforeEach(async ({ page }) => {
    injectNewApiToken(page);
    consoleErrors = collectConsoleErrors(page);
  });

  test('[New-API] Channels page renders', async ({ page }) => {
    await page.goto('/console/channel');
    await page.waitForLoadState('domcontentloaded');
    // Verify we're not on the login page
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    // Check for any page content (heading, table, or button)
    await expect(page.locator('.semi-table, h2, [role="table"], .semi-card').first()).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[New-API] Tokens page renders', async ({ page }) => {
    await page.goto('/console/token');
    await page.waitForLoadState('domcontentloaded');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    await expect(page.locator('.semi-table, h2, [role="table"], .semi-card').first()).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[New-API] Logs page renders', async ({ page }) => {
    await page.goto('/console/log');
    await page.waitForLoadState('domcontentloaded');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    await expect(page.locator('.semi-table, h2, [role="table"], .semi-card').first()).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });
});
