import { useEffect, useRef, useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { ActivityForm } from './ActivityForm';
import { ActivityLink } from './ActivityLinks';
import {
  canEditActivity,
  linkTypes,
  useActivityRevision,
  type ActivityKind,
  type ActivityRecord,
  type CrmTask,
  type LinkField,
} from './activities';
import { accessError, errorMessage, timestamp, useCustomerData, type Member } from './customerData';
import { Fields, PageTitle } from './shared';
import './customers.css';
import './activities.css';

export function TaskCompletion({ task, onChanged }: { task: CrmTask; onChanged: () => void }) {
  const { request, workspace, profile } = useCrm();
  const [busy, setBusy] = useState(false),
    [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  if (!canEditActivity(task, workspace.role, profile.id)) return null;
  async function toggle() {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      await request<CrmTask>('/crm/workspaces/' + workspace.id + '/tasks/' + task.id, {
        method: 'PATCH',
        body: JSON.stringify({ status: task.status === 'done' ? 'open' : 'done' }),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) onChanged();
    } catch (cause) {
      if (!controller.signal.aborted) {
        setError(errorMessage(cause));
        if (accessError(cause)) onChanged();
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <span>
      <button type="button" className="ws-pill" disabled={busy} onClick={() => void toggle()}>
        {busy ? 'Saving…' : task.status === 'done' ? 'Reopen task' : 'Complete task'}
      </button>
      {error && (
        <span role="alert" className="customer-error">
          {error}
        </span>
      )}
    </span>
  );
}

export function ActivityDetail({ kind, id }: { kind: ActivityKind; id: string }) {
  const { workspace, profile } = useCrm();
  const { locale } = useLocale();
  const [revision, refresh] = useActivityRevision();
  const base = '/crm/workspaces/' + workspace.id;
  const result = useCustomerData<ActivityRecord>(base + '/' + kind + '/' + id, revision);
  const members = useCustomerData<Member[]>('/auth/workspaces/' + workspace.id + '/members', revision);
  const [editing, setEditing] = useState<ActivityRecord | null>(null);
  const record = result?.data;
  const denied = accessError(result?.error) || accessError(members?.error);
  const allowed = record && canEditActivity(record, workspace.role, profile.id);
  useEffect(() => {
    if (denied || (record && !allowed)) setEditing(null);
  }, [record, allowed, denied]);
  function member(id: string) {
    return members?.data?.find((m) => m.user_id === id)?.email ?? (profile.id === id ? profile.email : id);
  }
  const task = record && 'due_at' in record ? record : undefined,
    meeting = record && 'starts_at' in record ? record : undefined;
  const noun = kind === 'tasks' ? 'task' : 'meeting';
  return (
    <>
      <a className="ws-text-button" href={kind === 'tasks' ? '#/tasks' : '#/calendar'}>
        {kind === 'tasks' ? 'Tasks' : 'Calendar'}
      </a>
      <PageTitle
        title={denied || !record ? (kind === 'tasks' ? 'Task' : 'Meeting') : record.title}
        description={workspace.name}
      />
      {!result && !denied && (
        <p className="ws-panel ws-empty" role="status">
          Loading {noun}…
        </p>
      )}
      {(denied || result?.error != null) && (
        <p className="ws-panel customer-error" role="alert">
          {denied ? 'This ' + noun + ' is unavailable.' : errorMessage(result?.error)}{' '}
          <button className="ws-text-button" onClick={refresh}>
            Retry record
          </button>
        </p>
      )}
      {editing && !denied && (
        <section className="ws-panel" hidden={!record || !allowed}>
          <h2>Edit {noun}</h2>
          <ActivityForm
            kind={kind}
            record={editing}
            onClose={() => setEditing(null)}
            onSaved={() => {
              setEditing(null);
              refresh();
            }}
            onAccessChanged={refresh}
          />
        </section>
      )}
      {record && !denied && !editing && (
        <section className="ws-panel customer-card activity-detail">
          <Fields
            rows={
              task
                ? [
                    ['Owner', member(record.owner_id)],
                    ['Due', timestamp(task.due_at, locale)],
                    ['Priority', task.priority],
                    ['Status', task.status === 'done' ? 'Completed' : 'Open'],
                  ]
                : meeting
                  ? [
                      ['Organizer', member(record.owner_id)],
                      ['Starts', timestamp(meeting.starts_at, locale)],
                      ['Ends', timestamp(meeting.ends_at, locale)],
                      ['Participants', meeting.participant_ids.map(member).join(', ') || 'None'],
                    ]
                  : []
            }
          />
          {(meeting || task?.description) && (
            <section>
              <h3>{meeting ? 'Goal' : 'Description'}</h3>
              <p className="activity-body">{meeting?.goal ?? task?.description}</p>
            </section>
          )}
          {(Object.keys(linkTypes) as LinkField[]).some((field) => record[field]) && (
            <section>
              <h3>Linked records</h3>
              <dl className="ws-fields">
                {(Object.keys(linkTypes) as LinkField[]).map(
                  (field) =>
                    record[field] && (
                      <div key={field}>
                        <dt>{linkTypes[field].label}</dt>
                        <dd>
                          <ActivityLink field={field} id={record[field]} />
                        </dd>
                      </div>
                    ),
                )}
              </dl>
            </section>
          )}
          {allowed && (
            <div className="customer-actions">
              <button className="ws-action" onClick={() => setEditing(record)}>
                Edit {noun}
              </button>
              {task && <TaskCompletion task={task} onChanged={refresh} />}
            </div>
          )}
        </section>
      )}
    </>
  );
}
