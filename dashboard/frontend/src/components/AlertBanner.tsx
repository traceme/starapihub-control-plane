import type { Alert } from '../types';
import { acknowledgeAlert } from '../api';
import styles from './AlertBanner.module.css';

interface Props {
  alerts: Alert[];
}

function relativeTime(timestamp: string): string {
  const delta = Math.max(0, Math.round((Date.now() - new Date(timestamp).getTime()) / 1000));
  if (delta < 60) return `${delta}s ago`;
  if (delta < 3600) return `${Math.round(delta / 60)}m ago`;
  return `${Math.round(delta / 3600)}h ago`;
}

export default function AlertBanner({ alerts }: Props) {
  const active = (alerts || []).filter((alert) => !alert.acknowledged);
  if (active.length === 0) return null;

  const hasCritical = active.some((alert) => alert.severity === 'CRITICAL');

  return (
    <section className={`${styles.banner} ${hasCritical ? styles.critical : styles.warning}`}>
      <div className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Incident Rail</span>
          <h2 className={styles.title}>{active.length} open alert{active.length === 1 ? '' : 's'}</h2>
        </div>
        <div className={styles.summary}>
          {hasCritical ? 'Critical operator intervention recommended.' : 'Warnings detected across the fleet.'}
        </div>
      </div>

      <div className={styles.messages}>
        {active.map((alert) => (
          <article key={alert.id} className={styles.alertRow}>
            <div className={styles.alertLead}>
              <span className={styles.badge}>{alert.severity}</span>
              <span className={styles.service}>{alert.service}</span>
              <span className={styles.time}>{relativeTime(alert.timestamp)}</span>
            </div>
            <p className={styles.msg}>{alert.message}</p>
            <button className={styles.ackBtn} onClick={() => acknowledgeAlert(alert.id)}>
              Acknowledge
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}
