import { useCallback, useEffect, useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { CustomerForm } from './CustomerForm';
import { CustomerHistory } from './CustomerHistory';
import { Fields, RecordDialog } from './shared';
import { accessError, customerLink, errorMessage, singular, timestamp, useCustomerData, type Customer, type CustomerRef } from './customerData';

function CompanyContacts({ path, onAccessError }: { path: string; onAccessError: (error: unknown) => void }) {
  const [offset, setOffset] = useState(0);
  const [revision, setRevision] = useState(0);
  const result = useCustomerData<Customer[]>(path + '&limit=11&offset=' + offset, revision);
  useEffect(() => {
    if (accessError(result?.error)) onAccessError(result?.error);
  }, [result?.error, onAccessError]);
  if (!result) return <p role="status">Loading contacts…</p>;
  if (result.error != null) return <p className="customer-error" role="alert">{errorMessage(result.error)} <button className="ws-text-button"
    onClick={() => setRevision(n => n + 1)}>Retry contacts</button></p>;
  const rows = result.data ?? [];
  return <>
    {rows.length ? <ul className="customer-linked">{rows.slice(0, 10).map(contact => <li key={contact.id}>
      <a className="ws-text-button" href={customerLink({kind: 'contacts', id: contact.id})}>{contact.name}</a>
      <small>{contact.position || contact.email}</small></li>)}</ul> : <p className="ws-subtext">No linked contacts.</p>}
    {(offset > 0 || rows.length > 10) && <div className="customer-pager">
      <button disabled={!offset} onClick={() => setOffset(n => n - 10)}>Previous contacts</button>
      <button disabled={rows.length <= 10} onClick={() => setOffset(n => n + 10)}>More contacts</button>
    </div>}
  </>;
}

export function CustomerCard({ selected, onClose, onListChanged, onNewContact, participant }: {
  selected: CustomerRef; onClose: () => void; onListChanged: () => void;
  onNewContact: (company: Customer) => void; participant: (id: string) => string;
}) {
  const { workspace } = useCrm();
  const { locale } = useLocale();
  const [revision, setRevision] = useState(0);
  const [editing, setEditing] = useState(false);
  const [denied, setDenied] = useState(false);
  const base = '/crm/workspaces/' + workspace.id;
  const path = base + '/' + selected.kind + '/' + selected.id;
  const result = useCustomerData<Customer>(denied ? null : path, revision);
  const record = result?.data;
  const company = useCustomerData<Customer>(!denied && record?.company_id ? base + '/companies/' + record.company_id : null, revision);
  const canWrite = workspace.role !== 'support';
  const onAccessError = useCallback(() => {
    setEditing(false); setDenied(true); onListChanged();
  }, [onListChanged]);
  const lostAccess = accessError(result?.error) || accessError(company?.error);
  useEffect(() => { if (lostAccess) onAccessError(); }, [lostAccess, onAccessError]);
  function retry() { setDenied(false); setRevision(n => n + 1); }
  const unavailable = denied || lostAccess || result?.error != null;
  const title = !unavailable && record ? (editing ? 'Edit ' + singular[selected.kind] : record.name) : (selected.kind === 'companies' ? 'Company' : 'Contact');
  const rows: [string, string][] = record ? [
    ['Email', record.email || '—'], ['Phone', record.phone || '—'],
    ...(selected.kind === 'contacts' ? [['Position', record.position || '—'] as [string,string]] : []),
    ['Segment', record.segment || '—'], ['Owner', participant(record.owner_id)],
    ['Created', timestamp(record.created_at, locale)], ['Updated', timestamp(record.updated_at, locale)],
  ] : [];
  return <RecordDialog title={title} onClose={onClose}><div className="customer-card">
    {unavailable ? <p role="alert" className="customer-error">{denied || lostAccess ? 'This record is unavailable or your access has changed.' : errorMessage(result?.error)}
      {' '}<button onClick={retry} className="ws-text-button">Retry</button></p>
      : !record ? <p role="status">Loading record…</p>
      : editing && canWrite ? <CustomerForm kind={selected.kind} record={record} companyName={company?.data?.name}
        onCancel={() => setEditing(false)} onAccessError={onAccessError}
        onSaved={() => { setEditing(false); setRevision(n => n + 1); onListChanged(); }}/>
      : <>
        <div className="customer-card-heading"><span className="ws-kicker">{singular[selected.kind]}</span>
          {canWrite && <button className="ws-pill" onClick={() => setEditing(true)}>Edit {singular[selected.kind]}</button>}</div>
        <Fields rows={rows}/>
        {record.tags?.length > 0 && <ul className="customer-tags" aria-label="Tags">{record.tags.map(tag => <li key={tag}>{tag}</li>)}</ul>}
        {selected.kind === 'contacts' && <section><h3>Company</h3>
          {!record.company_id ? <p className="ws-subtext">No linked company.</p>
            : company?.error != null ? <p role="alert" className="customer-error">{errorMessage(company.error)} <button className="ws-text-button" onClick={retry}>Retry</button></p>
            : !company?.data ? <p role="status">Loading company…</p>
            : <a className="ws-text-button" href={customerLink({kind:'companies',id:company.data.id})}>{company.data.name}</a>}
        </section>}
        {selected.kind === 'companies' && <section><div className="ws-section-head"><h3>Contacts</h3>
          {canWrite && <button className="ws-text-button" onClick={() => onNewContact(record)}>Add contact</button>}</div>
          <CompanyContacts path={base + '/contacts?company_id=' + record.id} onAccessError={onAccessError}/>
        </section>}
        <CustomerHistory key={revision} recordPath={path} canWrite={canWrite} participant={participant}
          linkedCompany={company?.data} onAccessError={onAccessError}/>
      </>}
  </div></RecordDialog>;
}
