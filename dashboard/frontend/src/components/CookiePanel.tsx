import type { CookieMap } from '../types';
import styles from './CookiePanel.module.css';

interface Props {
  cookies: CookieMap;
}

function severity(stats: { valid: number; total: number; high_utilization: number }): 'HEALTHY' | 'WARNING' | 'CRITICAL' {
  if (stats.valid === 0) return 'CRITICAL';
  if (stats.high_utilization > 0 || stats.valid / Math.max(1, stats.total) < 0.5) return 'WARNING';
  return 'HEALTHY';
}

function badgeClass(sev: string) {
  if (sev === 'CRITICAL') return styles.badgeCritical;
  if (sev === 'WARNING') return styles.badgeWarning;
  return styles.badgeHealthy;
}

// Best-guess admin URL from instance name
function adminUrl(instance: string): string {
  const match = instance.match(/clewdr[-_]?(\d+)/i);
  const port = match ? 8483 + parseInt(match[1], 10) : 8484;
  return `http://127.0.0.1:${port}`;
}

export default function CookiePanel({ cookies }: Props) {
  const entries = Object.entries(cookies);

  return (
    <div className={styles.page}>
      <h2 className={styles.title}>Cookie Health</h2>
      {entries.length === 0 && (
        <div className={styles.empty}>No ClewdR instances reporting yet. Data appears once the SSE stream sends cookie stats.</div>
      )}
      <div className={styles.grid}>
        {entries.map(([name, stats]) => {
          const sev = severity(stats);
          const total = Math.max(1, stats.total);
          return (
            <div key={name} className={styles.card}>
              <div className={styles.cardHeader}>
                <span className={styles.instanceName}>{name}</span>
                <span className={`${styles.badge} ${badgeClass(sev)}`}>{sev}</span>
              </div>

              <div className={styles.counts}>
                <div className={styles.countItem}>
                  <span className={styles.countNum} style={{ color: 'var(--green)' }}>{stats.valid}</span>
                  <span className={styles.countLabel}>Valid</span>
                </div>
                <div className={styles.countItem}>
                  <span className={styles.countNum} style={{ color: 'var(--amber)' }}>{stats.exhausted}</span>
                  <span className={styles.countLabel}>Exhausted</span>
                </div>
                <div className={styles.countItem}>
                  <span className={styles.countNum} style={{ color: 'var(--red)' }}>{stats.invalid}</span>
                  <span className={styles.countLabel}>Invalid</span>
                </div>
              </div>

              {/* Stacked bar */}
              <div className={styles.stackedBar}>
                <div
                  className={styles.stackSegment}
                  style={{ width: `${(stats.valid / total) * 100}%`, background: 'var(--green)' }}
                />
                <div
                  className={styles.stackSegment}
                  style={{ width: `${(stats.exhausted / total) * 100}%`, background: 'var(--amber)' }}
                />
                <div
                  className={styles.stackSegment}
                  style={{ width: `${(stats.invalid / total) * 100}%`, background: 'var(--red)' }}
                />
              </div>

              {/* Utilization */}
              <div className={styles.utilSection}>
                <div className={styles.utilRow}>
                  <span className={styles.utilLabel}>Cookie Utilization</span>
                  <span className={styles.utilPct}>{((stats.valid / total) * 100).toFixed(0)}%</span>
                </div>
                <div className={styles.progressTrack}>
                  <div
                    className={styles.progressFill}
                    style={{
                      width: `${(stats.valid / total) * 100}%`,
                      background: sev === 'CRITICAL' ? 'var(--red)' : sev === 'WARNING' ? 'var(--amber)' : 'var(--green)',
                    }}
                  />
                </div>
              </div>

              {stats.high_utilization > 0 && (
                <div className={styles.utilNote}>
                  {stats.high_utilization} cookie{stats.high_utilization > 1 ? 's' : ''} with high utilization
                </div>
              )}

              <div className={styles.cardFooter}>
                <a
                  href={adminUrl(name)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={styles.adminLink}
                >
                  Open Admin UI
                </a>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
