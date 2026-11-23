import { createContext, Fragment, useCallback, useContext, useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import { ApiError, currentSession, errorText, logout, request, subscribeSession } from './http';
import { AuthForm, CreateWorkspace, Gate } from './SignIn';

export type Role = 'sales_manager' | 'head_of_sales' | 'support' | 'admin';
export type Workspace = { id: string; name: string; role: Role };
type Profile = { id: string; email: string; workspaces: Workspace[] };
export const roles: Record<Role, string> = {
  sales_manager: 'Sales manager', head_of_sales: 'Head of sales', support: 'Support', admin: 'Administrator',
};
type Account = {
  request: typeof request; workspace: Workspace; profile: Profile;
  selectWorkspace: (id: string) => void;
  reload: () => Promise<void>;
};
const Context = createContext<Account | null>(null);
export function useCrm(): Account {
  const account = useContext(Context);
  if (!account) throw new Error('CRM workspace is required');
  return account;
}

function SignedIn({ children }: {children: ReactNode}) {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [issue, setIssue] = useState('');
  const [selected, setSelected] = useState(() => {
    try { return sessionStorage.getItem('linea.workspace') ?? ''; } catch { return ''; }
  });
  const latestProfileRequest = useRef(0);
  const reload = useCallback(async (signal?: AbortSignal) => {
    const sequence = ++latestProfileRequest.current;
    const isCurrent = () => !signal?.aborted && sequence === latestProfileRequest.current;
    try {
      const me = await request<Profile>('/auth/me', { signal });
      if (isCurrent()) { setProfile(me); setIssue(''); }
    } catch (error) {
      if (isCurrent()) setIssue(errorText(error));
    }
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    const update = () => { if (!document.hidden) void reload(controller.signal); };
    void reload(controller.signal);
    window.addEventListener('focus', update);
    const timer = window.setInterval(update, 30000);
    return () => { controller.abort(); window.removeEventListener('focus', update); window.clearInterval(timer); };
  }, [reload]);
  const scopedRequest: typeof request = useCallback(async <T,>(path: string, init?: RequestInit) => {
    try { return await request<T>(path, init); }
    catch (error) {
      if (error instanceof ApiError && (error.status === 403 || error.status === 404)) void reload();
      throw error;
    }
  }, [reload]);
  function selectWorkspace(id: string) {
    setSelected(id);
    try { sessionStorage.setItem('linea.workspace', id); } catch { /* Keep the selection in memory. */ }
  }
  if (issue) return <Gate title="Workspace unavailable"><p role="alert">{issue}</p>
    <button onClick={() => void reload()}>Try again</button><button onClick={() => void logout().catch(() => {})}>Sign out</button></Gate>;
  if (!profile) return <Gate title="Opening your workspace"><p role="status">Loading…</p></Gate>;
  const workspace = profile.workspaces.find(w => w.id === selected) ?? profile.workspaces[0];
  if (!workspace) return <CreateWorkspace email={profile.email} userID={profile.id} onCreated={async id => { selectWorkspace(id); await reload(); }}/>;
  return <Context.Provider value={{ profile, workspace, request: scopedRequest, selectWorkspace, reload }}>
    <Fragment key={[profile.id, workspace.id, workspace.role].join('/')}>{children}</Fragment>
  </Context.Provider>;
}

export function CrmSession({ children }: { children: ReactNode }) {
  const session = useSyncExternalStore(subscribeSession, currentSession);
  return session ? <SignedIn>{children}</SignedIn> : <AuthForm/>;
}
