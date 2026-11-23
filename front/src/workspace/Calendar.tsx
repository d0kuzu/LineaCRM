import { useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { CreateActivityDialog } from './ActivityForm';
import { activityHref, localDay, shiftDay, useActivityRevision, type CrmMeeting } from './activities';
import { accessError, errorMessage, useCustomerData } from './customerData';
import { PageTitle, Switcher } from './shared';
import './activities.css';
const pageSize = 35;
function endOfDay(day: string) {
  const next = new Date(day + 'T00:00');
  next.setDate(next.getDate() + 1);
  return next;
}
function weekStart(day: string) {
  return shiftDay(day, -((new Date(day + 'T12:00').getDay() + 6) % 7));
}
export function Calendar() {
  const { workspace } = useCrm();
  const { locale } = useLocale();
  const [view, setView] = useState<'day' | 'week'>('week'),
    [day, setDay] = useState(() => localDay(new Date()));
  const [creating, setCreating] = useState(false),
    [page, setPage] = useState(0);
  const [revision, refresh] = useActivityRevision();
  const start = view === 'day' ? day : weekStart(day),
    length = view === 'day' ? 1 : 7;
  const days = [...new Set(Array.from({ length }, (_, i) => shiftDay(start, i)))];
  const last = days[days.length - 1];
  const from = new Date(start + 'T00:00');
  const to = endOfDay(last);
  const params = new URLSearchParams({
    limit: String(pageSize + 1),
    offset: String(page * pageSize),
  });
  // Local midnight can fall outside RFC 3339's four-digit years at either edge.
  if (from.getUTCFullYear() >= 1) params.set('from', from.toISOString());
  if (to.getUTCFullYear() <= 9999) params.set('to', to.toISOString());
  const rows = useCustomerData<CrmMeeting[]>(
    '/crm/workspaces/' + workspace.id + '/meetings?' + params,
    revision,
  );
  const records = rows?.data?.slice(0, pageSize) ?? [];
  const dateFormat = new Intl.DateTimeFormat(locale, { weekday: 'short', month: 'short', day: 'numeric' });
  const timeFormat = new Intl.DateTimeFormat(locale, { hour: 'numeric', minute: '2-digit' });
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  function go(next: string) {
    if (
      !/^\d{4}-\d{2}-\d{2}$/.test(next) ||
      next < '0001-01-01' ||
      localDay(new Date(next + 'T12:00')) !== next
    )
      return;
    setDay(next);
    setPage(0);
  }
  return (
    <>
      <div className="customer-page-head">
        <PageTitle title="Calendar" description={workspace.name + ' · ' + zone} />
        {workspace.role !== 'support' && !accessError(rows?.error) && (
          <button className="ws-action" onClick={() => setCreating(true)}>
            New meeting
          </button>
        )}
      </div>
      <div className="ws-toolbar customer-toolbar">
        <Switcher
          values={[
            ['week', 'Week'],
            ['day', 'Day'],
          ]}
          selected={view}
          label="Calendar view"
          onSelect={(v) => {
            setView(v);
            setPage(0);
          }}
        />
        <div className="activity-calendar-nav">
          <button
            className="ws-pill"
            disabled={start === '0001-01-01'}
            onClick={() => go(shiftDay(day, -length))}
          >
            Previous {view}
          </button>
          <button className="ws-pill" onClick={() => go(localDay(new Date()))}>
            Today
          </button>
          <button
            className="ws-pill"
            disabled={last === '9999-12-31'}
            onClick={() => go(shiftDay(day, length))}
          >
            Next {view}
          </button>
          <input
            type="date"
            aria-label="Calendar date"
            value={day}
            min="0001-01-01"
            max="9999-12-31"
            onChange={(e) => go(e.target.value)}
          />
        </div>
      </div>
      <section className="ws-panel">
        <div className="ws-section-head">
          <h2>
            {dateFormat.format(new Date(start + 'T12:00'))}
            {view === 'week' ? ' – ' + dateFormat.format(new Date(last + 'T12:00')) : ''}
          </h2>
          <button className="ws-text-button" onClick={refresh}>
            Refresh calendar
          </button>
        </div>
        {!rows ? (
          <p role="status" className="ws-empty">
            Loading meetings…
          </p>
        ) : rows.error != null ? (
          <p role="alert" className="customer-error">
            {errorMessage(rows.error)}
          </p>
        ) : (
          <div className={'activity-agenda ' + (view === 'day' ? 'activity-day' : '')}>
            {days.map((value) => {
              const from = new Date(value + 'T00:00'),
                to = endOfDay(value);
              const events = records.filter((m) => new Date(m.starts_at) < to && new Date(m.ends_at) > from);
              return (
                <section className="activity-agenda-day" aria-label={value} key={value}>
                  <h3>
                    <button
                      className="ws-text-button"
                      aria-label={'Open day ' + value}
                      onClick={() => {
                        go(value);
                        setView('day');
                      }}
                    >
                      {dateFormat.format(new Date(value + 'T12:00'))}
                    </button>
                  </h3>
                  {events.map((m) => {
                    const begin = new Date(m.starts_at),
                      end = new Date(m.ends_at);
                    const interval =
                      (begin < from ? 'Continues' : timeFormat.format(begin)) +
                      ' – ' +
                      (end > to ? 'next day' : timeFormat.format(end));
                    return (
                      <a className="activity-event" key={m.id} href={activityHref('meetings', m.id)}>
                        <small>{interval}</small>
                        <strong>{m.title}</strong>
                      </a>
                    );
                  })}
                  {!events.length && (
                    <small className="ws-subtext">
                      {page > 0 || (rows?.data?.length ?? 0) > pageSize
                        ? 'No meetings on this page.'
                        : 'No meetings.'}
                    </small>
                  )}
                </section>
              );
            })}
          </div>
        )}
        {!rows?.error && (page > 0 || (rows?.data?.length ?? 0) > pageSize) && (
          <nav className="customer-pager" aria-label="Calendar pages">
            <button disabled={!page} onClick={() => setPage((n) => n - 1)}>
              Previous page
            </button>
            <span>Page {page + 1}</span>
            <button disabled={(rows?.data?.length ?? 0) <= pageSize} onClick={() => setPage((n) => n + 1)}>
              Next page
            </button>
          </nav>
        )}
      </section>
      {creating && !accessError(rows?.error) && (
        <CreateActivityDialog
          kind="meetings"
          initialDate={day}
          onClose={() => setCreating(false)}
          onSaved={(row) => {
            setCreating(false);
            window.location.hash = activityHref('meetings', row.id);
          }}
        />
      )}
    </>
  );
}
