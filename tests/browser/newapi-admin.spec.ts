import { test, expect } from '@playwright/test';
import { injectNewApiToken, collectConsoleErrors } from './auth.setup';

const NEWAPI_PAGES = [
  { path: '/console/channel', name: 'Channels', contentPattern: /[Cc]hannel|[Pp]rovider/ },
  { path: '/console/token', name: 'Tokens', contentPattern: /[Tt]oken|API/ },
  { path: '/console/log', name: 'Logs', contentPattern: /[Ll]og/ },
] as const;

test.describe('New-API Admin Pages', () => {
  let consoleErrors: string[];

  test.beforeEach(async ({ page }) => {
    injectNewApiToken(page);
    consoleErrors = collectConsoleErrors(page);
  });

  test('[New-API] Channels page renders', async ({ page }) => {
    await page.goto('/console/channel');
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByText(/[Cc]hannel|[Pp]rovider/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[New-API] Tokens page renders', async ({ page }) => {
    await page.goto('/console/token');
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByText(/[Tt]oken|API/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[New-API] Logs page renders', async ({ page }) => {
    await page.goto('/console/log');
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByText(/[Ll]og/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });
});
