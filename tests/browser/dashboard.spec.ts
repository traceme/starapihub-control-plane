import { test, expect } from '@playwright/test';
import { injectDashboardToken, collectConsoleErrors, assertNoErrorBoundary } from './auth.setup';

const DASHBOARD_PAGES = [
  { path: '/', name: 'Home', contentText: 'Command Center' },
  { path: '/cookies', name: 'Cookies', contentPattern: /HEALTHY|WARNING|CRITICAL|No cookie data|Cookie/ },
  { path: '/models', name: 'Models', contentPattern: /Models|No models|Failed to load|Model/ },
  { path: '/logs', name: 'Logs', contentPattern: /[Ll]ogs|No entries|Failed to load/ },
  { path: '/ops', name: 'Ops', contentPattern: /Sync|Diff|Bootstrap|Audit/ },
  { path: '/setup', name: 'Setup', contentPattern: /Setup|wizard|Provider/i },
] as const;

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
    await expect(page.getByText(/HEALTHY|WARNING|CRITICAL|No cookie data|Cookie/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Models page renders model editor', async ({ page }) => {
    await page.goto('/models');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByText(/Models|No models|Failed to load|Model/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Logs page renders log viewer', async ({ page }) => {
    await page.goto('/logs');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByText(/[Ll]ogs|No entries|Failed to load/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Ops page renders ops panel', async ({ page }) => {
    await page.goto('/ops');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByText(/Sync|Diff|Bootstrap|Audit/)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });

  test('[Dashboard] Setup page renders setup wizard', async ({ page }) => {
    await page.goto('/setup');
    await page.waitForLoadState('domcontentloaded');
    await assertNoErrorBoundary(page);
    await expect(page.getByText(/Setup|wizard|Provider/i)).toBeVisible({ timeout: 10000 });
    expect(consoleErrors).toHaveLength(0);
  });
});
