import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { CompanyPicker } from './CompanyPicker';
import { accessError, errorMessage, singular, type Customer, type CustomerKind } from './customerData';

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
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);

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
    if (active.current) return;
    if (!name.trim()) { setError('Enter a name.'); return; }
    const finalTags = withDraftTag();
    if (!finalTags) return;
    const fields: Record<string, string | string[] | null> = {};
    const textFields = { name, email, phone, segment, ...(kind === 'contacts' ? { position } : {}) };
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
    <fieldset disabled={busy} className="customer-field-grid">
      <label className="customer-wide">Name<input required autoFocus value={name} maxLength={200} onChange={e => setName(e.target.value)}/></label>
      <label>Email<input type="email" value={email} maxLength={254} onChange={e => setEmail(e.target.value)}/></label>
      <label>Phone<input type="tel" value={phone} maxLength={64} onChange={e => setPhone(e.target.value)}/></label>
      <label>Segment<input value={segment} maxLength={80} onChange={e => setSegment(e.target.value)}/></label>
      {kind === 'contacts' && <><label>Position<input value={position} maxLength={200} onChange={e => setPosition(e.target.value)}/></label>
        <CompanyPicker value={company} currentName={companyName} onChange={setCompany} onAccessError={onAccessError}/></>}
      <fieldset className="customer-tag-editor customer-wide"><legend>Tags</legend>
        {tags.length > 0 && <ul>{tags.map((tag, i) => <li key={tag}><span>{tag}</span><button type="button" aria-label={'Remove tag ' + tag}
          onClick={() => setTags(items => items.filter((_, index) => index !== i))}>×</button></li>)}</ul>}
        <div><input aria-label="New tag" value={draftTag} placeholder="New tag" onChange={e => setDraftTag(e.target.value)}
          onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); addTag(); } }}/>
          <button type="button" className="ws-pill" disabled={!draftTag.trim()} onClick={addTag}>Add tag</button></div>
      </fieldset>
    </fieldset>
    {error && <p role="alert" className="customer-error">{error}</p>}
    <div className="customer-actions"><button type="submit" className="ws-action" disabled={busy}>
      {busy ? 'Saving…' : record ? 'Save changes' : 'Create ' + singular[kind]}</button>
      <button type="button" className="ws-pill" onClick={onCancel}>Cancel</button></div>
  </form>;
}
