import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { errorText, logout } from './http';
import { roles, useCrm, type Role } from './session';

type Member = { user_id: string; email: string; role: Role };
export function TeamDialog({ onClose }: { onClose: () => void }) {
  const { request, profile, workspace, reload } = useCrm();
  const ref = useRef<HTMLDialogElement>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [issue, setIssue] = useState('');
  const [busy, setBusy] = useState(false);
  const base = '/auth/workspaces/' + workspace.id + '/members';
  const admin = workspace.role === 'admin';
  const read = useCallback(async (signal?: AbortSignal) => {
    try { const result = await request<Member[]>(base, { signal }); if (!signal?.aborted) setMembers(result); }
    catch (error) { if (!signal?.aborted) { setMembers([]); setIssue(errorText(error)); } }
  }, [base, request]);
  useEffect(() => {
    const dialog = ref.current; dialog?.showModal();
    const controller = new AbortController(); void read(controller.signal);
    return () => { controller.abort(); dialog?.close(); };
  }, [read]);
  async function update(method: string, id: string, body?: object) {
    setBusy(true); setIssue('');
    try {
      await request(base + (id ? '/' + encodeURIComponent(id) : ''), { method, body: body && JSON.stringify(body) });
      await Promise.all([read(), reload()]);
      return true;
    } catch (error) { setIssue(errorText(error)); return false; }
    finally { setBusy(false); }
  }
  async function add(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    if (await update('POST', '', { user_id: String(data.get('account')).trim(), role: data.get('role') })) form.reset();
  }
  const choices = Object.entries(roles).map(([role, label]) => <option key={role} value={role}>{label}</option>);
  return <dialog ref={ref} className="team-dialog" aria-label="Workspace members" onCancel={onClose}>
    <header><h2>{workspace.name}</h2><button onClick={onClose} aria-label="Close">×</button></header>
    <p>{profile.email} · {roles[workspace.role]}</p>
    <p>Your account ID <code className="account-id">{profile.id}</code></p>
    <h3>Workspace members</h3>
    {issue && <p role="alert">{issue}</p>}
    <ul>{members.map(member => <li key={member.user_id}><span>{member.email}</span>
      {admin ? <>
        <select aria-label={'Role for ' + member.email} value={member.role} disabled={busy}
          onChange={event => void update('PATCH', member.user_id, {role: event.target.value})}>{choices}</select>
        <button disabled={busy} onClick={() => { if (window.confirm('Remove ' + member.email + ' from this workspace?')) void update('DELETE', member.user_id); }}>Remove</button>
      </> : <small>{roles[member.role]}</small>}
    </li>)}</ul>
    {admin && <><h3>Add a member</h3><form onSubmit={add}>
      <input name="account" aria-label="Account ID" placeholder="Account ID" required/>
      <select name="role" aria-label="New member role" defaultValue="sales_manager">{choices}</select>
      <button disabled={busy}>Add member</button>
    </form></>}
    <div className="team-actions"><button onClick={() => void logout().catch(() => {})}>Sign out</button>
      <button onClick={onClose}>Done</button></div>
  </dialog>;
}
