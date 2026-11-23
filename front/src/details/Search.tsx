import { useState, useEffect } from 'react';
import { useLocale } from '../i18n';
import { formatCount } from '../plurals';
import { deals, meetings } from '../overview/data';
import { calendarActivities } from '../workspace/Calendar';
import { useCrm } from '../crm/session';
import { errorText } from '../crm/http';
import { useDemoOpportunities } from './demoOpportunities';
import { salesLink, type Deal, type Lead } from '../workspace/salesData';
import { dateLabel, money, timeLabel } from '../workspace/format';
import { Fields, PageTitle, RecordDialog } from '../workspace/shared';
import companyIcon from './assets/company.svg';
import contactIcon from './assets/contact.svg';
import dealIcon from './assets/deal.svg';
import meetingIcon from './assets/meeting.svg';
import searchIcon from './assets/search.svg';
import { fill } from './format';

const icons = { company: companyIcon, contact: contactIcon, lead: dealIcon, deal: dealIcon, meeting: meetingIcon };
type SearchCard = { id: string; name: string; segment: string; email: string; position?: string };
type Result = {
  type: keyof typeof icons;
  title: string;
  company: string;
  subtitle: string;
  href?: string;
  fields?: [string, string][];
  suggested?: boolean;
  demo?: boolean;
};

export function Search() {
  const { messages: { details: copy, overview, workspace }, locale } = useLocale();
  const [query, setQuery] = useState('');
  const [selection, setSelection] = useState<Result | null>(null);
  const pipeline = useDemoOpportunities();
  const results: Result[] = [];
  const { workspace: team, request } = useCrm();
  const term = query.trim();
  const [live, setLive] = useState<{ term: string; results: Result[]; error?: string } | null>(null);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    setLive(null);
    async function findRecords() {
      const params = new URLSearchParams({ q: term, limit: term ? '25' : '4' });
      const path = '/crm/workspaces/' + team.id + '/';
      try {
        const options = { signal: abort.signal };
        const [companies, people, leads, liveDeals] = await Promise.all([
          request<SearchCard[]>(path + 'companies?' + params, options),
          request<SearchCard[]>(path + 'contacts?' + params, options),
          request<Lead[]>(path + 'leads?' + params, options),
          request<Deal[]>(path + 'deals?' + params, options),
        ]);
        const hits: Result[] = [
          ...companies.map(record => ({
            type: 'company' as const, title: record.name, company: record.name,
            subtitle: record.segment || record.email, href: '#/customers/companies/' + record.id,
          })),
          ...people.map(record => ({
            type: 'contact' as const, title: record.name, company: '', subtitle: record.position || record.email,
            href: '#/customers/contacts/' + record.id,
          })),
          ...leads.map(lead => ({
            type: 'lead' as const, title: lead.title, company: '', subtitle: lead.converted ? 'Converted' : 'Open',
            href: salesLink('leads', lead.id),
          })),
          ...liveDeals.map(deal => ({
            type: 'deal' as const, title: deal.title, company: '', subtitle: '', href: salesLink('deals', deal.id),
          })),
        ];
        if (!abort.signal.aborted) setLive({ term, results: hits });
      } catch (err) {
        if (!abort.signal.aborted) setLive({ term, results: [], error: errorText(err) });
      }
    }
    const timer = window.setTimeout(() => void findRecords(), 180);
    return () => { clearTimeout(timer); abort.abort(); };
  }, [term, team.id, request, attempt]);
  for (const deal of pipeline) {
    const stage = workspace.sales.stages[deal.stage];
    const amount = money(deal.amount, locale);
    results.push({
      type: 'deal', demo: true, title: deal.title, company: deal.company, href: deal.href, suggested: deal.id === 'orbita',
      subtitle: [deal.company, stage, amount].join(' · '),
      fields: [[workspace.general.company, deal.company], [workspace.general.stage, stage], [workspace.general.amount, amount], [workspace.general.nextStep, deal.activity]],
    });
  }
  for (const meeting of meetings) {
    const account = overview.accounts[meeting.id];
    results.push({
      type: 'meeting', demo: true, title: [account.company, copy.meeting.names[meeting.id]].join(' · '), company: account.company,
      subtitle: fill(copy.meeting.shortTime, { time: timeLabel(meeting.startsAt, locale) }), href: '#/meeting/' + meeting.id,
      suggested: meeting.id === 'orbita',
    });
  }
  for (const [start, kind, organization] of calendarActivities) {
    const company = organization ?? workspace.calendar.team;
    const date = dateLabel(start, locale, { month: 'short', day: 'numeric' });
    const time = timeLabel(start, locale);
    results.push({
      type: 'meeting', demo: true, title: [company, workspace.calendar.names[kind]].join(' · '), company,
      subtitle: [date, time].join(' · '),
      fields: [[workspace.general.company, company], [workspace.general.date, date], [workspace.general.time, time]],
    });
  }
  const normalize = (text: string) => text.toLocaleLowerCase(locale).normalize('NFKD').replace(/\p{M}/gu, '');
  const words = normalize(query.trim()).split(/\s+/);
  const demoResults = query.trim() ? results.filter(result => {
    const haystack = normalize([result.title, result.company, result.subtitle, copy.search[result.type]].join(' '));
    return words.every(word => haystack.includes(word));
  }) : results.filter(result => result.suggested);
  const found = live?.term === term ? live : null;
  const filtered = [...(found?.results ?? []), ...demoResults];
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
      {found?.error ? <p role="alert">{found.error} <button onClick={() => setAttempt(n => n + 1)}>Try again</button></p>
        : <p className="detail-muted" role="status">{found ? formatCount(copy.search.resultCount, filtered.length, locale) : 'Searching…'}</p>}
      <ul className="detail-results-list">{filtered.map(result => {
        const content = <><img src={icons[result.type]} alt="" /><span><strong>{result.title}</strong>
          <small>{[copy.search[result.type], result.subtitle, result.demo ? 'Demo' : ''].filter(Boolean).join(' · ')}</small></span></>;
        return <li key={result.href ?? [result.type, result.company, result.title].join('/')}>{result.href
          ? <a href={result.href}>{content}</a>
          : <button type="button" onClick={() => setSelection(result)}>{content}</button>}</li>;
      })}</ul>
      {found && !found.error && !filtered.length && <p>{copy.search.empty}</p>}
    </section>
    {selection && <RecordDialog title={selection.title} onClose={() => setSelection(null)}>
      <Fields rows={selection.fields ?? []} />
      {relatedDeal && <a className="ws-action" href={'#/deal/' + relatedDeal.id}>{copy.search.relatedDeal}</a>}
    </RecordDialog>}
  </>;
}
