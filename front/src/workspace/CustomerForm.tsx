import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { CompanyPicker } from './CompanyPicker';
import { accessError, errorMessage, singular, useCustomerFields, type Customer, type CustomerKind } from './customerData';

function FieldLabel({ text, required }: { text: string; required: boolean }) {
  return <span>{text}{required && <span aria-hidden="true"> *</span>}</span>;
}

export function CustomerForm({ kind, record, companyName, companyID, onSaved, onCancel, onAccessError }: {
  kind: CustomerKind; record?: Customer; companyName?: string; companyID?: string;
  onSaved: (result: Customer) => void; onCancel: () => void; onAccessError: (error: unknown) => void;
}) {
  const { request, workspace } = useCrm();
  const [name, setName] = useState(record?.name ?? '');
  const [email, setEmail] = useState(record?.email ?? '');
  const [phone, setPhone] = useState(record?.phone ?? '');
  const [segment, setSegment] = useState(record?.segment ?? '');
  const [position, setPosition] = useState(record?.position ?? '');
  const [company, setCompany] = useState(record?.company_id ?? companyID ?? '');
  const [tags, setTags] = useState(record?.tags ?? []);
  const [draftTag, setDraftTag] = useState('');
  const [custom, setCustom] = useState<Record<string,string>>(() => ({...record?.custom_fields}));
  const [revision, setRevision] = useState(0);
  const definitions = useCustomerFields(kind, revision);
  const required = (id: string) => definitions?.data?.some(f => f.id === id && f.required) ?? false;
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (accessError(definitions?.error)) onAccessError(definitions?.error);
  }, [definitions?.error, onAccessError]);

  function withDraftTag() {
    const text = draftTag.trim();
    if (!text || tags.some(tag => tag.toLowerCase() === text.toLowerCase())) return tags;
    if (Array.from(text).length > 50 || tags.length >= 20) {
      setError('Use up to 20 tags of 50 characters each.');
      return null;
    }
    return [...tags, text];
  }
  function addTag() {
    const next = withDraftTag();
    if (next) { setTags(next); setDraftTag(''); setError(''); }
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    if (active.current || !definitions?.data) return;
    if (!name.trim()) { setError('Enter a name.'); return; }
    const finalTags = withDraftTag();
    if (!finalTags) return;
    const fields: Record<string, string | string[] | null | Record<string,string | null>> = {};
    const textFields = { name, email, phone, segment, ...(kind === 'contacts' ? { position } : {}) };
    const builtinValues: Record<string, string | string[]> = {...textFields, company_id:company, tags:finalTags};
    const changedCustom: Record<string,string | null> = {};
    for (const def of definitions.data) {
      const value = def.builtin ? builtinValues[def.id] : custom[def.id];
      const present = Array.isArray(value) ? value.length > 0 : Boolean(value?.trim());
      if (def.required && !present) { setError(def.label + ' is required.'); return; }
      if (!def.builtin) {
        const text = (custom[def.id] ?? '').trim();
        if (Array.from(text).length > 2000) { setError(def.label + ' must be 2,000 characters or fewer.'); return; }
        if (text !== (record?.custom_fields?.[def.id] ?? '')) changedCustom[def.id] = text || null;
      }
    }
    if (Object.keys(changedCustom).length) fields.custom_fields = changedCustom;
    for (const [key, raw] of Object.entries(textFields)) {
      const value = raw.trim();
      if (!record || value !== record[key as keyof Customer]) fields[key] = value;
    }
    if (!record || JSON.stringify(finalTags) !== JSON.stringify(record.tags)) fields.tags = finalTags;
    if (kind === 'contacts' && (!record || company !== (record.company_id ?? ''))) fields.company_id = company || null;
    if (!Object.keys(fields).length) { onCancel(); return; }
    const controller = new AbortController();
    active.current = controller;
    setBusy(true); setError('');
    try {
      const base = '/crm/workspaces/' + workspace.id + '/' + kind;
      const result = await request<Customer>(record ? base + '/' + record.id : base, {
        method: record ? 'PATCH' : 'POST', body: JSON.stringify(fields), signal: controller.signal,
      });
      if (!controller.signal.aborted) onSaved(result);
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
  return <form className="customer-form" onSubmit={save}>
    {!definitions && <p role="status">Loading fields…</p>}
    {definitions?.error != null && <p role="alert" className="customer-error">{errorMessage(definitions.error)} <button type="button" className="ws-text-button" onClick={() => setRevision(n => n + 1)}>Retry fields</button></p>}
    <fieldset disabled={busy || !definitions?.data} className="customer-field-grid">
      <label className="customer-wide"><FieldLabel text="Name" required/><input aria-label="Name" required autoFocus value={name} maxLength={200} onChange={e => setName(e.target.value)}/></label>
      <label><FieldLabel text="Email" required={required('email')}/><input aria-label="Email" type="email" required={required('email')} value={email} maxLength={254} onChange={e => setEmail(e.target.value)}/></label>
      <label><FieldLabel text="Phone" required={required('phone')}/><input aria-label="Phone" type="tel" required={required('phone')} value={phone} maxLength={64} onChange={e => setPhone(e.target.value)}/></label>
      <label><FieldLabel text="Segment" required={required('segment')}/><input aria-label="Segment" required={required('segment')} value={segment} maxLength={80} onChange={e => setSegment(e.target.value)}/></label>
      {kind === 'contacts' && <><label><FieldLabel text="Position" required={required('position')}/><input aria-label="Position" required={required('position')} value={position} maxLength={200} onChange={e => setPosition(e.target.value)}/></label>
        <CompanyPicker value={company} required={required('company_id')} currentName={companyName} onChange={setCompany} onAccessError={onAccessError}/></>}
      <fieldset className="customer-tag-editor customer-wide"><legend>Tags{required('tags') && <span aria-hidden="true"> *</span>}</legend>
        {tags.length > 0 && <ul>{tags.map((tag, i) => <li key={tag}><span>{tag}</span><button type="button" aria-label={'Remove tag ' + tag}
          onClick={() => setTags(items => items.filter((_, index) => index !== i))}>×</button></li>)}</ul>}
        <div><input aria-label="New tag" required={required('tags') && tags.length === 0} value={draftTag} placeholder="New tag" onChange={e => setDraftTag(e.target.value)}
          onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); addTag(); } }}/>
          <button type="button" className="ws-pill" disabled={!draftTag.trim()} onClick={addTag}>Add tag</button></div>
      </fieldset>
      {definitions?.data?.filter(def => !def.builtin).map(def =>
        <label key={def.id} className="customer-wide"><FieldLabel text={def.label} required={def.required}/><textarea rows={2} aria-label={def.label} required={def.required}
          value={custom[def.id] ?? ''} onChange={e => { const value = e.target.value; setCustom(current => ({...current, [def.id]:value})); }}/></label>)}
    </fieldset>
    {error && <p role="alert" className="customer-error">{error}</p>}
    <div className="customer-actions"><button type="submit" className="ws-action" disabled={busy || !definitions?.data}>
      {busy ? 'Saving…' : record ? 'Save changes' : 'Create ' + singular[kind]}</button>
      <button type="button" className="ws-pill" onClick={onCancel}>Cancel</button>
      <button className="ws-text-button" type="button" disabled={busy || !definitions?.data}
        onClick={() => { setRevision(n => n + 1); setError(''); }}>Refresh field requirements</button></div>
  </form>;
}
