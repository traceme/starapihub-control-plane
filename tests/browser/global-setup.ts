import type { FullConfig } from '@playwright/test';

async function globalSetup(config: FullConfig) {
  // GATEWAY_URL routes through nginx so the request generates an nginx access
  // log entry for CI-07. Falls back to NEWAPI_URL for backward compatibility,
  // but CI-07 will only pass when the request hits the nginx ingress.
  const gatewayUrl = process.env.GATEWAY_URL || process.env.NEWAPI_URL || 'http://localhost:3000';
  const apiKey = process.env.API_KEY;

  if (!apiKey) {
    throw new Error(
      'CI-05: API_KEY env var is required for smoke inference. ' +
      'Set API_KEY to a valid New-API token before running tests.'
    );
  }

  const resp = await fetch(`${gatewayUrl}/v1/chat/completions`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: process.env.SMOKE_MODEL || 'cheap-chat',
      messages: [{ role: 'user', content: 'ping' }],
      max_tokens: 5,
    }),
  });

  if (!resp.ok && resp.status !== 402 && resp.status !== 429) {
    throw new Error(
      `CI-05: Smoke inference failed with status ${resp.status}. ` +
      `URL: ${gatewayUrl}/v1/chat/completions\n` +
      `Response: ${await resp.text()}`
    );
  }

  // Allow brief propagation time for nginx log to be written and polled
  await new Promise(r => setTimeout(r, 2000));
}

export default globalSetup;
