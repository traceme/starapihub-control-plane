import { defineConfig } from '@playwright/test';

export default defineConfig({
  globalSetup: require.resolve('./global-setup'),
  testDir: '.',
  timeout: 30_000,
  retries: 0,
  use: {
    trace: 'retain-on-failure',
  },
  reporter: [
    ['list'],
    ['junit', { outputFile: '../../artifacts/browser-results.xml' }],
    ['json', { outputFile: '../../artifacts/browser-results.json' }],
  ],
  projects: [
    {
      name: 'dashboard',
      testMatch: /dashboard\.spec\.ts/,
      use: {
        baseURL: process.env.DASHBOARD_URL || 'http://localhost:8090',
      },
    },
    {
      name: 'newapi-admin',
      testMatch: /newapi-admin\.spec\.ts/,
      use: {
        baseURL: process.env.NEWAPI_URL || 'http://localhost:3000',
      },
    },
  ],
});
