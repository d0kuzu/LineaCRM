import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { accessError, errorMessage, useCustomerFields, type CustomerKind, type FieldDefinition } from './customerData';
import { RecordDialog } from './shared';

type Draft = { field?: FieldDefinition; label: string; required: boolean };

export function CustomerFieldSettings({ kind, onClose, onAccessError }: {
  kind: CustomerKind; onClose: () => void; onAccessError: (error: unknown) => void;
}) {
  const { workspace, request } = useCrm();
  const [revision, setRevision] = useState(0);
  const definitions = useCustomerFields(kind, revision);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (accessError(definitions?.error)) onAccessError(definitions?.error);
  }, [definitions?.error, onAccessError]);
  function edit(field?: FieldDefinition) {
    setDraft({ field, label: field?.label ?? '', required: field?.required ?? false });
    setError('');
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    if (!draft || active.current || workspace.role !== 'admin') return;
    const label = draft.label.trim();
    if (!label || Array.from(label).length > 80) { setError('Enter a field label of 1 to 80 characters.'); return; }
    const payload: { label?: string; required?: boolean } = {};
    if (!draft.field || (!draft.field.builtin && label !== draft.field.label)) payload.label = label;
    if (!draft.field || draft.required !== draft.field.required) payload.required = draft.required;
    if (!Object.keys(payload).length) { setDraft(null); return; }
    const controller = new AbortController(); active.current = controller;
    setBusy(true); setError('');
    try {
      const base = '/crm/workspaces/' + workspace.id + '/fields/' + kind;
      await request<FieldDefinition>(draft.field ? base + '/' + encodeURIComponent(draft.field.id) : base, {
        method: draft.field ? 'PATCH' : 'POST', body: JSON.stringify(payload), signal: controller.signal,
      });
      if (!controller.signal.aborted) { setDraft(null); setRevision(n => n + 1); }
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onAccessError(cause);
        else setError(errorMessage(cause));
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  const title = kind === 'companies' ? 'Company fields' : 'Contact fields';
  const rows = definitions?.data ?? [];
  return <RecordDialog title={title} onClose={onClose}><div className="customer-card customer-settings">
    {!definitions ? <p role="status">Loading fields…</p> : definitions.error != null
      ? <p role="alert" className="customer-error">{errorMessage(definitions.error)} <button className="ws-text-button" onClick={() => setRevision(n => n + 1)}>Retry fields</button></p>
      : draft ? <form onSubmit={save} className="customer-form">
        <h3>{draft.field ? 'Edit field' : 'Add text field'}</h3>
        <fieldset className="customer-field-grid" disabled={busy}>
          {draft.field?.builtin ? <strong>{draft.label}</strong> :
            <label className="customer-wide">Field label<input autoFocus required value={draft.label} onChange={e => setDraft({...draft, label:e.target.value})}/></label>}
          <label className="customer-checkbox customer-wide"><input type="checkbox" checked={draft.required} onChange={e => setDraft({...draft, required:e.target.checked})}/>Required</label>
        </fieldset>
        {error && <p className="customer-error" role="alert">{error}</p>}
        <div className="customer-actions"><button className="ws-action" type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save field'}</button>
          <button type="button" className="ws-pill" disabled={busy} onClick={() => setDraft(null)}>Cancel</button></div>
      </form> : <>
        <div className="customer-card-heading"><span>{workspace.name}</span><button className="ws-action" onClick={() => edit()}>Add text field</button></div>
        <ul className="customer-field-list">
          {!rows.some(row => row.id === 'name') && <li><strong>Name</strong><span>Always required</span></li>}
          {rows.map(field => <li key={field.id}><strong>{field.label}</strong><span>{field.id === 'name' ? 'Always required' : field.required ? 'Required' : 'Optional'}</span>
            {field.id !== 'name' && <button className="ws-text-button" aria-label={'Edit ' + field.label} onClick={() => edit(field)}>Edit</button>}</li>)}
        </ul>
      </>}
  </div></RecordDialog>;
}
