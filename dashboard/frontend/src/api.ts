import type { LogEntry, LogicalModel, WizardStatus } from './types';

function getToken(): string {
  return localStorage.getItem('starapihub_token') || '';
}

function headers(): HeadersInit {
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${getToken()}`,
  };
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { ...headers(), ...init?.headers },
  });
  if (res.status === 401) {
    localStorage.removeItem('starapihub_token');
    window.location.reload();
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json();
}

// Logs
export async function fetchLogs(params: {
  status?: string;
  model?: string;
  limit?: number;
}): Promise<LogEntry[]> {
  const q = new URLSearchParams();
  if (params.status) q.set('status', params.status);
  if (params.model) q.set('model', params.model);
  if (params.limit) q.set('limit', String(params.limit));
  return request<LogEntry[]>(`/api/logs?${q.toString()}`);
}

// Models
export async function fetchModels(): Promise<LogicalModel[]> {
  return request<LogicalModel[]>('/api/models');
}

export async function createModel(model: Omit<LogicalModel, 'id'>): Promise<LogicalModel> {
  return request<LogicalModel>('/api/models', {
    method: 'POST',
    body: JSON.stringify(model),
  });
}

export async function updateModel(id: string, model: Partial<LogicalModel>): Promise<LogicalModel> {
  return request<LogicalModel>(`/api/models/${id}`, {
    method: 'PUT',
    body: JSON.stringify(model),
  });
}

export async function deleteModel(id: string): Promise<void> {
  await fetch(`/api/models/${id}`, { method: 'DELETE', headers: headers() });
}

// Alerts
export async function acknowledgeAlert(id: number): Promise<void> {
  await fetch(`/api/alerts/${id}/ack`, { method: 'POST', headers: headers() });
}

// Wizard
export async function getWizardStatus(): Promise<WizardStatus> {
  return request<WizardStatus>('/api/wizard/status');
}

export async function wizardProvider(data: { name: string; api_key: string }): Promise<void> {
  await request<unknown>('/api/wizard/provider', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function wizardModel(data: { name: string; upstream_model: string }): Promise<void> {
  await request<unknown>('/api/wizard/model', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function wizardTest(): Promise<{ success: boolean; response: string; api_key?: string }> {
  return request('/api/wizard/test', { method: 'POST' });
}

// SSE connection via fetch streaming (EventSource doesn't support auth headers)
export function connectSSE(
  token: string,
  onMessage: (data: unknown) => void,
  onError: (err: unknown) => void,
  onOpen?: () => void,
): () => void {
  let aborted = false;
  const controller = new AbortController();

  async function connect() {
    try {
      const res = await fetch(`/api/sse?token=${encodeURIComponent(token)}`, {
        headers: { Authorization: `Bearer ${token}` },
        signal: controller.signal,
      });
      if (!res.ok) {
        onError(new Error(`SSE ${res.status}`));
        return;
      }
      if (!res.body) {
        onError(new Error('No response body'));
        return;
      }
      onOpen?.();
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';

      while (!aborted) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop() || '';
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const parsed = JSON.parse(line.slice(6));
              onMessage(parsed);
            } catch {
              // ignore malformed JSON
            }
          }
        }
      }
    } catch (err) {
      if (!aborted) {
        onError(err);
        // Reconnect after 3s
        setTimeout(() => {
          if (!aborted) connect();
        }, 3000);
      }
    }
  }

  connect();

  return () => {
    aborted = true;
    controller.abort();
  };
}
