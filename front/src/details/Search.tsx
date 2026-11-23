import { useEffect, useState } from 'react';
import { useLocale } from '../i18n';
import { formatCount } from '../plurals';
import { useCrm } from '../crm/session';
import { errorText } from '../crm/http';
import { activityHref, useActivityRevision, type CrmMeeting, type CrmTask } from '../workspace/activities';
import { customerLink, timestamp, type Customer } from '../workspace/customerData';
import { salesLink, type Deal, type Lead } from '../workspace/salesData';
import { PageTitle } from '../workspace/shared';
import companyIcon from './assets/company.svg';
import contactIcon from './assets/contact.svg';
import dealIcon from './assets/deal.svg';
import meetingIcon from './assets/meeting.svg';
import searchIcon from './assets/search.svg';

const icons = {
  company: companyIcon,
  contact: contactIcon,
  lead: dealIcon,
  deal: dealIcon,
  task: '/icons/nav-tasks.svg',
  meeting: meetingIcon,
};
type Result = { type: keyof typeof icons; title: string; subtitle: string; href: string };

export function Search() {
  const {
    messages: {
      details: { search: copy },
    },
    locale,
  } = useLocale();
  const { workspace, request } = useCrm();
  const [query, setQuery] = useState('');
  const [revision, reload] = useActivityRevision();
  const [response, setResponse] = useState<{ key: string; rows: Result[]; error?: string } | null>(null);
  const term = query.trim();
  const key = JSON.stringify([workspace.id, term, revision]);

  useEffect(() => {
    const controller = new AbortController();
    async function findRecords() {
      const params = new URLSearchParams({ q: term, limit: term ? '25' : '4' });
      const path = '/crm/workspaces/' + workspace.id + '/';
      const list = <T,>(kind: string) =>
        request<T[]>(path + kind + '?' + params, { signal: controller.signal });
      try {
        const [companies, contacts, leads, deals, tasks, meetings] = await Promise.all([
          list<Customer>('companies'),
          list<Customer>('contacts'),
          list<Lead>('leads'),
          list<Deal>('deals'),
          list<CrmTask>('tasks'),
          list<CrmMeeting>('meetings'),
        ]);
        const rows: Result[] = [
          ...companies.map((row) => ({
            type: 'company' as const,
            title: row.name,
            subtitle: row.segment || row.email,
            href: customerLink({ kind: 'companies', id: row.id }),
          })),
          ...contacts.map((row) => ({
            type: 'contact' as const,
            title: row.name,
            subtitle: row.position || row.email,
            href: customerLink({ kind: 'contacts', id: row.id }),
          })),
          ...leads.map((row) => ({
            type: 'lead' as const,
            title: row.title,
            subtitle: row.converted ? 'Converted' : 'Open',
            href: salesLink('leads', row.id),
          })),
          ...deals.map((row) => ({
            type: 'deal' as const,
            title: row.title,
            subtitle: '',
            href: salesLink('deals', row.id),
          })),
          ...tasks.map((row) => ({
            type: 'task' as const,
            title: row.title,
            subtitle: timestamp(row.due_at, locale),
            href: activityHref('tasks', row.id),
          })),
          ...meetings.map((row) => ({
            type: 'meeting' as const,
            title: row.title,
            subtitle: timestamp(row.starts_at, locale),
            href: activityHref('meetings', row.id),
          })),
        ];
        if (!controller.signal.aborted) setResponse({ key, rows });
      } catch (error) {
        if (!controller.signal.aborted) setResponse({ key, rows: [], error: errorText(error) });
      }
    }
    const timer = window.setTimeout(() => void findRecords(), 180);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [key, term, workspace.id, request, locale]);

  const found = response?.key === key ? response : null;
  return (
    <>
      <PageTitle title={copy.title} description={copy.subtitle} />
      <label className="detail-search-box">
        <img src={searchIcon} alt="" />
        <input
          type="search"
          id="crm-search-input"
          autoFocus
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          aria-label={copy.title}
          placeholder={copy.placeholder}
        />
      </label>
      <section className="detail-card detail-results">
        <h2>{term ? copy.results : copy.suggested}</h2>
        {found?.error ? (
          <p role="alert">
            {found.error} <button onClick={reload}>Try again</button>
          </p>
        ) : (
          <p className="detail-muted" role="status">
            {found ? formatCount(copy.resultCount, found.rows.length, locale) : 'Searching…'}
          </p>
        )}
        <ul className="detail-results-list">
          {found?.rows.map((row) => (
            <li key={row.href}>
              <a href={row.href}>
                <img src={icons[row.type]} alt="" />
                <span>
                  <strong>{row.title}</strong>
                  <small>{[copy[row.type], row.subtitle].filter(Boolean).join(' · ')}</small>
                </span>
              </a>
            </li>
          ))}
        </ul>
        {found && !found.error && !found.rows.length && <p>{copy.empty}</p>}
      </section>
    </>
  );
}
