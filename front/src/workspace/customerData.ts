import { useEffect, useState } from 'react';
import { useCrm } from '../crm/session';

export type CustomerKind = 'companies' | 'contacts';
export type CustomerRef = { kind: CustomerKind; id: string };
export interface Customer {
  id: string; workspace_id: string; owner_id: string; name: string; email: string; phone: string;
  tags: string[]; segment: string; position?: string; company_id?: string | null;
  custom_fields?: Record<string, string>;
  created_at: string; updated_at: string;
}
export interface Member { user_id: string; email: string; role: string }
export interface FieldDefinition { id: string; label: string; required: boolean; builtin: boolean }
export const singular = { companies: 'company', contacts: 'contact' } as const;
export const customerLink = ({ kind, id }: CustomerRef) => '#/customers/' + kind + '/' + encodeURIComponent(id);
export function accessError(error: unknown) {
  return typeof error === 'object' && error !== null && 'status' in error &&
    (error.status === 401 || error.status === 403 || error.status === 404);
}
export function errorMessage(error: unknown) {
  return error instanceof Error && error.message ? error.message : 'The request could not be completed.';
}
export function timestamp(value: string, locale: string) {
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}

interface Result<T> { path: string; revision: number; data?: T; error?: unknown }

// A response belongs to one URL and one revision. Changing either immediately
// hides the old result, even before React runs the next effect.
export function useCustomerData<T>(path: string | null, revision = 0, delay = 0) {
  const { request } = useCrm();
  const [result, setResult] = useState<Result<T> | null>(null);
  useEffect(() => {
    if (!path) return;
    const controller = new AbortController();
    async function read(url: string) {
      try {
        const data = await request<T>(url, { signal: controller.signal });
        if (!controller.signal.aborted) setResult({ path: url, revision, data });
      } catch (error) {
        if (!controller.signal.aborted) setResult({ path: url, revision, error });
      }
    }
    const timer = window.setTimeout(() => { void read(path); }, delay);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [path, revision, request, delay]);
  return result && result.path === path && result.revision === revision ? result : null;
}

export function useCustomerFields(kind: CustomerKind, revision: number) {
  const { workspace } = useCrm();
  return useCustomerData<FieldDefinition[]>('/crm/workspaces/' + workspace.id + '/fields/' + kind, revision);
}
