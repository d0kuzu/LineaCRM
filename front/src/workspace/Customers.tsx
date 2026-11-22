import { useState } from 'react';
import { useLocale } from '../i18n';
import { dateLabel, timeLabel } from './format';
import { Fields, PageTitle, RecordDialog, Switcher } from './shared';

export const contacts = [
  { name: 'Aliya Kim', company: 'Orbita', segment: 'key', owner: 'Maria Orlova', last: '2026-11-04T09:40:00Z' },
  { name: 'Timur Akhmetov', company: 'Terra Logistics', segment: 'new', owner: 'Ilya Sokolov', last: '2026-11-03T16:15:00Z' },
  { name: 'Dina Aliyeva', company: 'Qala Studio', segment: 'mid', owner: 'Maria Orlova', last: '2026-11-02T11:20:00Z' },
  { name: 'Sergey Volkov', company: 'Nord Service', segment: 'key', owner: 'Ilya Sokolov', last: '2026-11-02T09:10:00Z' },
  { name: 'Aizhan Sadykova', company: 'Alma Group', segment: 'partners', owner: 'Maria Orlova', last: '2026-10-30T14:30:00Z' },
  { name: 'Ivan Petrov', company: 'Mayak', segment: 'new', owner: 'Ilya Sokolov', last: '2026-10-29T13:45:00Z' },
] as const;
type Customer = typeof contacts[number];

export function Customers() {
  const { messages: { workspace: { customers: copy, general } }, locale } = useLocale();
  const [view, setView] = useState<'contacts' | 'companies'>('contacts');
  const [segment, setSegment] = useState('');
  const [query, setQuery] = useState('');
  const [record, setRecord] = useState<Customer | null>(null);
  const companyFirst = view === 'companies';
  const columns = companyFirst
    ? [general.company, general.contact, copy.segment, general.owner, copy.lastContact]
    : [general.contact, general.company, copy.segment, general.owner, copy.lastContact];
  const rows = contacts.filter(contact =>
    (!segment || contact.segment === segment) &&
    [contact.name, contact.company, contact.owner].join(' ').toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  const cells = (contact: Customer) => [
    ...(companyFirst ? [contact.company, contact.name] : [contact.name, contact.company]),
    copy.segments[contact.segment], contact.owner,
    [dateLabel(contact.last, locale, { month: 'short', day: 'numeric' }), timeLabel(contact.last, locale)].join(', '),
  ];
  const stats = [[copy.allContacts, 128], [copy.keyAccounts, 24], [copy.noNextStep, 8], [copy.duplicates, 3]] as const;
  return <>
    <PageTitle title={companyFirst ? copy.companies : copy.title} description={copy.description}/>
    <div className="ws-toolbar">
      <Switcher values={[[ 'contacts', copy.contacts ], [ 'companies', copy.companies ]]} selected={view} onSelect={setView} label={copy.title}/>
      <select aria-label={copy.segment} value={segment} onChange={event => setSegment(event.target.value)}>
        <option value="">{copy.allSegments}</option>
        {Object.entries(copy.segments).map(([id, text]) => <option value={id} key={id}>{text}</option>)}
      </select>
    </div>
    <div className="ws-stats">{stats.map(([name, value], index) => <article className="ws-stat" key={name}>
      <span className="ws-kicker">{name}</span><strong>{value.toLocaleString(locale)}</strong><small>{copy.summaryNotes[index]}</small>
    </article>)}</div>
    <section className="ws-panel ws-table-panel">
      <div className="ws-section-head"><h2>{companyFirst ? general.companyDirectory : copy.contactDirectory}</h2>
        <input type="search" className="ws-search" aria-label={general.searchContacts} placeholder={general.searchContacts} value={query} onChange={e => setQuery(e.target.value)}/></div>
      <div className="ws-table-scroll"><table className="ws-table"><thead><tr>{columns.map(title => <th key={title}>{title}</th>)}</tr></thead>
        <tbody>{rows.map(contact => { const [first, ...rest] = cells(contact); return <tr key={contact.name}>
          <td><button className="ws-text-button" onClick={() => setRecord(contact)}>{first}</button></td>
          {rest.map((text, index) => <td key={index}>{text}</td>)}</tr>; })}</tbody>
      </table></div>
      {!rows.length && <p className="ws-empty">{general.noResults}</p>}
    </section>
    {record && <RecordDialog title={companyFirst ? record.company : record.name} onClose={() => setRecord(null)}>
      <Fields rows={cells(record).map((value, index) => [columns[index], value])}/>
      {(record.company === 'Orbita' || record.company === 'Mayak') && <a className="ws-action" href={'#/deal/' + record.company.toLowerCase()}>{general.openDeal}</a>}
    </RecordDialog>}
  </>;
}
