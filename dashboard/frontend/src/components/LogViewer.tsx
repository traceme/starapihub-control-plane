import { useState, useEffect, useCallback, useRef } from 'react';
import type { LogEntry } from '../types';
import { fetchLogs } from '../api';
import styles from './LogViewer.module.css';

function statusClass(status: number): string {
  if (status >= 200 && status < 300) return styles.status2xx;
  if (status >= 400 && status < 500) return styles.status4xx;
  return styles.status5xx;
}

export default function LogViewer() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [expanded, setExpanded] = useState<string | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState('');
  const [modelFilter, setModelFilter] = useState('');
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await fetchLogs({
        status: statusFilter || undefined,
        model: modelFilter || undefined,
        limit: 100,
      });
      setLogs(data);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load logs');
    } finally {
      setLoading(false);
    }
  }, [statusFilter, modelFilter]);

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, 5000);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [load]);

  const toggleRow = (id: string) => {
    setExpanded(expanded === id ? null : id);
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h2 className={styles.title}>Gateway Request Logs</h2>
        <div className={styles.filters}>
          <select
            className={styles.filterSelect}
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            <option value="">All Status</option>
            <option value="2xx">2xx</option>
            <option value="4xx">4xx</option>
            <option value="5xx">5xx</option>
          </select>
          <input
            className={styles.filterInput}
            placeholder="Filter by model..."
            value={modelFilter}
            onChange={(e) => setModelFilter(e.target.value)}
          />
          <button className={styles.refreshBtn} onClick={load}>
            Refresh
          </button>
        </div>
      </div>

      <p className={styles.infoLink}>
        Looking for billing &amp; token usage?{' '}
        <a href="http://localhost:3000" target="_blank" rel="noopener noreferrer">
          New-API Admin (port 3000) &rarr;
        </a>
      </p>

      {error && <div className={styles.error}>{error}</div>}

      {loading ? (
        <p className={styles.empty}>Loading logs...</p>
      ) : logs.length === 0 ? (
        <p className={styles.empty}>No gateway request logs found. Logs appear after inference requests pass through the nginx gateway.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Timestamp</th>
                <th>Method</th>
                <th>Path</th>
                <th>Status</th>
                <th>Latency</th>
                <th>Model</th>
                <th>Request ID</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <>
                  <tr
                    key={log.request_id}
                    className={styles.logRow}
                    onClick={() => toggleRow(log.request_id)}
                  >
                    <td className={styles.mono}>
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </td>
                    <td>
                      <span className={styles.method}>{log.method}</span>
                    </td>
                    <td className={styles.mono} title={log.path}>
                      {log.path}
                    </td>
                    <td>
                      <span className={`${styles.statusBadge} ${statusClass(log.status)}`}>
                        {log.status}
                      </span>
                    </td>
                    <td className={styles.mono}>{log.latency_ms}ms</td>
                    <td className={styles.mono}>{log.model || '-'}</td>
                    <td className={styles.mono} title={log.request_id}>
                      {log.request_id.slice(0, 8)}...
                    </td>
                  </tr>
                  {expanded === log.request_id && (
                    <tr key={`${log.request_id}-detail`}>
                      <td colSpan={7} className={styles.detailCell}>
                        <div className={styles.detail}>
                          <div><strong>Request ID:</strong> {log.request_id}</div>
                          <div><strong>Full Path:</strong> {log.path}</div>
                          <div><strong>Timestamp:</strong> {log.timestamp}</div>
                          <div><strong>Latency:</strong> {log.latency_ms}ms</div>
                          {log.trace && (
                            <div>
                              <strong>Trace:</strong>
                              <pre className={styles.trace}>{log.trace}</pre>
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
