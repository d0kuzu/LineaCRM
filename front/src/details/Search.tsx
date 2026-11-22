import { useState } from 'react';
import { useLocale } from '../i18n';
import { formatCount } from '../plurals';
import { deals, meetings } from '../overview/data';
import { calendarActivities } from '../workspace/Calendar';
import { contacts } from '../workspace/Customers';
import { usePipeline } from '../workspace/Sales';
import { dateLabel, money, timeLabel } from '../workspace/format';
import { Fields, PageTitle, RecordDialog } from '../workspace/shared';
import companyIcon from './assets/company.svg';
import contactIcon from './assets/contact.svg';
import dealIcon from './assets/deal.svg';
import meetingIcon from './assets/meeting.svg';
import searchIcon from './assets/search.svg';
import { fill } from './format';

const icons = { company: companyIcon, contact: contactIcon, deal: dealIcon, meeting: meetingIcon };
type Result = {
  type: keyof typeof icons;
  title: string;
  company: string;
  subtitle: string;
  href?: string;
  fields?: [string, string][];
  suggested?: boolean;
};

export function Search() {
  const { messages: { details: copy, overview, workspace }, locale } = useLocale();
  const [query, setQuery] = useState('');
  const [selection, setSelection] = useState<Result | null>(null);
  const pipeline = usePipeline();
  const results: Result[] = [];
  const companies = new Set([...contacts.map(c => c.company), ...pipeline.map(d => d.company)]);
  for (const company of companies) {
    const contact = contacts.find(c => c.company === company);
    const fields: [string, string][] = [[copy.search.company, company]];
    if (contact) fields.push([copy.search.contact, contact.name]);
    results.push({
      type: 'company', company, fields, suggested: company === overview.accounts.orbita.company,
      title: fill(copy.search.companyTitle, { company }),
      subtitle: company === overview.accounts.orbita.company ? copy.contacts.midMarket : company,
    });
  }
  for (const contact of contacts) {
    const fields: [string, string][] = [
      [copy.search.company, contact.company],
      [workspace.customers.segment, workspace.customers.segments[contact.segment]],
      [workspace.general.owner, contact.owner],
    ];
    results.push({
      type: 'contact', title: contact.name, company: contact.company, fields, suggested: contact.name === copy.contacts.aliya,
      subtitle: contact.name === copy.contacts.aliya ? [copy.contacts.aliyaRole, contact.company].join(' · ') : contact.company,
    });
  }
  for (const deal of pipeline) {
    const stage = workspace.sales.stages[deal.stage];
    const amount = money(deal.amount, locale);
    results.push({
      type: 'deal', title: deal.title, company: deal.company, href: deal.href, suggested: deal.id === 'orbita',
      subtitle: [deal.company, stage, amount].join(' · '),
      fields: [[workspace.general.company, deal.company], [workspace.general.stage, stage], [workspace.general.amount, amount], [workspace.general.nextStep, deal.activity]],
    });
  }
  for (const meeting of meetings) {
    const account = overview.accounts[meeting.id];
    results.push({
      type: 'meeting', title: [account.company, copy.meeting.names[meeting.id]].join(' · '), company: account.company,
      subtitle: fill(copy.meeting.shortTime, { time: timeLabel(meeting.startsAt, locale) }), href: '#/meeting/' + meeting.id,
      suggested: meeting.id === 'orbita',
    });
  }
  for (const [start, kind, organization] of calendarActivities) {
    const company = organization ?? workspace.calendar.team;
    const date = dateLabel(start, locale, { month: 'short', day: 'numeric' });
    const time = timeLabel(start, locale);
    results.push({
      type: 'meeting', title: [company, workspace.calendar.names[kind]].join(' · '), company,
      subtitle: [date, time].join(' · '),
      fields: [[workspace.general.company, company], [workspace.general.date, date], [workspace.general.time, time]],
    });
  }
  const normalize = (text: string) => text.toLocaleLowerCase(locale).normalize('NFKD').replace(/\p{M}/gu, '');
  const words = normalize(query.trim()).split(/\s+/);
  const filtered = query.trim() ? results.filter(result => {
    const haystack = normalize([result.title, result.company, result.subtitle, copy.search[result.type]].join(' '));
    return words.every(word => haystack.includes(word));
  }) : results.filter(result => result.suggested);
  const relatedDeal = selection?.type !== 'deal' && deals.find(d => overview.accounts[d.id].company === selection?.company);

  return <>
    <PageTitle title={copy.search.title} description={copy.search.subtitle} />
    <label className="detail-search-box">
      <img src={searchIcon} alt="" />
      <input type="search" id="crm-search-input" autoFocus value={query} onChange={event => setQuery(event.target.value)}
        aria-label={copy.search.title} placeholder={copy.search.placeholder} />
    </label>
    <section className="detail-card detail-results">
      <h2>{query.trim() ? copy.search.results : copy.search.suggested}</h2>
      <p className="detail-muted" role="status">{formatCount(copy.search.resultCount, filtered.length, locale)}</p>
      <ul className="detail-results-list">{filtered.map(result => {
        const content = <><img src={icons[result.type]} alt="" /><span><strong>{result.title}</strong>
          <small>{[copy.search[result.type], result.subtitle].join(' · ')}</small></span></>;
        return <li key={[result.type, result.company, result.title].join('/')}>{result.href
          ? <a href={result.href}>{content}</a>
          : <button type="button" onClick={() => setSelection(result)}>{content}</button>}</li>;
      })}</ul>
      {!filtered.length && <p>{copy.search.empty}</p>}
    </section>
    {selection && <RecordDialog title={selection.title} onClose={() => setSelection(null)}>
      <Fields rows={selection.fields ?? []} />
      {relatedDeal && <a className="ws-action" href={'#/deal/' + relatedDeal.id}>{copy.search.relatedDeal}</a>}
    </RecordDialog>}
  </>;
}
