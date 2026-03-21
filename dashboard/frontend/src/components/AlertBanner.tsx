import type { Alert } from '../types';
import { acknowledgeAlert } from '../api';
import styles from './AlertBanner.module.css';

interface Props {
  alerts: Alert[];
}

export default function AlertBanner({ alerts }: Props) {
  const active = alerts.filter((a) => !a.acknowledged);
  if (active.length === 0) return null;

  const hasCritical = active.some((a) => a.type === 'critical');

  return (
    <div className={`${styles.banner} ${hasCritical ? styles.critical : styles.warning}`}>
      <div className={styles.inner}>
        <span className={styles.icon}>{hasCritical ? '!' : '\u26A0'}</span>
        <div className={styles.messages}>
          {active.map((alert) => (
            <div key={alert.id} className={styles.alertRow}>
              <span className={styles.badge}>{alert.type.toUpperCase()}</span>
              <span className={styles.service}>[{alert.service}]</span>
              <span className={styles.msg}>{alert.message}</span>
              <button
                className={styles.ackBtn}
                onClick={() => acknowledgeAlert(alert.id)}
              >
                Acknowledge
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
