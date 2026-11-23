import { useEffect, useMemo, useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { CreateActivityDialog } from './ActivityForm';
import { TaskCompletion } from './ActivityDetail';
import { activityHref, useActivityRevision, type CrmMeeting, type CrmTask } from './activities';
import { accessError, errorMessage, timestamp, useCustomerData, type Member } from './customerData';
import { PageTitle, Switcher } from './shared';

const views = [
  ['open', 'Open'],
  ['overdue', 'Overdue'],
  ['done', 'Completed'],
  ['all', 'All tasks'],
] as const;
const pageSize = 20;
export function Tasks() {
  const { workspace, profile } = useCrm();
  const { locale } = useLocale();
  const [view, setView] = useState<(typeof views)[number][0]>('open');
  const [q, setQ] = useState(''),
    [page, setPage] = useState(0),
    [creating, setCreating] = useState(false);
  const [revision, refresh] = useActivityRevision();
  const [minute, setMinute] = useState(0);
  useEffect(() => {
    const clock = window.setInterval(() => {
      if (!document.hidden) setMinute((n) => n + 1);
    }, 60_000);
    return () => window.clearInterval(clock);
  }, []);
  const now = useMemo(() => new Date().toISOString(), [revision, view, minute]);
  const base = '/crm/workspaces/' + workspace.id;
  const params = new URLSearchParams({
    q: q.trim(),
    limit: String(pageSize + 1),
    offset: String(page * pageSize),
  });
  if (view !== 'all') params.set('status', view === 'done' ? 'done' : 'open');
  if (view === 'overdue') params.set('to', now);
  const rows = useCustomerData<CrmTask[]>(base + '/tasks?' + params, revision, 150);
  const members = useCustomerData<Member[]>('/auth/workspaces/' + workspace.id + '/members', revision);
  const nextMeeting = useCustomerData<CrmMeeting[]>(
    base + '/meetings?' + new URLSearchParams({ from: now, limit: '1' }),
    revision,
  );
  const denied = accessError(rows?.error) || accessError(members?.error);
  const next = nextMeeting?.data?.[0];
  return (
    <>
      <div className="customer-page-head">
        <PageTitle title="Tasks" description={workspace.name} />
        {workspace.role !== 'support' && !denied && (
          <button className="ws-action" onClick={() => setCreating(true)}>
            New task
          </button>
        )}
      </div>
      <div className="ws-toolbar customer-toolbar">
        <Switcher
          values={views}
          selected={view}
          label="Task status"
          onSelect={(v) => {
            setView(v);
            setPage(0);
          }}
        />
        <input
          className="ws-search"
          type="search"
          aria-label="Search tasks"
          placeholder="Search tasks"
          maxLength={200}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(0);
          }}
        />
      </div>
      <div className="ws-task-layout">
        <section className="ws-panel">
          <div className="ws-section-head">
            <h2>{views.find(([key]) => key === view)?.[1]}</h2>
            <button className="ws-text-button" onClick={refresh}>
              Refresh tasks
            </button>
          </div>
          {denied || rows?.error != null ? (
            <p className="customer-error" role="alert">
              {denied ? 'Your workspace access has changed.' : errorMessage(rows?.error)}
            </p>
          ) : !rows ? (
            <p role="status" className="ws-empty">
              Loading tasks…
            </p>
          ) : !rows.data?.length ? (
            <p role="status" className="ws-empty">
              No matching tasks.
            </p>
          ) : (
            rows.data.slice(0, pageSize).map((task) => (
              <article key={task.id} className="ws-task activity-task">
                <div>
                  <a className="ws-text-button" href={activityHref('tasks', task.id)}>
                    {task.title}
                  </a>
                  <small>
                    {members?.data?.find((m) => m.user_id === task.owner_id)?.email ??
                      (profile.id === task.owner_id ? profile.email : task.owner_id)}{' '}
                    · {task.priority} priority ·{' '}
                    {task.status === 'done'
                      ? 'Completed'
                      : new Date(task.due_at) < new Date(now)
                        ? 'Overdue'
                        : 'Open'}
                  </small>
                </div>
                <time dateTime={task.due_at}>{timestamp(task.due_at, locale)}</time>
                <TaskCompletion task={task} onChanged={refresh} />
              </article>
            ))
          )}
          {!denied && (page > 0 || (rows?.data?.length ?? 0) > pageSize) && (
            <nav className="customer-pager" aria-label="Task pages">
              <button disabled={!page} onClick={() => setPage((n) => n - 1)}>
                Previous
              </button>
              <span>Page {page + 1}</span>
              <button disabled={(rows?.data?.length ?? 0) <= pageSize} onClick={() => setPage((n) => n + 1)}>
                Next
              </button>
            </nav>
          )}
        </section>
        <aside className="ws-panel ws-next">
          <h2>
            {next && Date.parse(next.starts_at) <= Date.parse(now) ? 'Meeting in progress' : 'Upcoming meeting'}
          </h2>
          {denied ? null : !nextMeeting ? (
            <p role="status">Loading…</p>
          ) : nextMeeting.error != null ? (
            <p role="alert">{errorMessage(nextMeeting.error)}</p>
          ) : next ? (
            <>
              <p className="ws-subtext">{timestamp(next.starts_at, locale)}</p>
              <h3>{next.title}</h3>
              <p className="activity-body">{next.goal}</p>
              <a className="ws-action" href={activityHref('meetings', next.id)}>
                Open meeting
              </a>
            </>
          ) : (
            <p className="ws-subtext">No upcoming meetings.</p>
          )}
        </aside>
      </div>
      {creating && !denied && (
        <CreateActivityDialog
          kind="tasks"
          onClose={() => setCreating(false)}
          onSaved={(row) => {
            setCreating(false);
            window.location.hash = activityHref('tasks', row.id);
          }}
        />
      )}
    </>
  );
}
