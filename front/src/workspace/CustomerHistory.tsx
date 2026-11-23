import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { accessError, errorMessage, timestamp, type FieldDefinition } from './customerData';

const eventNames: Record<string,string> = {
  created: 'Record created', updated: 'Record updated', note: 'Note', email: 'Email', call: 'Call', meeting: 'Meeting',
};
const fieldNames: Record<string,string> = {
  name: 'Name', phone: 'Phone', email: 'Email', tags: 'Tags', segment: 'Segment',
  company_id: 'Company', owner_id: 'Owner', position: 'Position',
};
interface Entry {
  id: string; kind: string; actor_id: string; created_at: string; body?: string;
  changes?: Record<string, { before: unknown; after: unknown; label?: string }> | null;
}
interface HistoryPage { items: Entry[]; next_cursor: string | null }
function filled(value: unknown) {
  return value != null && value !== '' && (!Array.isArray(value) || value.length > 0);
}

export function CustomerHistory({ recordPath, canWrite, participant, linkedCompany, fieldDefinitions, onAccessError }: {
  recordPath: string; canWrite: boolean; participant: (id: string) => string; onAccessError: (error: unknown) => void;
  linkedCompany?: { id: string; name: string };
  fieldDefinitions: FieldDefinition[];
}) {
  const { request } = useCrm();
  const { locale } = useLocale();
  const [items, setItems] = useState<Entry[] | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [interaction, setInteraction] = useState('note');
  const [body, setBody] = useState('');
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);
  const bodyLength = Array.from(body.trim()).length;
  const read = useRef<AbortController | null>(null);
  const write = useRef<AbortController | null>(null);
  function labelFor(field: string, recordedLabel?: string) {
    if (!field.startsWith('custom:')) return fieldNames[field] ?? field;
    return recordedLabel || fieldDefinitions.find(def => def.id === field.slice(7))?.label || 'Custom field';
  }
  function displayValue(field: string, value: unknown): string {
    if (!filled(value)) return 'Empty';
    if (field === 'owner_id' && typeof value === 'string') return participant(value);
    if (field === 'company_id' && linkedCompany && linkedCompany.id === value) return linkedCompany.name;
    if (Array.isArray(value)) return value.join(', ');
    return typeof value === 'object' ? JSON.stringify(value) : String(value);
  }

  const load = useCallback(async (before?: string) => {
    read.current?.abort();
    const controller = new AbortController();
    read.current = controller;
    setLoading(true); setError('');
    if (!before) { setItems(null); setCursor(null); }
    try {
      const params = new URLSearchParams({ limit: '50' });
      if (before) params.set('before', before);
      const page = await request<HistoryPage>(recordPath + '/history?' + params, { signal: controller.signal });
      if (controller.signal.aborted) return;
      setItems(previous => {
        if (!before || !previous) return page.items;
        const seen = new Set(previous.map(entry => entry.id));
        return previous.concat(page.items.filter(entry => !seen.has(entry.id)));
      });
      setCursor(page.next_cursor);
    } catch (cause) {
      if (controller.signal.aborted) return;
      setItems(null); setCursor(null); setError(errorMessage(cause));
      if (accessError(cause)) onAccessError(cause);
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [recordPath, request, onAccessError]);
  useEffect(() => {
    void load();
    return () => { read.current?.abort(); write.current?.abort(); };
  }, [load]);

  async function add(event: FormEvent) {
    event.preventDefault();
    if (!bodyLength || bodyLength > 4000 || write.current) return;
    const controller = new AbortController(); write.current = controller;
    setSaving(true); setSaveError('');
    try {
      await request(recordPath + '/interactions', { method: 'POST', body: JSON.stringify({kind: interaction, body: body.trim()}), signal: controller.signal });
      if (!controller.signal.aborted) { setBody(''); void load(); }
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onAccessError(cause);
        else setSaveError(errorMessage(cause));
      }
    } finally {
      write.current = null;
      if (!controller.signal.aborted) setSaving(false);
    }
  }
  return <section className="customer-history"><h3>History</h3>
    {canWrite && <form onSubmit={add} className="customer-interaction">
      <label>Interaction<select value={interaction} onChange={event => setInteraction(event.target.value)} disabled={saving}>
        {['note', 'call', 'meeting', 'email'].map(kind => <option key={kind} value={kind}>{eventNames[kind]}</option>)}
      </select></label>
      <label>Details<textarea value={body} onChange={event => setBody(event.target.value)} required rows={2} disabled={saving}/></label>
      {bodyLength > 4000 && <p role="alert" className="customer-error">Details must be 4,000 characters or fewer.</p>}
      <button type="submit" className="ws-pill" disabled={!bodyLength || bodyLength > 4000 || saving}>{saving ? 'Saving…' : 'Add to history'}</button>
      {saveError && <p role="alert" className="customer-error">{saveError}</p>}
    </form>}
    {error && <p role="alert" className="customer-error">{error} <button className="ws-text-button" onClick={() => void load()}>Retry history</button></p>}
    {items && <ol className="customer-timeline">{items.map(entry => <li key={entry.id}>
      <strong>{eventNames[entry.kind] ?? entry.kind}</strong>
      <small>{participant(entry.actor_id)} · <time dateTime={entry.created_at}>{timestamp(entry.created_at, locale)}</time></small>
      {entry.body && <p>{entry.body}</p>}
      {Object.entries(entry.changes ?? {})
        .filter(([, change]) => entry.kind !== 'created' || filled(change.after))
        .map(([field, change]) => <p key={field} className="customer-change">
          <b>{labelFor(field, change.label)}</b> {entry.kind !== 'created' && <>{displayValue(field, change.before)} → </>}{displayValue(field, change.after)}</p>)}
    </li>)}</ol>}
    {items?.length === 0 && <p className="ws-subtext">No history yet.</p>}
    {loading && <p role="status" className="ws-subtext">Loading history…</p>}
    {cursor && <button type="button" className="ws-pill" disabled={loading} onClick={() => void load(cursor)}>Load older events</button>}
  </section>;
}
