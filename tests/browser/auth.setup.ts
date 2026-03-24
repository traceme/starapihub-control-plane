import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';

/**
 * Inject dashboard auth token into sessionStorage before page load.
 * Dashboard checks sessionStorage('starapihub_token') on mount.
 */
export function injectDashboardToken(page: Page): void {
  const token = process.env.DASHBOARD_TOKEN || 'test-token';
  page.addInitScript((t: string) => {
    window.sessionStorage.setItem('starapihub_token', t);
  }, token);
}

/**
 * Inject New-API admin user object into localStorage before page load.
 * New-API checks localStorage('user') for auth; AdminRoute requires role >= 10.
 * The user object MUST include a valid 'token' field — without it, API calls
 * return 401 and the app redirects to /login?expired=true.
 */
export function injectNewApiToken(page: Page): void {
  const token = process.env.ADMIN_TOKEN || '';
  const userJson = process.env.ADMIN_USER_JSON || JSON.stringify({
    username: 'admin',
    role: 100,
    status: 1,
    id: 1,
    token: token,
  });
  page.addInitScript((u: string) => {
    window.localStorage.setItem('user', u);
  }, userJson);
}

/**
 * Collect console.error messages from the page, filtering out known SSE
 * reconnection errors that are expected during normal dashboard operation.
 * Returns a mutable array reference -- check length after page interactions.
 */
export function collectConsoleErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      const text = msg.text();
      if (!text.includes('SSE error:')) {
        errors.push(text);
      }
    }
  });
  return errors;
}

/**
 * Assert that the ErrorBoundary fallback is NOT visible on the page.
 * ErrorBoundary renders: <h2>Something went wrong</h2> + <button>Try Again</button>
 */
export async function assertNoErrorBoundary(page: Page): Promise<void> {
  await expect(page.getByText('Something went wrong')).not.toBeVisible({ timeout: 2000 });
  await expect(page.getByRole('button', { name: 'Try Again' })).not.toBeVisible({ timeout: 2000 });
}
