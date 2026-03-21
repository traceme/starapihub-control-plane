import type { SSEState } from '../types';
import styles from './HealthDashboard.module.css';

interface Props {
  state: SSEState;
}

const SERVICE_LABELS: Record<string, string> = {
  new_api: 'New-API',
  bifrost: 'Bifrost',
  postgres: 'Postgres',
  redis: 'Redis',
};

function statusColor(status: string): string {
  if (status === 'healthy') return 'var(--green)';
  if (status === 'stale') return 'var(--amber)';
  return 'var(--red)';
}

function statusBarColor(code: string): string {
  if (code.startsWith('2')) return 'var(--green)';
  if (code.startsWith('4')) return 'var(--amber)';
  return 'var(--red)';
}

function formatRate(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return n.toFixed(1);
}

export default function HealthDashboard({ state }: Props) {
  const { health, log_stats } = state;
  const services = Object.entries(health);

  // Compute model chart max for scaling
  const modelEntries = Object.entries(log_stats.by_model || {});
  const maxModel = Math.max(1, ...modelEntries.map(([, v]) => v));

  // Status distribution
  const statusEntries = Object.entries(log_stats.by_status || {});
  const statusTotal = Math.max(1, statusEntries.reduce((s, [, v]) => s + v, 0));

  return (
    <div className={styles.page}>
      {/* Service Grid */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Service Status</h2>
        <div className={styles.serviceGrid}>
          {services.length === 0 && (
            <div className={styles.emptyCard}>
              <p className={styles.emptyTitle}>Connecting to services...</p>
              <p className={styles.emptyHint}>Health data will appear here once the dashboard connects to your stack via SSE.</p>
            </div>
          )}
          {services.map(([name, svc]) => {
            const label = SERVICE_LABELS[name] || name.replace(/_/g, ' ');
            return (
              <div
                key={name}
                className={styles.serviceCard}
                style={{ borderLeftColor: statusColor(svc.status) }}
                role="status"
                aria-label={`${label}: ${svc.status}, latency ${svc.latency_ms}ms`}
              >
                <div className={styles.serviceHeader}>
                  <span
                    className={styles.dot}
                    style={{ background: statusColor(svc.status) }}
                    aria-hidden="true"
                  />
                  <span className={styles.serviceName}>{label}</span>
                </div>
                <div className={styles.serviceBody}>
                  <span className={styles.statusLabel}>{svc.status.toUpperCase()}</span>
                  <span className={styles.latency}>{svc.latency_ms}ms</span>
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {/* Traffic Stats */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Traffic</h2>
        <div className={styles.statsGrid}>
          <div className={styles.statCard}>
            <div className={styles.statValue}>{formatRate(log_stats.request_rate)}</div>
            <div className={styles.statLabel}>req/s</div>
          </div>
          <div className={styles.statCard}>
            <div className={styles.statValue}>{log_stats.p50_latency_ms}</div>
            <div className={styles.statLabel}>p50 (ms)</div>
          </div>
          <div className={styles.statCard}>
            <div className={styles.statValue}>{log_stats.p99_latency_ms}</div>
            <div className={styles.statLabel}>p99 (ms)</div>
          </div>
          <div className={styles.statCard}>
            <div
              className={styles.statValue}
              style={{ color: log_stats.error_rate > 0.05 ? 'var(--red)' : undefined }}
            >
              {(log_stats.error_rate * 100).toFixed(2)}%
            </div>
            <div className={styles.statLabel}>Error Rate</div>
          </div>
        </div>
      </section>

      <div className={styles.twoCol}>
        {/* Model Usage Chart */}
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Model Usage (24h)</h2>
          <div className={styles.chartBox}>
            {modelEntries.length === 0 && (
              <p className={styles.emptyHint}>No model traffic recorded yet. Send a request through the gateway to see usage stats.</p>
            )}
            {modelEntries
              .sort((a, b) => b[1] - a[1])
              .map(([model, count]) => (
                <div key={model} className={styles.barRow}>
                  <span className={styles.barLabel} title={model}>
                    {model}
                  </span>
                  <div className={styles.barTrack}>
                    <div
                      className={styles.barFill}
                      style={{ width: `${(count / maxModel) * 100}%` }}
                    />
                  </div>
                  <span className={styles.barValue}>{count.toLocaleString()}</span>
                </div>
              ))}
          </div>
        </section>

        {/* Status Code Distribution */}
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Status Codes</h2>
          <div className={styles.chartBox}>
            {statusEntries.length === 0 && (
              <p className={styles.emptyHint}>Status code breakdown appears after traffic flows through the gateway.</p>
            )}
            {statusEntries
              .sort((a, b) => b[1] - a[1])
              .map(([code, count]) => (
                <div key={code} className={styles.barRow}>
                  <span className={styles.barLabel}>{code}</span>
                  <div className={styles.barTrack}>
                    <div
                      className={styles.barFill}
                      style={{
                        width: `${(count / statusTotal) * 100}%`,
                        background: statusBarColor(code),
                      }}
                    />
                  </div>
                  <span className={styles.barValue}>
                    {((count / statusTotal) * 100).toFixed(1)}%
                  </span>
                </div>
              ))}
          </div>
        </section>
      </div>
    </div>
  );
}
