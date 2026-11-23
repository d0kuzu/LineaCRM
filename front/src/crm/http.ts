type Tokens = { access_token: string; refresh_token: string };
type Session = { tokens: Tokens; refresh?: Promise<void> };
const key = 'linea.auth.v1';
const listeners = new Set<() => void>();

function validTokens(value: unknown): value is Tokens {
  if (!value || typeof value !== 'object') return false;
  return 'access_token' in value && typeof value.access_token === 'string' && !!value.access_token &&
    'refresh_token' in value && typeof value.refresh_token === 'string' && !!value.refresh_token;
}

function restore(): Session | null {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(key) ?? 'null');
    return validTokens(value) ? { tokens: value } : null;
  } catch { return null; }
}

let active = restore();
export const currentSession = () => active;
export function subscribeSession(callback: () => void) {
  listeners.add(callback);
  return () => { listeners.delete(callback); };
}
function persist() {
  try {
    if (active) sessionStorage.setItem(key, JSON.stringify(active.tokens));
    else sessionStorage.removeItem(key);
  } catch { /* The tab can still use an in-memory session. */ }
}
function replace(session: Session | null) {
  active = session;
  persist();
  listeners.forEach(notify => notify());
}

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) { super(message); }
}

async function send<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && init.body !== null) headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', 'Bearer ' + token);
  const response = await fetch(path, { ...init, headers, credentials: 'omit', cache: 'no-store' });
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    const message = typeof body?.error === 'string' ? body.error : 'Service unavailable. Please try again.';
    throw new ApiError(response.status, message);
  }
  return response.status === 204 ? undefined as T : await response.json() as T;
}
const post = (body: unknown): RequestInit => ({ method: 'POST', body: JSON.stringify(body) });

export async function authenticate(mode: 'login' | 'register', email: string, password: string) {
  const value = await send<Tokens>('/auth/' + mode, post({ email, password }));
  if (!validTokens(value)) throw new Error('Invalid sign-in response.');
  replace({ tokens: value });
}

export const resetPassword = (token: string, password: string) =>
  send<void>('/auth/password/reset', post({ token, password }));

function revoke(tokens: Tokens) {
  return send<void>('/auth/logout', post({ refresh_token: tokens.refresh_token }));
}

export async function logout() {
  const session = active;
  replace(null);
  if (session) await revoke(session.tokens);
}

async function rotate(session: Session) {
  let tokens: Tokens;
  try {
    tokens = await send<Tokens>('/auth/refresh', post({ refresh_token: session.tokens.refresh_token }));
    if (!validTokens(tokens)) throw new Error('Invalid session response.');
  } catch (error) {
    if (active === session && error instanceof ApiError && error.status === 401) replace(null);
    throw error;
  }
  if (active !== session) {
    await revoke(tokens);
    throw new ApiError(401, 'Session ended.');
  }
  session.tokens = tokens;
  persist();
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const session = active;
  if (!session) throw new ApiError(401, 'Sign in to continue.');
  const token = session.tokens.access_token;
  let value: T;
  try {
    value = await send<T>(path, init, token);
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 401 || session !== active) throw error;
    if (session.tokens.access_token === token) {
      session.refresh ??= rotate(session).finally(() => { session.refresh = undefined; });
      await session.refresh;
    }
    if (session !== active) throw new ApiError(401, 'Session ended.');
    value = await send<T>(path, init, session.tokens.access_token);
  }
  if (active !== session) throw new ApiError(401, 'Session ended.');
  return value;
}

export function errorText(error: unknown) {
  if (error instanceof ApiError) return error.message;
  return 'Could not reach Linea. Check the connection and try again.';
}
