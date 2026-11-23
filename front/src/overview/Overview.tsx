import { useEffect, useRef, useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { activityHref, canEditActivity, type Priority, type CrmTask } from '../workspace/activities';
import { accessError, errorMessage, useCustomerData, type Member } from '../workspace/customerData';
import { salesLink } from '../workspace/salesData';
import { firstPage, pageSize, useOverviewDay, type Group, type Offsets, type Snapshot } from './dashboard';
import './overview.css';

const sections: Group[] = ['tasks', 'meetings', 'at_risk_deals'];
const priorities: Record<Priority, string> = { high: 'High', normal: 'Normal', low: 'Low' };
function Pager({
  title,
  offset,
  total,
  onChange,
}: {
  title: string;
  offset: number;
  total: number;
  onChange: (next: number) => void;
}) {
  if (total <= pageSize && offset === 0) return null;
  return (
    <nav className="overview-pager" aria-label={title + ' pages'}>
      <button disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - pageSize))}>
        Previous
      </button>
      <span>
        {Math.min(total, offset + 1)}–{Math.min(total, offset + pageSize)} of {total}
      </span>
      <button disabled={offset + pageSize >= total} onClick={() => onChange(offset + pageSize)}>
        Next
      </button>
    </nav>
  );
}
function Summary({ data, locale }: { data: Snapshot; locale: string }) {
  const number = new Intl.NumberFormat(locale);
  return (
    <section className="overview-metrics" aria-label="Daily summary">
      <div className="overview-focus">
        <h2>Daily summary</h2>
        <p className="overview-focus-title">{number.format(data.tasks.total)}</p>
        <p className="overview-focus-description">Open tasks due by the end of today</p>
      </div>
      {(
        [
          ['Overdue tasks', data.tasks.overdue_total, 'metric-icon2.svg'],
          ['Meetings in the next 7 days', data.meetings.total, 'metric-icon1.svg'],
          ['Deals needing attention', data.at_risk_deals.total, 'metric-icon3.svg'],
        ] as const
      ).map(([label, count, icon]) => (
        <div className="overview-metric" key={label}>
          <img src={'/icons/' + icon} width="32" height="32" alt="" />
          <strong className="overview-metric-value">{number.format(count)}</strong>
          <span className="overview-metric-label">{label}</span>
        </div>
      ))}
    </section>
  );
}

export function Overview() {
  const { workspace, profile, request } = useCrm();
  const { locale } = useLocale();
  const { day, revision, refresh } = useOverviewDay();
  const [paging, setPaging] = useState<{ day: string; offsets: Offsets }>({
    day: day.from,
    offsets: firstPage,
  });
  const offsets = paging.day === day.from ? paging.offsets : firstPage;
  const params = new URLSearchParams({
    from: day.from,
    to: day.to,
    limit: String(pageSize),
    task_offset: String(offsets.tasks),
    meeting_offset: String(offsets.meetings),
    deal_offset: String(offsets.at_risk_deals),
  });
  const base = '/crm/workspaces/' + workspace.id;
  const result = useCustomerData<Snapshot>(base + '/overview?' + params, revision);
  const roster = useCustomerData<Member[]>('/auth/workspaces/' + workspace.id + '/members', revision);
  const data = result?.data;
  const [busy, setBusy] = useState('');
  const [feedback, setFeedback] = useState<{ error: boolean; text: string } | null>(null);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (!data) return;
    const clamped = { ...offsets };
    let changed = false;
    for (const group of sections) {
      const last = Math.max(0, Math.ceil(data[group].total / pageSize) - 1) * pageSize;
      if (clamped[group] > last) {
        clamped[group] = last;
        changed = true;
      }
    }
    if (changed) setPaging({ day: day.from, offsets: clamped });
  }, [data, day.from, offsets]);
  const date = new Intl.DateTimeFormat(locale, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  const when = new Intl.DateTimeFormat(locale, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
  const clock = new Intl.DateTimeFormat(locale, { hour: 'numeric', minute: '2-digit' });
  const asOf = Date.parse(data?.as_of ?? '');
  const denied = accessError(result?.error) || accessError(roster?.error);
  function owner(id: string) {
    return roster?.data?.find((m) => m.user_id === id)?.email || (id === profile.id && profile.email) || id;
  }
  function page(group: Group, title: string) {
    return (
      <Pager
        title={title}
        offset={offsets[group]}
        total={data?.[group].total ?? 0}
        onChange={(next) => setPaging({ day: day.from, offsets: { ...offsets, [group]: next } })}
      />
    );
  }
  async function setPriority(task: CrmTask, priority: Priority) {
    if (active.current || priority === task.priority || !canEditActivity(task, workspace.role, profile.id))
      return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(task.id);
    setFeedback(null);
    try {
      await request<CrmTask>(base + '/tasks/' + task.id, {
        method: 'PATCH',
        body: JSON.stringify({ priority }),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) {
        setFeedback({ error: false, text: 'Task priority updated.' });
        refresh();
      }
    } catch (cause) {
      if (!controller.signal.aborted) {
        setFeedback({ error: true, text: errorMessage(cause) });
        refresh();
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy('');
    }
  }
  return (
    <div className="work-overview">
      <header className="overview-heading">
        <div>
          <h1>Work overview</h1>
          <p>
            {workspace.name} · {date.format(new Date(day.from))} · {date.resolvedOptions().timeZone}
          </p>
        </div>
        <button type="button" className="overview-button overview-button-dark" onClick={refresh}>
          Refresh overview
        </button>
      </header>
      {denied || result?.error != null ? (
        <p role="alert" className="overview-panel overview-state">
          {denied ? 'Your workspace access has changed.' : errorMessage(result?.error)}{' '}
          <button type="button" className="ws-text-button" onClick={refresh}>
            Try again
          </button>
        </p>
      ) : !data ? (
        <p role="status" className="overview-panel overview-state">
          Loading overview…
        </p>
      ) : (
        <>
          {feedback && (
            <p
              className={'overview-feedback' + (feedback.error ? ' customer-error' : '')}
              role={feedback.error ? 'alert' : 'status'}
            >
              {feedback.text}
            </p>
          )}
          {roster?.error != null && (
            <p role="alert" className="overview-feedback">
              Could not load team member names.{' '}
              <button className="ws-text-button" onClick={refresh}>
                Retry
              </button>
            </p>
          )}
          <Summary data={data} locale={locale} />
          <div className="overview-middle">
            <section className="overview-panel" aria-labelledby="plan-title">
              <header className="overview-panel-header">
                <h2 id="plan-title">Daily plan</h2>
                <p>Open tasks by priority, then due time</p>
              </header>
              {data.tasks.items.length ? (
                <ul className="overview-tasks">
                  {data.tasks.items.map((task) => (
                    <li key={task.id}>
                      <div>
                        <a href={activityHref('tasks', task.id)}>{task.title}</a>
                        <small>{owner(task.owner_id)}</small>
                        <small className={Date.parse(task.due_at) < asOf ? 'overview-overdue' : undefined}>
                          {Date.parse(task.due_at) < asOf ? 'Overdue · ' : 'Due · '}
                          <time dateTime={task.due_at}>{when.format(new Date(task.due_at))}</time>
                        </small>
                      </div>
                      {canEditActivity(task, workspace.role, profile.id) ? (
                        <select
                          aria-label={'Priority for ' + task.title}
                          value={task.priority}
                          disabled={!!busy}
                          onChange={(event) => void setPriority(task, event.target.value as Priority)}
                        >
                          {Object.entries(priorities).map(([value, label]) => (
                            <option value={value} key={value}>
                              {label}
                            </option>
                          ))}
                        </select>
                      ) : (
                        <span className="overview-priority">{priorities[task.priority]} priority</span>
                      )}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="overview-empty">No open tasks due by the end of today.</p>
              )}
              {page('tasks', 'Daily plan')}
            </section>
            <section className="overview-panel" aria-labelledby="meetings-title">
              <header className="overview-panel-header">
                <h2 id="meetings-title">Meetings</h2>
                <p>In progress and coming up over the next seven days</p>
              </header>
              {data.meetings.items.length ? (
                <ul className="overview-agenda">
                  {data.meetings.items.map((meeting) => {
                    const start = new Date(meeting.starts_at),
                      end = new Date(meeting.ends_at);
                    return (
                      <li key={meeting.id}>
                        <a href={activityHref('meetings', meeting.id)}>
                          <time dateTime={meeting.starts_at}>{clock.format(start)}</time>
                          <span className="overview-meeting-details">
                            <strong>{meeting.title}</strong>
                            <span>
                              {start.getTime() <= asOf && end.getTime() > asOf ? 'In progress · ' : ''}
                              {date.format(start)}
                            </span>
                            <span>{owner(meeting.owner_id)}</span>
                          </span>
                          <img src="/icons/arrow-up-right1.svg" alt="" width="8" height="8" />
                        </a>
                      </li>
                    );
                  })}
                </ul>
              ) : (
                <p className="overview-empty">No meetings in progress or planned in the next seven days.</p>
              )}
              {page('meetings', 'Meetings')}
            </section>
          </div>
          <section className="overview-panel overview-attention" aria-labelledby="attention-title">
            <header>
              <h2 id="attention-title">Deals needing attention</h2>
              <a className="overview-button" href="#/sales/deals">
                All deals
              </a>
            </header>
            <p className="overview-attention-note">
              Alerts reflect the tasks and activity you can see for each open deal.
            </p>
            {data.at_risk_deals.items.length ? (
              <div className="overview-table-wrap">
                <table className="overview-deals" aria-labelledby="attention-title">
                  <thead>
                    <tr>
                      <th scope="col">Open deal</th>
                      <th scope="col">Needs attention</th>
                      <th scope="col">Last activity</th>
                      <th scope="col">Owner</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.at_risk_deals.items.map((deal) => (
                      <tr key={deal.id}>
                        <th scope="row">
                          <a href={salesLink('deals', deal.id)}>{deal.title}</a>
                        </th>
                        <td>
                          {deal.missing_next_task && <div>No next task scheduled</div>}
                          {deal.stale && <div>No activity in the last 7 days</div>}
                        </td>
                        <td>
                          <time dateTime={deal.last_activity_at}>
                            {when.format(new Date(deal.last_activity_at))}
                          </time>
                        </td>
                        <td>{owner(deal.owner_id)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="overview-empty">No open deals need attention.</p>
            )}
            {page('at_risk_deals', 'Deals needing attention')}
          </section>
        </>
      )}
    </div>
  );
}
