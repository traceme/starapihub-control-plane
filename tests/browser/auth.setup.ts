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
 * Programmatic login to New-API: POST /api/user/login, capture session cookie,
 * inject into browser context + localStorage user object.
 * Returns true if login succeeded, false if credentials missing or login failed.
 */
export async function loginNewApi(page: Page): Promise<boolean> {
  const newApiUrl = process.env.NEWAPI_URL || 'http://localhost:3000';
  const username = process.env.ADMIN_USERNAME;
  const password = process.env.ADMIN_PASSWORD;

  if (!username || !password) {
    return false;
  }

  const resp = await fetch(`${newApiUrl}/api/user/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });

  if (!resp.ok) return false;

  const body = await resp.json();
  if (!body.success) return false;

  // Check for 2FA requirement
  if (body.data?.require_2fa) {
    throw new Error(
      'CI-08: New-API admin account has 2FA enabled. ' +
      'Disable 2FA for the CI admin user to use programmatic login.'
    );
  }

  // Parse Set-Cookie header and inject into browser context
  const setCookieHeader = resp.headers.get('set-cookie') || '';
  if (setCookieHeader) {
    const url = new URL(newApiUrl);
    const cookies: Array<{name: string; value: string; domain: string; path: string}> = [];
    // Set-Cookie may have multiple values separated by comma-space for date parts,
    // so split carefully on comma followed by space and a cookie name pattern
    for (const part of setCookieHeader.split(/,\s*(?=[A-Za-z_]+=)/)) {
      const [nameValue] = part.split(';');
      if (nameValue && nameValue.includes('=')) {
        const eqIdx = nameValue.indexOf('=');
        cookies.push({
          name: nameValue.substring(0, eqIdx).trim(),
          value: nameValue.substring(eqIdx + 1).trim(),
          domain: url.hostname,
          path: '/',
        });
      }
    }
    if (cookies.length > 0) {
      await page.context().addCookies(cookies);
    }
  }

  // Inject user data into localStorage BEFORE page loads
  // New-API frontend reads localStorage('user') for auth state and New-API-User header
  const userData = body.data;
  await page.addInitScript((u: string) => {
    window.localStorage.setItem('user', u);
  }, JSON.stringify(userData));

  return true;
}

/**
 * Collect console.error messages from the page, filtering out known
 * expected errors. Returns a mutable array reference.
 *
 * Filtered patterns:
 * - 'SSE error:' — dashboard SSE reconnection (normal behavior)
 * - '401' / 'Unauthorized' — New-API API calls when using synthetic
 *   auth token (pages render fine, data calls fail without real session).
 *   When using real auth via loginNewApi(), set filterAuth=false (the default)
 *   so 401 errors are caught as real failures.
 * - 'Failed to load resource' — network errors already covered by status checks
 */
export function collectConsoleErrors(page: Page, filterAuth = false): string[] {
  const errors: string[] = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      const text = msg.text();
      if (text.includes('SSE error:')) return;
      if (filterAuth && (
        text.includes('401') ||
        text.includes('Unauthorized') ||
        text.includes('Failed to load resource')
      )) return;
      errors.push(text);
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
