import { useState, useEffect, useCallback } from 'react';
import type { LogicalModel, ProviderEntry } from '../types';
import { fetchModels, createModel, updateModel, deleteModel } from '../api';
import styles from './ModelEditor.module.css';

const EMPTY_PROVIDER: ProviderEntry = { provider_id: '', upstream_model: '', weight: 1, priority: 0 };

function emptyModel(): Omit<LogicalModel, 'id'> {
  return {
    name: '',
    display_name: '',
    risk_level: 'low',
    channel: '',
    providers: [{ ...EMPTY_PROVIDER }],
  };
}

export default function ModelEditor() {
  const [models, setModels] = useState<LogicalModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editing, setEditing] = useState<LogicalModel | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState<Omit<LogicalModel, 'id'>>(emptyModel());
  const [saving, setSaving] = useState(false);
  const [pendingChanges, setPendingChanges] = useState<string[]>([]);

  const load = useCallback(async () => {
    try {
      const data = await fetchModels();
      setModels(data);
      setError('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load models');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const startCreate = () => {
    setEditing(null);
    setForm(emptyModel());
    setCreating(true);
  };

  const startEdit = (m: LogicalModel) => {
    setCreating(false);
    setEditing(m);
    setForm({ name: m.name, display_name: m.display_name, risk_level: m.risk_level, channel: m.channel, providers: [...m.providers] });
  };

  const cancel = () => {
    setCreating(false);
    setEditing(null);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (editing) {
        await updateModel(editing.id, form);
        setPendingChanges((p) => [...p, `Updated ${form.name}`]);
      } else {
        await createModel(form);
        setPendingChanges((p) => [...p, `Created ${form.name}`]);
      }
      cancel();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (m: LogicalModel) => {
    if (!confirm(`Delete model "${m.name}"?`)) return;
    try {
      await deleteModel(m.id);
      setPendingChanges((p) => [...p, `Deleted ${m.name}`]);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed');
    }
  };

  const updateProvider = (idx: number, field: keyof ProviderEntry, value: string | number) => {
    setForm((f) => {
      const providers = [...f.providers];
      providers[idx] = { ...providers[idx], [field]: value };
      return { ...f, providers };
    });
  };

  const addProvider = () => {
    setForm((f) => ({ ...f, providers: [...f.providers, { ...EMPTY_PROVIDER, priority: f.providers.length }] }));
  };

  const removeProvider = (idx: number) => {
    setForm((f) => ({ ...f, providers: f.providers.filter((_, i) => i !== idx) }));
  };

  const moveProvider = (idx: number, dir: -1 | 1) => {
    setForm((f) => {
      const providers = [...f.providers];
      const target = idx + dir;
      if (target < 0 || target >= providers.length) return f;
      [providers[idx], providers[target]] = [providers[target], providers[idx]];
      return { ...f, providers: providers.map((p, i) => ({ ...p, priority: i })) };
    });
  };

  const showForm = creating || editing;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h2 className={styles.title}>Model Routing</h2>
        <button className={styles.addBtn} onClick={startCreate}>+ Add Model</button>
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {pendingChanges.length > 0 && (
        <div className={styles.pending}>
          <strong>Pending Changes:</strong>
          <ul>{pendingChanges.map((c, i) => <li key={i}>{c}</li>)}</ul>
          <button className={styles.applyBtn} onClick={() => setPendingChanges([])}>Clear</button>
        </div>
      )}

      {showForm && (
        <div className={styles.formCard}>
          <h3 className={styles.formTitle}>{editing ? 'Edit Model' : 'New Model'}</h3>
          <div className={styles.formGrid}>
            <label className={styles.field}>
              <span>Name</span>
              <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="claude-sonnet" />
            </label>
            <label className={styles.field}>
              <span>Display Name</span>
              <input value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} placeholder="Claude Sonnet" />
            </label>
            <label className={styles.field}>
              <span>Risk Level</span>
              <select value={form.risk_level} onChange={(e) => setForm({ ...form, risk_level: e.target.value as 'low' | 'medium' | 'high' })}>
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </select>
            </label>
            <label className={styles.field}>
              <span>Channel</span>
              <input value={form.channel} onChange={(e) => setForm({ ...form, channel: e.target.value })} placeholder="default" />
            </label>
          </div>

          <h4 className={styles.subTitle}>Provider Chain (priority order)</h4>
          {form.providers.map((p, i) => (
            <div key={i} className={styles.providerRow}>
              <span className={styles.providerIdx}>#{i + 1}</span>
              <input className={styles.providerInput} placeholder="Provider ID" value={p.provider_id} onChange={(e) => updateProvider(i, 'provider_id', e.target.value)} />
              <input className={styles.providerInput} placeholder="Upstream Model" value={p.upstream_model} onChange={(e) => updateProvider(i, 'upstream_model', e.target.value)} />
              <label className={styles.weightLabel}>
                W:
                <input type="range" min={0} max={10} step={1} value={p.weight} onChange={(e) => updateProvider(i, 'weight', Number(e.target.value))} />
                <span className={styles.weightVal}>{p.weight}</span>
              </label>
              <button className={styles.iconBtn} onClick={() => moveProvider(i, -1)} disabled={i === 0} title="Move up">&uarr;</button>
              <button className={styles.iconBtn} onClick={() => moveProvider(i, 1)} disabled={i === form.providers.length - 1} title="Move down">&darr;</button>
              <button className={styles.iconBtn} onClick={() => removeProvider(i)} title="Remove">&times;</button>
            </div>
          ))}
          <button className={styles.linkBtn} onClick={addProvider}>+ Add Provider</button>

          <div className={styles.formActions}>
            <button className={styles.cancelBtn} onClick={cancel}>Cancel</button>
            <button className={styles.saveBtn} onClick={handleSave} disabled={saving || !form.name.trim()}>
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      )}

      {loading ? (
        <p className={styles.empty}>Loading models...</p>
      ) : models.length === 0 ? (
        <p className={styles.empty}>No models configured. Click "Add Model" to create one.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Name</th>
                <th>Display Name</th>
                <th>Providers</th>
                <th>Channel</th>
                <th>Risk</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {models.map((m) => (
                <tr key={m.id}>
                  <td className={styles.mono}>{m.name}</td>
                  <td>{m.display_name}</td>
                  <td className={styles.providerCell}>
                    {m.providers.map((p, i) => (
                      <span key={i} className={styles.providerTag}>
                        {p.provider_id}:{p.upstream_model} (w{p.weight})
                      </span>
                    ))}
                  </td>
                  <td className={styles.mono}>{m.channel}</td>
                  <td>
                    <span className={`${styles.riskBadge} ${styles[`risk_${m.risk_level}`]}`}>
                      {m.risk_level}
                    </span>
                  </td>
                  <td>
                    <button className={styles.tblBtn} onClick={() => startEdit(m)}>Edit</button>
                    <button className={`${styles.tblBtn} ${styles.tblBtnDanger}`} onClick={() => handleDelete(m)}>Delete</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
