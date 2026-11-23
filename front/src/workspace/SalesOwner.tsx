import { useEffect, useRef, useState, type FormEvent } from 'react';
import { ApiError } from '../crm/http';
import { useCrm } from '../crm/session';
import { accessError, errorMessage, useCustomerData, type Member } from './customerData';
import type { Deal } from './salesData';

export function canChangeOwner(role: string) {
  return role === 'admin' || role === 'head_of_sales';
}

export function useSalesOwners(enabled: boolean, onDenied: (cause: unknown) => void) {
  const { workspace } = useCrm();
  const [revision, setRevision] = useState(0);
  const roster = useCustomerData<Member[]>(
    enabled ? '/auth/workspaces/' + workspace.id + '/members' : null,
    revision,
  );
  useEffect(() => {
    if (accessError(roster?.error)) onDenied(roster?.error);
  }, [roster?.error, onDenied]);
  return {
    members: (roster?.data ?? []).filter(
      (member) => canChangeOwner(member.role) || member.role === 'sales_manager',
    ),
    loaded: !!roster?.data,
    error: roster?.error,
    reload: () => setRevision((n) => n + 1),
  };
}

export function SalesOwnerSelect({
  owner,
  label,
  roster,
  onChange,
}: {
  owner: string;
  label: string;
  roster: ReturnType<typeof useSalesOwners>;
  onChange: (id: string) => void;
}) {
  return (
    <>
      <label className="customer-wide">
        Owner
        <select
          aria-label="Owner"
          value={owner}
          disabled={!roster.loaded}
          onChange={(event) => onChange(event.target.value)}
        >
          {!roster.members.some((member) => member.user_id === owner) && (
            <option value={owner} disabled>
              {label}
            </option>
          )}
          {roster.members.map((member) => (
            <option key={member.user_id} value={member.user_id}>
              {member.email || member.user_id}
            </option>
          ))}
        </select>
      </label>
      {!roster.loaded && roster.error == null && <small role="status">Loading team members…</small>}
      {roster.error != null && (
        <p role="alert" className="customer-error">
          Could not load team members.{' '}
          <button type="button" className="ws-text-button" onClick={roster.reload}>
            Retry members
          </button>
        </p>
      )}
    </>
  );
}

export function ChangeDealOwner({
  deal,
  ownerLabel,
  onSaved,
  onCancel,
  onDenied,
}: {
  deal: Deal;
  ownerLabel: (id: string) => string;
  onSaved: () => void;
  onCancel: () => void;
  onDenied: (cause: unknown) => void;
}) {
  const { workspace, request } = useCrm();
  const roster = useSalesOwners(true, onDenied);
  const [owner, setOwner] = useState(deal.owner_id);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  const canSave = owner !== deal.owner_id && roster.members.some((member) => member.user_id === owner);

  async function save(event: FormEvent) {
    event.preventDefault();
    if (!canSave || active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setError('');
    setBusy(true);
    try {
      await request<Deal>('/crm/workspaces/' + workspace.id + '/deals/' + deal.id, {
        method: 'PATCH',
        body: JSON.stringify({ owner_id: owner }),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) onSaved();
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onDenied(cause);
        else {
          setError(errorMessage(cause));
          if (cause instanceof ApiError && cause.status === 400) roster.reload();
        }
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <form className="customer-form" onSubmit={save}>
      <fieldset className="customer-field-grid" disabled={busy}>
        <SalesOwnerSelect owner={owner} label={ownerLabel(owner)} roster={roster} onChange={setOwner} />
      </fieldset>
      {error && (
        <p role="alert" className="customer-error">
          {error}
        </p>
      )}
      <div className="customer-actions">
        <button className="ws-action" disabled={busy || !canSave}>
          {busy ? 'Saving…' : 'Save owner'}
        </button>
        <button type="button" className="ws-pill" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}
