import { useState, useEffect, useCallback } from 'react';
import { fetchSyncStatus, fetchBootstrapStatus, fetchAuditLog } from '../api';
import type { AuditEntry } from '../api';

type Tab = 'sync' | 'bootstrap' | 'audit';

export default function OpsPanel() {
  const [tab, setTab] = useState<Tab>('sync');
  const [syncData, setSyncData] = useState<AuditEntry | null>(null);
  const [bootData, setBootData] = useState<AuditEntry | null>(null);
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      if (tab === 'sync') {
        const data = await fetchSyncStatus();
        if ('total_actions' in data) setSyncData(data as AuditEntry);
        else setSyncData(null);
      } else if (tab === 'bootstrap') {
        const data = await fetchBootstrapStatus();
        if ('total_actions' in data) setBootData(data as AuditEntry);
        else setBootData(null);
      } else {
        const { entries } = await fetchAuditLog(20);
        setAuditEntries(entries || []);
      }
    } catch (e) {
      setError(String(e));
    }
    setLoading(false);
  }, [tab]);

  useEffect(() => { refresh(); }, [refresh]);

  return (
    <div style={{ padding: '1rem' }}>
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        {(['sync', 'bootstrap', 'audit'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            style={{
              padding: '0.5rem 1rem',
              background: tab === t ? '#2563eb' : '#374151',
              color: '#fff',
              border: 'none',
              borderRadius: '0.25rem',
              cursor: 'pointer',
              textTransform: 'capitalize',
            }}
          >
            {t}
          </button>
        ))}
        <button
          onClick={refresh}
          style={{ marginLeft: 'auto', padding: '0.5rem 1rem', background: '#1f2937', color: '#9ca3af', border: '1px solid #4b5563', borderRadius: '0.25rem', cursor: 'pointer' }}
        >
          {loading ? 'Loading...' : 'Refresh'}
        </button>
      </div>

      {error && <div style={{ color: '#ef4444', marginBottom: '0.5rem' }}>{error}</div>}

      {tab === 'sync' && <SyncView entry={syncData} />}
      {tab === 'bootstrap' && <BootstrapView entry={bootData} />}
      {tab === 'audit' && <AuditLogView entries={auditEntries} />}
    </div>
  );
}

function SyncView({ entry }: { entry: AuditEntry | null }) {
  if (!entry) return <NoData label="sync" cmd="starapihub sync" />;
  return (
    <div>
      <h3 style={{ margin: '0 0 0.5rem' }}>Last Sync</h3>
      <StatusSummary entry={entry} />
      {entry.changes && entry.changes.length > 0 && <ChangesTable changes={entry.changes} />}
    </div>
  );
}

function BootstrapView({ entry }: { entry: AuditEntry | null }) {
  if (!entry) return <NoData label="bootstrap" cmd="starapihub bootstrap" />;
  return (
    <div>
      <h3 style={{ margin: '0 0 0.5rem' }}>Last Bootstrap</h3>
      <StatusSummary entry={entry} />
      {entry.bootstrap_steps && (
        <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: '0.5rem', fontSize: '0.875rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #374151', textAlign: 'left' }}>
              <th style={{ padding: '0.5rem' }}>Step</th>
              <th style={{ padding: '0.5rem' }}>Status</th>
              <th style={{ padding: '0.5rem' }}>Message</th>
            </tr>
          </thead>
          <tbody>
            {entry.bootstrap_steps.map((s, i) => (
              <tr key={i} style={{ borderBottom: '1px solid #1f2937' }}>
                <td style={{ padding: '0.5rem', fontFamily: 'monospace' }}>{s.name}</td>
                <td style={{ padding: '0.5rem' }}>
                  <StatusBadge status={s.status} />
                </td>
                <td style={{ padding: '0.5rem', color: '#9ca3af' }}>{s.message || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function AuditLogView({ entries }: { entries: AuditEntry[] }) {
  if (entries.length === 0) return <NoData label="audit log" cmd="starapihub sync" />;
  return (
    <div>
      <h3 style={{ margin: '0 0 0.5rem' }}>Audit Log (last {entries.length})</h3>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #374151', textAlign: 'left' }}>
            <th style={{ padding: '0.5rem' }}>Time</th>
            <th style={{ padding: '0.5rem' }}>Operation</th>
            <th style={{ padding: '0.5rem' }}>Actions</th>
            <th style={{ padding: '0.5rem' }}>OK</th>
            <th style={{ padding: '0.5rem' }}>Fail</th>
            <th style={{ padding: '0.5rem' }}>Duration</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e, i) => (
            <tr key={i} style={{ borderBottom: '1px solid #1f2937' }}>
              <td style={{ padding: '0.5rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{e.timestamp}</td>
              <td style={{ padding: '0.5rem' }}>{e.operation}</td>
              <td style={{ padding: '0.5rem' }}>{e.total_actions}</td>
              <td style={{ padding: '0.5rem', color: '#22c55e' }}>{e.succeeded}</td>
              <td style={{ padding: '0.5rem', color: e.failed > 0 ? '#ef4444' : '#6b7280' }}>{e.failed}</td>
              <td style={{ padding: '0.5rem' }}>{e.duration_ms}ms</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function StatusSummary({ entry }: { entry: AuditEntry }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))', gap: '0.5rem', marginBottom: '0.75rem' }}>
      <Stat label="Time" value={entry.timestamp} />
      <Stat label="Total" value={String(entry.total_actions)} />
      <Stat label="OK" value={String(entry.succeeded)} color="#22c55e" />
      <Stat label="Failed" value={String(entry.failed)} color={entry.failed > 0 ? '#ef4444' : undefined} />
      <Stat label="Drift" value={String(entry.drift_warnings)} color={entry.drift_warnings > 0 ? '#f59e0b' : undefined} />
      <Stat label="Duration" value={`${entry.duration_ms}ms`} />
    </div>
  );
}

function Stat({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div style={{ background: '#1f2937', borderRadius: '0.25rem', padding: '0.5rem' }}>
      <div style={{ fontSize: '0.75rem', color: '#9ca3af' }}>{label}</div>
      <div style={{ fontSize: '1rem', fontFamily: 'monospace', color: color || '#e5e7eb' }}>{value}</div>
    </div>
  );
}

function ChangesTable({ changes }: { changes: AuditEntry['changes'] }) {
  if (!changes || changes.length === 0) return null;
  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: '0.5rem', fontSize: '0.875rem' }}>
      <thead>
        <tr style={{ borderBottom: '1px solid #374151', textAlign: 'left' }}>
          <th style={{ padding: '0.5rem' }}>Type</th>
          <th style={{ padding: '0.5rem' }}>Resource</th>
          <th style={{ padding: '0.5rem' }}>Action</th>
          <th style={{ padding: '0.5rem' }}>Status</th>
          <th style={{ padding: '0.5rem' }}>Error</th>
        </tr>
      </thead>
      <tbody>
        {changes.map((c, i) => (
          <tr key={i} style={{ borderBottom: '1px solid #1f2937' }}>
            <td style={{ padding: '0.5rem', fontFamily: 'monospace' }}>{c.resource_type}</td>
            <td style={{ padding: '0.5rem', fontFamily: 'monospace' }}>{c.resource_id}</td>
            <td style={{ padding: '0.5rem' }}>{c.action}</td>
            <td style={{ padding: '0.5rem' }}><StatusBadge status={c.status} /></td>
            <td style={{ padding: '0.5rem', color: '#ef4444', fontSize: '0.75rem' }}>{c.error || '-'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    ok: '#22c55e',
    failed: '#ef4444',
    skipped: '#6b7280',
    'applied-with-drift': '#f59e0b',
    unverified: '#f59e0b',
  };
  return (
    <span style={{ color: colors[status] || '#9ca3af', fontWeight: 600 }}>
      {status}
    </span>
  );
}

function NoData({ label, cmd }: { label: string; cmd: string }) {
  return (
    <div style={{ color: '#6b7280', padding: '2rem', textAlign: 'center' }}>
      <div>No {label} data recorded yet.</div>
      <div style={{ marginTop: '0.5rem', fontFamily: 'monospace', fontSize: '0.875rem' }}>
        Run: {cmd}
      </div>
    </div>
  );
}
