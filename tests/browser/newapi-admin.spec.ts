import { test, expect } from '@playwright/test';
import { injectNewApiToken, collectConsoleErrors } from './auth.setup';

const NEWAPI_URL = process.env.NEWAPI_URL || 'http://localhost:3000';

/**
 * Check if New-API is reachable and not rate-limited before running tests.
 * New-API sometimes returns 429 on all requests (including HTML pages)
 * when rate limits are hit or the server is in a bad state.
 */
async function isNewApiAvailable(): Promise<boolean> {
  try {
    const resp = await fetch(`${NEWAPI_URL}/api/status`, { signal: AbortSignal.timeout(5000) });
    return resp.status !== 429;
  } catch {
    // Also try the root — /api/status may not exist
    try {
      const resp = await fetch(NEWAPI_URL, { signal: AbortSignal.timeout(5000) });
      return resp.status !== 429;
    } catch {
      return false;
    }
  }
}

test.describe('New-API Admin Pages', () => {
  let consoleErrors: string[];

  test.beforeAll(async () => {
    const available = await isNewApiAvailable();
    if (!available) {
      test.skip(true, `New-API at ${NEWAPI_URL} is unavailable or rate-limited (429)`);
    }
  });

  test.beforeEach(async ({ page }) => {
    injectNewApiToken(page);
    // filterAuth=true: New-API API calls return 401 with synthetic token,
    // but the SPA pages render correctly. Only catch unexpected errors.
    consoleErrors = collectConsoleErrors(page, true);
  });

  test('[New-API] Channels page renders', async ({ page }) => {
    const resp = await page.goto(`${NEWAPI_URL}/console/channel`);
    if (resp && resp.status() === 429) {
      test.skip(true, 'New-API returning 429 — rate limited');
      return;
    }
    await page.waitForLoadState('domcontentloaded');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    // Wait for React SPA to hydrate and render content
    await expect(page.locator('body').first()).not.toHaveText('', { timeout: 15000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[New-API] Tokens page renders', async ({ page }) => {
    const resp = await page.goto(`${NEWAPI_URL}/console/token`);
    if (resp && resp.status() === 429) {
      test.skip(true, 'New-API returning 429 — rate limited');
      return;
    }
    await page.waitForLoadState('domcontentloaded');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    await expect(page.locator('body').first()).not.toHaveText('', { timeout: 15000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[New-API] Logs page renders', async ({ page }) => {
    const resp = await page.goto(`${NEWAPI_URL}/console/log`);
    if (resp && resp.status() === 429) {
      test.skip(true, 'New-API returning 429 — rate limited');
      return;
    }
    await page.waitForLoadState('domcontentloaded');
    await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    await expect(page.locator('body').first()).not.toHaveText('', { timeout: 15000 });
    expect(consoleErrors).toHaveLength(0);
  });
});
