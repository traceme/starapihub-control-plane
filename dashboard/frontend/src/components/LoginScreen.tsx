import { useState } from 'react';
import styles from './LoginScreen.module.css';

interface Props {
  onLogin: (token: string) => void;
}

export default function LoginScreen({ onLogin }: Props) {
  const [value, setValue] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const token = value.trim();
    if (!token) return;

    setLoading(true);
    setError('');

    try {
      const res = await fetch(`/api/sse?token=${encodeURIComponent(token)}`, {
        headers: { Authorization: `Bearer ${token}` },
        signal: AbortSignal.timeout(5000),
      });
      if (res.status === 401) {
        setError('Invalid token. Please check and try again.');
        setLoading(false);
        return;
      }
      // Any non-401 is acceptable — the SSE endpoint may stay open
      res.body?.cancel();
      onLogin(token);
    } catch {
      // Network errors are OK — backend might not be running yet
      // Store the token and let the main app handle reconnects
      onLogin(token);
    }
  };

  return (
    <div className={styles.wrapper}>
      <form className={styles.card} onSubmit={handleSubmit}>
        <div className={styles.logo}>&#9733;</div>
        <h1 className={styles.title}>StarAPIHub Command Center</h1>
        <p className={styles.subtitle}>Enter your API token to connect</p>
        <input
          type="password"
          className={styles.input}
          placeholder="Bearer token"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoFocus
        />
        {error && <p className={styles.error}>{error}</p>}
        <button className={styles.btn} type="submit" disabled={loading || !value.trim()}>
          {loading ? 'Connecting...' : 'Connect'}
        </button>
      </form>
    </div>
  );
}
