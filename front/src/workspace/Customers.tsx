import { useCallback, useEffect, useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { CompanyPicker } from './CompanyPicker';
import { CustomerCard } from './CustomerCard';
import { CustomerForm } from './CustomerForm';
import { CustomerFieldSettings } from './CustomerFieldSettings';
import { accessError, customerLink, errorMessage, singular, useCustomerData, type Customer, type CustomerKind, type CustomerRef, type Member } from './customerData';
import { PageTitle, RecordDialog, Switcher } from './shared';
import './customers.css';

const pageSize = 20;
const emptyFilters = { q: '', tag: '', segment: '', owner_id: '', company_id: '' };
type Filters = typeof emptyFilters;
type Creation = { kind: CustomerKind; company?: Customer; source: string };

export function Customers({ initialRecord }: { initialRecord?: CustomerRef }) {
  const { workspace, profile } = useCrm();
  const { locale, messages: { workspace: { customers: copy } } } = useLocale();
  const [kind, setKind] = useState<CustomerKind>(initialRecord?.kind ?? 'contacts');
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [filterPanel, setFilterPanel] = useState(false);
  const [sort, setSort] = useState('name');
  const [order, setOrder] = useState('asc');
  const [page, setPage] = useState(0);
  const [revision, setRevision] = useState(0);
  const [create, setCreate] = useState<Creation | null>(null);
  const [settings, setSettings] = useState(false);
  const [notice, setNotice] = useState('');
  const refresh = useCallback(() => setRevision(n => n + 1), []);
  const source = initialRecord ? customerLink(initialRecord) : '#/customers';
  const creation = create?.source === source ? create : null;
  useEffect(() => { if (create && create.source !== source) setCreate(null); }, [source, create]);
  const canWrite = workspace.role !== 'support';
  const base = '/crm/workspaces/' + workspace.id;
  const params = new URLSearchParams({sort, order, limit: String(pageSize + 1), offset: String(page * pageSize)});
  Object.entries(filters).forEach(([key, raw]) => {
    const value = key === 'owner_id' ? raw : raw.trim();
    if (value && (key !== 'company_id' || kind === 'contacts')) params.set(key, value);
  });
  const list = useCustomerData<Customer[]>(base + '/' + kind + '?' + params, revision, 160);
  const members = useCustomerData<Member[]>('/auth/workspaces/' + workspace.id + '/members', revision);
  const denied = accessError(list?.error) || accessError(members?.error);
  const people = members?.data ?? [];
  function participant(id: string) {
    return people.find(person => person.user_id === id)?.email || (id === profile.id && profile.email) || id;
  }
  function filter(key: keyof Filters, value: string) {
    setFilters(current => ({...current, [key]: value})); setPage(0);
  }
  function switchKind(next: CustomerKind) {
    setKind(next); setPage(0); setFilters(emptyFilters); setNotice('');
  }
  function begin(kind: CustomerKind, company?: Customer) { setNotice(''); setCreate({kind, company, source}); }
  function closeCard() { refresh(); window.location.hash = '/customers'; }
  const count = [filters.tag, filters.segment, filters.owner_id, kind === 'contacts' ? filters.company_id : ''].filter(x => x.trim()).length;
  const items = list?.data ?? [];
  const visible = items.slice(0, pageSize);
  const label = kind === 'contacts' ? copy.contacts : copy.companies;
  const date = new Intl.DateTimeFormat(locale, { dateStyle: 'medium' });
  if (denied) return <><PageTitle title={label} description={workspace.name}/>
    <p role="alert" className="ws-panel customer-error">This directory is unavailable or your access has changed.{' '}
      <button className="ws-text-button" onClick={() => { setCreate(null); if (initialRecord) closeCard(); refresh(); }}>Retry directory</button></p></>;
  return <>
    <div className="customer-page-head"><PageTitle title={label} description={copy.description}/>
      {canWrite && <div className="customer-actions">{workspace.role === 'admin' && <button className="ws-pill" onClick={() => setSettings(true)}>Manage fields</button>}
        <button className="ws-action" onClick={() => begin(kind)}>New {singular[kind]}</button></div>}</div>
    <div className="ws-toolbar customer-toolbar"><Switcher<CustomerKind> values={[[ 'contacts', copy.contacts ], [ 'companies', copy.companies ]]}
      selected={kind} onSelect={switchKind} label="Customer directory"/>
      <input className="ws-search" type="search" aria-label={'Search ' + kind} placeholder={'Search ' + kind} maxLength={200}
        value={filters.q} onChange={e => filter('q', e.target.value)}/>
      <button className="ws-pill" aria-expanded={filterPanel} aria-controls="customer-filters" onClick={() => setFilterPanel(open => !open)}>
        Filters{count > 0 ? ' (' + count + ')' : ''}</button>
      <label className="customer-sort">Sort <select value={sort} onChange={e => {setSort(e.target.value); setPage(0);}}>
        <option value="name">Name</option><option value="created_at">Created</option><option value="updated_at">Updated</option>
      </select></label>
      <select aria-label="Sort direction" value={order} onChange={e => {setOrder(e.target.value); setPage(0);}}>
        <option value="asc">Ascending</option><option value="desc">Descending</option></select>
      {filterPanel && <div id="customer-filters" className="customer-field-grid customer-filters">
        <label>Tag<input value={filters.tag} maxLength={50} onChange={e => filter('tag', e.target.value)}/></label>
        <label>Segment<input value={filters.segment} maxLength={80} onChange={e => filter('segment', e.target.value)}/></label>
        <label>Owner<select value={filters.owner_id} onChange={e => filter('owner_id', e.target.value)}>
          <option value="">All available owners</option>
          {(workspace.role === 'sales_manager' ? [{user_id:profile.id,email:profile.email}] : people).map(person =>
            <option key={person.user_id} value={person.user_id}>{person.email || person.user_id}</option>)}
        </select></label>
        {kind === 'contacts' && <CompanyPicker label="Filter by company" value={filters.company_id} onChange={id => filter('company_id', id)} onAccessError={refresh}/>}
        <button className="ws-pill" onClick={() => {setFilters(emptyFilters); setPage(0);}}>Clear filters</button>
      </div>}
    </div>
    {notice && <p role="status" className="customer-error">{notice}</p>}
    {members?.error != null && <p role="alert" className="customer-error">Could not load team members. <button className="ws-text-button" onClick={refresh}>Retry</button></p>}
    <section className="ws-panel ws-table-panel"><div className="ws-section-head"><h2>{kind === 'contacts' ? copy.contactDirectory : 'Company directory'}</h2>
      {visible.length > 0 && <small>{(page * pageSize + 1).toLocaleString(locale)}–{(page * pageSize + visible.length).toLocaleString(locale)}</small>}</div>
      {!list ? <p role="status" className="ws-empty">Loading {kind}…</p>
        : list.error != null ? <p role="alert" className="customer-error">{errorMessage(list.error)} <button className="ws-text-button" onClick={refresh}>Retry list</button></p>
        : visible.length === 0 ? <p className="ws-empty" role="status">{page || params.has('q') || count ? 'No matching records.' : 'No ' + kind + ' in this workspace yet.'}</p>
        : <div className="ws-table-scroll"><table className="ws-table customer-table"><thead><tr>
          <th>{kind === 'contacts' ? 'Contact' : 'Company'}</th><th>{kind === 'contacts' ? 'Company' : 'Phone'}</th><th>Segment &amp; tags</th><th>Owner</th><th>Updated</th>
        </tr></thead><tbody>{visible.map(row => <tr key={row.id}>
          <td><a className="ws-text-button" href={customerLink({kind,id:row.id})}>{row.name}</a><small>{row.email || row.position}</small></td>
          <td>{kind === 'companies' ? row.phone || '—' : row.company_id
            ? <a className="customer-relation-link" href={customerLink({kind:'companies',id:row.company_id})}>View company</a> : '—'}</td>
          <td>{row.segment || (row.tags.length ? '' : '—')}{row.tags.length > 0 && <ul className="customer-tags">{row.tags.map(tag => <li key={tag}>{tag}</li>)}</ul>}</td>
          <td>{participant(row.owner_id)}</td><td><time dateTime={row.updated_at}>{date.format(new Date(row.updated_at))}</time></td>
        </tr>)}</tbody></table></div>}
      {(page > 0 || items.length > pageSize) && <nav aria-label="Directory pages" className="customer-pager">
        <button className="ws-pill" disabled={page === 0} onClick={() => setPage(n => n-1)}>Previous</button>
        <span>Page {(page + 1).toLocaleString(locale)}</span>
        <button className="ws-pill" disabled={items.length <= pageSize} onClick={() => setPage(n => n+1)}>Next</button></nav>}
    </section>
    {settings && !initialRecord && workspace.role === 'admin' ? <CustomerFieldSettings kind={kind} onClose={() => setSettings(false)}
      onAccessError={error => { setSettings(false); setNotice(errorMessage(error)); refresh(); }}/>
      : creation && canWrite ? <RecordDialog title={'New ' + singular[creation.kind]} onClose={() => setCreate(null)}>
      <div className="customer-card"><CustomerForm kind={creation.kind} companyID={creation.company?.id} companyName={creation.company?.name}
        onCancel={() => setCreate(null)} onAccessError={error => { setCreate(null); setNotice(errorMessage(error)); refresh(); }}
        onSaved={result => { setCreate(null); refresh(); window.location.hash = customerLink({kind:creation.kind,id:result.id}); }}/></div></RecordDialog>
      : initialRecord && <CustomerCard key={source} selected={initialRecord} participant={participant} onClose={closeCard} onListChanged={refresh}
          onNewContact={company => begin('contacts', company)}/>}
  </>;
}
