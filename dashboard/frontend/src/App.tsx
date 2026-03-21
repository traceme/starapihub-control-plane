import { useState, useEffect, useCallback, useRef } from 'react';
import { Routes, Route, NavLink, Navigate } from 'react-router-dom';
import type { SSEState } from './types';
import { connectSSE } from './api';
import LoginScreen from './components/LoginScreen';
import HealthDashboard from './components/HealthDashboard';
import CookiePanel from './components/CookiePanel';
import ModelEditor from './components/ModelEditor';
import LogViewer from './components/LogViewer';
import SetupWizard from './components/SetupWizard';
import AlertBanner from './components/AlertBanner';
import styles from './App.module.css';

const NAV_ITEMS = [
  { to: '/', label: 'Home', icon: '\u25A0' },
  { to: '/models', label: 'Models', icon: '\u25C6' },
  { to: '/cookies', label: 'Cookies', icon: '\u25CF' },
  { to: '/logs', label: 'Logs', icon: '\u25B6' },
  { to: '/setup', label: 'Setup', icon: '\u2699' },
];

const EMPTY_STATE: SSEState = {
  health: {},
  cookies: {},
  log_stats: {
    request_rate: 0,
    p50_latency_ms: 0,
    p99_latency_ms: 0,
    error_rate: 0,
    by_model: {},
    by_status: {},
  },
  alerts: [],
};

export default function App() {
  const [token, setToken] = useState(() => localStorage.getItem('starapihub_token') || '');
  const [connected, setConnected] = useState(false);
  const [state, setState] = useState<SSEState>(EMPTY_STATE);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const disconnectRef = useRef<(() => void) | null>(null);

  const handleLogin = useCallback((t: string) => {
    localStorage.setItem('starapihub_token', t);
    setToken(t);
  }, []);

  const handleLogout = useCallback(() => {
    localStorage.removeItem('starapihub_token');
    setToken('');
    setConnected(false);
    if (disconnectRef.current) disconnectRef.current();
  }, []);

  useEffect(() => {
    if (!token) return;

    const disconnect = connectSSE(
      token,
      (data) => {
        setState(data as SSEState);
      },
      (err) => {
        console.error('SSE error:', err);
        setConnected(false);
      },
      () => setConnected(true),
    );
    disconnectRef.current = disconnect;

    return () => {
      disconnect();
      disconnectRef.current = null;
    };
  }, [token]);

  if (!token) {
    return <LoginScreen onLogin={handleLogin} />;
  }

  return (
    <div className={styles.layout}>
      <aside className={`${styles.sidebar} ${sidebarOpen ? styles.sidebarOpen : ''}`}>
        <div className={styles.brand}>
          <span className={styles.brandIcon}>&#9733;</span>
          <span className={styles.brandText}>StarAPIHub</span>
        </div>
        <nav className={styles.nav}>
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `${styles.navItem} ${isActive ? styles.navActive : ''}`
              }
              onClick={() => setSidebarOpen(false)}
            >
              <span className={styles.navIcon}>{item.icon}</span>
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className={styles.sidebarFooter}>
          <button className={styles.logoutBtn} onClick={handleLogout}>
            Sign Out
          </button>
        </div>
      </aside>

      <div className={styles.main}>
        <header className={styles.topbar}>
          <button
            className={styles.hamburger}
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label="Toggle menu"
          >
            &#9776;
          </button>
          <h1 className={styles.pageTitle}>Command Center</h1>
          <div className={styles.topbarRight}>
            <span
              className={`${styles.connDot} ${connected ? styles.connGreen : styles.connRed}`}
            />
            <span className={styles.connLabel}>
              {connected ? 'Connected' : 'Disconnected'}
            </span>
          </div>
        </header>

        <AlertBanner alerts={state.alerts} />

        <main className={styles.content}>
          <Routes>
            <Route path="/" element={<HealthDashboard state={state} />} />
            <Route path="/cookies" element={<CookiePanel cookies={state.cookies} />} />
            <Route path="/models" element={<ModelEditor />} />
            <Route path="/logs" element={<LogViewer />} />
            <Route path="/setup" element={<SetupWizard />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </div>

      {sidebarOpen && (
        <div className={styles.overlay} onClick={() => setSidebarOpen(false)} />
      )}
    </div>
  );
}
