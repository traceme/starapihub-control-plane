export interface ServiceHealth {
  status: 'healthy' | 'unhealthy' | 'stale';
  latency_ms: number;
}

export interface HealthMap {
  [service: string]: ServiceHealth;
}

export interface CookieStats {
  valid: number;
  exhausted: number;
  invalid: number;
  total: number;
  high_utilization: number;
}

export interface CookieMap {
  [instance: string]: CookieStats;
}

export interface ModelStats {
  [model: string]: number;
}

export interface StatusStats {
  [code: string]: number;
}

export interface LogStats {
  request_rate: number;
  p50_latency_ms: number;
  p99_latency_ms: number;
  error_rate: number;
  by_model: ModelStats;
  by_status: StatusStats;
}

export interface Alert {
  id: number;
  type: string;
  severity: 'CRITICAL' | 'WARNING' | 'INFO';
  signal: string;
  service: string;
  message: string;
  timestamp: string;
  acknowledged?: boolean;
}

export interface SSEState {
  health: HealthMap;
  cookies: CookieMap;
  log_stats: LogStats;
  alerts: Alert[];
}

export interface LogEntry {
  timestamp: string;
  method: string;
  path: string;
  status: number;
  latency_ms: number;
  request_id: string;
  model: string;
  trace?: string;
}

export interface LogicalModel {
  id: string;
  name: string;
  display_name: string;
  billing_name: string;
  description: string;
  upstream_model: string;
  risk_level: 'low' | 'medium' | 'high';
  allowed_groups: string[];
  channel: string;
  route_policy: string;
  unofficial_allowed: boolean;
  caching_allowed: boolean;
  providers: ProviderEntry[];
  live_providers: ProviderEntry[];
  policy_warning?: string;
}

export interface ModelDetail {
  name: string;
  display_name: string;
  billing_name: string;
  description: string;
  upstream_model: string;
  risk_level: 'low' | 'medium' | 'high';
  allowed_groups: string[];
  channel: string;
  route_policy: string;
  unofficial_allowed: boolean;
  caching_allowed: boolean;
  model_ratio: number | null;
  completion_ratio: number | null;
  cache_ratio: number | null;
  model_price: number | null;
}

export interface ModelFormData {
  name: string;
  display_name: string;
  billing_name: string;
  description: string;
  upstream_model: string;
  risk_level: 'low' | 'medium' | 'high';
  allowed_groups: string[];
  channel: string;
  route_policy: string;
  unofficial_allowed: boolean;
  caching_allowed: boolean;
  model_ratio: number | null;
  completion_ratio: number | null;
  cache_ratio: number | null;
  model_price: number | null;
}

export interface ModelsMetadata {
  channels: { id: string; name: string }[];
  route_policies: { id: string; description: string }[];
  risk_levels: string[];
}

export interface ProviderEntry {
  provider_id: string;
  upstream_model: string;
  weight: number;
  priority: number;
}

export interface WizardStatus {
  completed: boolean;
  current_step: number;
}
