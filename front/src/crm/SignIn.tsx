import { useState, type FormEvent, type ReactNode } from 'react';
import { authenticate, errorText, logout, request, resetPassword } from './http';
import './account.css';

export function Gate({ title, children }: { title: string; children: ReactNode }) {
  return <main className="account-page">
    <section className="account-card">
      <div className="account-brand"><img src="/icons/linea-brand-mark.svg" alt=""/>Linea</div>
      <h1>{title}</h1>{children}
    </section>
  </main>;
}

export function AuthForm() {
  const [mode, setMode] = useState<'login' | 'register' | 'reset'>('login');
  const [issue, setIssue] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const title = mode === 'login' ? 'Sign in' : mode === 'register' ? 'Create account' : 'Reset password';
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const password = String(data.get('password'));
    setIssue(''); setNotice(''); setBusy(true);
    try {
      if (mode === 'reset') {
        await resetPassword(String(data.get('code')).trim(), password);
        setMode('login'); setNotice('Password updated. Sign in with your new password.');
      } else {
        await authenticate(mode, String(data.get('email')).trim(), password);
      }
    } catch (error) { setIssue(errorText(error)); }
    finally { setBusy(false); }
  }
  function change(next: typeof mode) { setMode(next); setNotice(''); setIssue(''); }
  return <Gate title={title}>
    {mode === 'reset' && <p>Request a recovery code from your operator and enter it below.</p>}
    <form key={mode} onSubmit={submit}>
      {mode === 'reset' ? <label>Recovery code<input autoComplete="one-time-code" name="code" required/></label>
        : <label>Email<input name="email" type="email" autoComplete="username" required/></label>}
      <label>{mode === 'reset' ? 'New password' : 'Password'}<input name="password" type="password" required
        autoComplete={mode === 'login' ? 'current-password' : 'new-password'}/></label>
      {issue && <p role="alert">{issue}</p>}{notice && <p role="status">{notice}</p>}
      <button className="account-primary" disabled={busy}>{busy ? 'Please wait…' : title}</button>
    </form>
    <div className="account-links">
      {mode !== 'login' && <button disabled={busy} onClick={() => change('login')}>Back to sign in</button>}
      {mode === 'login' && <><button disabled={busy} onClick={() => change('register')}>Create account</button>
        <button disabled={busy} onClick={() => change('reset')}>Forgot password?</button></>}
    </div>
  </Gate>;
}

export function CreateWorkspace({ email, userID, onCreated }: { email: string; userID: string; onCreated: (id: string) => Promise<void> }) {
  const [issue, setIssue] = useState('');
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = String(new FormData(event.currentTarget).get('name')).trim();
    if (!name) return;
    setBusy(true); setIssue('');
    try {
      const result = await request<{ id: string }>('/auth/workspaces', { method: 'POST', body: JSON.stringify({ name }) });
      await onCreated(result.id);
    } catch (error) { setIssue(errorText(error)); } finally { setBusy(false); }
  }
  return <Gate title="Set up your workspace">
    <p>Signed in as {email}. To join an existing team, give its administrator your account ID.</p>
    <code className="account-id">{userID}</code>
    <form onSubmit={submit}>
      <label>New workspace name<input name="name" required maxLength={160}/></label>
      {issue && <p role="alert">{issue}</p>}
      <button className="account-primary" disabled={busy}>Create workspace</button>
    </form>
    <div className="account-links"><button onClick={() => window.location.reload()}>Check membership</button>
      <button onClick={() => void logout().catch(() => {})}>Sign out</button></div>
  </Gate>;
}
