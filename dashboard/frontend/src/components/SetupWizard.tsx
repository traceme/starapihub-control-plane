import { useState, useEffect } from 'react';
import { getWizardStatus, wizardProvider, wizardModel, wizardTest } from '../api';
import styles from './SetupWizard.module.css';

const STEPS = ['Add Provider', 'Create Model', 'Test Request', 'Done'];

export default function SetupWizard() {
  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(true);
  const [completed, setCompleted] = useState(false);
  const [error, setError] = useState('');

  // Step 1
  const [providerName, setProviderName] = useState('');
  const [apiKey, setApiKey] = useState('');

  // Step 2
  const [modelName, setModelName] = useState('');
  const [upstreamModel, setUpstreamModel] = useState('');

  // Step 3
  const [testResult, setTestResult] = useState<{ success: boolean; response: string; api_key?: string } | null>(null);

  // Step 4
  const [finalKey, setFinalKey] = useState('');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const status = await getWizardStatus();
        if (status.completed) {
          setCompleted(true);
        } else {
          setStep(status.current_step || 0);
        }
      } catch {
        // Wizard endpoint may not exist yet — start from scratch
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const handleProvider = async () => {
    setError('');
    try {
      await wizardProvider({ name: providerName, api_key: apiKey });
      setStep(1);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    }
  };

  const handleModel = async () => {
    setError('');
    try {
      await wizardModel({ name: modelName, upstream_model: upstreamModel });
      setStep(2);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed');
    }
  };

  const handleTest = async () => {
    setError('');
    try {
      const result = await wizardTest();
      setTestResult(result);
      if (result.api_key) setFinalKey(result.api_key);
      if (result.success) setStep(3);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Test failed');
    }
  };

  const copyKey = async () => {
    try {
      await navigator.clipboard.writeText(finalKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback: select text
    }
  };

  if (loading) return <p className={styles.empty}>Loading wizard...</p>;

  if (completed) {
    return (
      <div className={styles.page}>
        <div className={styles.card}>
          <h2 className={styles.doneTitle}>Setup Complete</h2>
          <p className={styles.doneText}>Your StarAPIHub stack is configured and running. Use the other pages to manage models, monitor cookies, and view logs.</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.page}>
      {/* Stepper */}
      <div className={styles.stepper}>
        {STEPS.map((label, i) => (
          <div
            key={i}
            className={`${styles.stepItem} ${i === step ? styles.stepActive : ''} ${i < step ? styles.stepDone : ''}`}
          >
            <span className={styles.stepCircle}>{i < step ? '\u2713' : i + 1}</span>
            <span className={styles.stepLabel}>{label}</span>
          </div>
        ))}
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {/* Step 1: Provider */}
      {step === 0 && (
        <div className={styles.card}>
          <h3 className={styles.cardTitle}>Add API Provider</h3>
          <p className={styles.cardDesc}>Enter a provider name and its API key. This will configure the upstream connection.</p>
          <div className={styles.form}>
            <label className={styles.field}>
              <span>Provider Name</span>
              <input
                value={providerName}
                onChange={(e) => setProviderName(e.target.value)}
                placeholder="e.g., anthropic, openai"
              />
            </label>
            <label className={styles.field}>
              <span>API Key</span>
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="sk-..."
              />
            </label>
            <button
              className={styles.btn}
              onClick={handleProvider}
              disabled={!providerName.trim() || !apiKey.trim()}
            >
              Continue
            </button>
          </div>
        </div>
      )}

      {/* Step 2: Model */}
      {step === 1 && (
        <div className={styles.card}>
          <h3 className={styles.cardTitle}>Create a Model</h3>
          <p className={styles.cardDesc}>Define a logical model name and map it to the upstream model identifier.</p>
          <div className={styles.form}>
            <label className={styles.field}>
              <span>Model Name</span>
              <input
                value={modelName}
                onChange={(e) => setModelName(e.target.value)}
                placeholder="e.g., claude-sonnet"
              />
            </label>
            <label className={styles.field}>
              <span>Upstream Model</span>
              <input
                value={upstreamModel}
                onChange={(e) => setUpstreamModel(e.target.value)}
                placeholder="e.g., claude-sonnet-4-20250514"
              />
            </label>
            <button
              className={styles.btn}
              onClick={handleModel}
              disabled={!modelName.trim() || !upstreamModel.trim()}
            >
              Continue
            </button>
          </div>
        </div>
      )}

      {/* Step 3: Test */}
      {step === 2 && (
        <div className={styles.card}>
          <h3 className={styles.cardTitle}>Test Request</h3>
          <p className={styles.cardDesc}>Send a test request through the full stack to verify everything works.</p>
          <pre className={styles.curlBlock}>{`curl -X POST http://localhost:3000/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${modelName || 'claude-sonnet'}","messages":[{"role":"user","content":"Hello"}]}'`}</pre>
          <button className={styles.btn} onClick={handleTest}>
            Run Test
          </button>
          {testResult && (
            <div className={`${styles.testResult} ${testResult.success ? styles.testOk : styles.testFail}`}>
              <strong>{testResult.success ? 'Success' : 'Failed'}</strong>
              <pre className={styles.testPre}>{testResult.response}</pre>
            </div>
          )}
        </div>
      )}

      {/* Step 4: Done */}
      {step === 3 && (
        <div className={styles.card}>
          <h3 className={styles.doneTitle}>All Set!</h3>
          <p className={styles.doneText}>Your StarAPIHub stack is ready to use.</p>
          {finalKey && (
            <div className={styles.keyBox}>
              <label className={styles.keyLabel}>Your API Key</label>
              <div className={styles.keyRow}>
                <code className={styles.keyValue}>{finalKey}</code>
                <button className={styles.copyBtn} onClick={copyKey}>
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
