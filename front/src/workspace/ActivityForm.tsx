import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { AccessNotice, MemberOptions, Participants } from './ActivityMembers';
import { ActivityLinkPicker } from './ActivityLinks';
import { TaskReminderField } from './TaskReminderField';
import { canChangeOwner } from './SalesOwner';
import {
  linkTypes,
  localDateTime,
  localDay,
  taskChangedEvent,
  type ActivityKind,
  type ActivityRecord,
  type InitialLink,
  type LinkField,
  type Priority,
} from './activities';
import { accessError, errorMessage, useCustomerData, type Member } from './customerData';
import { RecordDialog } from './shared';

type FormProps = {
  kind: ActivityKind;
  record?: ActivityRecord;
  initialDate?: string;
  initialLink?: InitialLink;
  onSaved: (record: ActivityRecord) => void;
  onClose: () => void;
  onAccessChanged?: () => void;
};
type Payload = Record<string, string | number | string[] | null>;
function initialTimes(day?: string) {
  const now = new Date();
  const start = new Date(now);
  start.setMinutes(0, 0, 0);
  start.setHours(start.getHours() + 1);
  if (day && day !== localDay(now)) {
    const selected = new Date(day + 'T09:00');
    start.setTime(selected.getTime());
  }
  return { start: localDateTime(start), end: localDateTime(new Date(start.getTime() + 3600000)) };
}
function isoTime(value: string, label: string) {
  const result = new Date(value);
  if (!Number.isFinite(result.getTime()) || localDateTime(result) !== value)
    throw new Error('Enter a valid local ' + label + '.');
  return result.toISOString();
}

export function ActivityForm({
  kind,
  record,
  initialDate,
  initialLink,
  onSaved,
  onClose,
  onAccessChanged,
}: FormProps) {
  const { workspace, profile, request } = useCrm();
  const task = record && 'due_at' in record ? record : undefined;
  const meeting = record && 'starts_at' in record ? record : undefined;
  const isTask = kind === 'tasks';
  const elevated = canChangeOwner(workspace.role);
  const [title, setTitle] = useState(record?.title ?? '');
  const [body, setBody] = useState(task?.description ?? meeting?.goal ?? '');
  const [priority, setPriority] = useState<Priority>(task?.priority ?? 'normal');
  const [reminder, setReminder] = useState(task?.reminder_minutes?.toString() ?? '');
  const [owner, setOwner] = useState(record?.owner_id ?? profile.id);
  const [originalTimes] = useState(() => {
    if (task) return { start: localDateTime(new Date(task.due_at)), end: '' };
    if (meeting)
      return {
        start: localDateTime(new Date(meeting.starts_at)),
        end: localDateTime(new Date(meeting.ends_at)),
      };
    return initialTimes(initialDate);
  });
  const [start, setStart] = useState(originalTimes.start);
  const [end, setEnd] = useState(originalTimes.end);
  const [participants, setParticipants] = useState(meeting?.participant_ids ?? []);
  const [links, setLinks] = useState<Record<LinkField, string>>(() => ({
    company_id: record?.company_id ?? (initialLink?.field === 'company_id' ? initialLink.id : ''),
    contact_id: record?.contact_id ?? (initialLink?.field === 'contact_id' ? initialLink.id : ''),
    deal_id: record?.deal_id ?? (initialLink?.field === 'deal_id' ? initialLink.id : ''),
  }));
  const [rosterRevision, setRosterRevision] = useState(0);
  const roster = useCustomerData<Member[]>(
    elevated || !isTask ? '/auth/workspaces/' + workspace.id + '/members' : null,
    rosterRevision,
  );
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  function changes(): Payload {
    const p: Payload = {};
    const name = title.trim(),
      text = body.trim();
    if (!name || Array.from(name).length > 200) throw new Error('Enter a title of 1 to 200 characters.');
    if (!isTask && !text) throw new Error('Enter a meeting goal.');
    if (Array.from(text).length > 4000)
      throw new Error((isTask ? 'Description' : 'Goal') + ' must be 4,000 characters or fewer.');
    if (elevated && owner !== (record?.owner_id ?? profile.id)) p.owner_id = owner;
    if (!record || name !== record.title) p.title = name;
    if (isTask) {
      const minutes = reminder === '' ? null : Number(reminder);
      if (minutes !== null && (!Number.isInteger(minutes) || minutes < 0 || minutes > 43200))
        throw new Error('Enter a reminder between 0 and 43,200 minutes before the deadline.');
      if (minutes !== (task?.reminder_minutes ?? null)) p.reminder_minutes = minutes;
      if (text !== (task?.description ?? '')) p.description = text;
      if (!task || priority !== task.priority) p.priority = priority;
      if (!task || start !== originalTimes.start) p.due_at = isoTime(start, 'due date and time');
    } else {
      if (!meeting || text !== meeting.goal) p.goal = text;
      const startISO =
        meeting && start === originalTimes.start ? meeting.starts_at : isoTime(start, 'start time');
      const endISO = meeting && end === originalTimes.end ? meeting.ends_at : isoTime(end, 'end time');
      if (new Date(endISO).getTime() <= new Date(startISO).getTime())
        throw new Error('End time must be after the start time.');
      if (!meeting || start !== originalTimes.start) p.starts_at = startISO;
      if (!meeting || end !== originalTimes.end) p.ends_at = endISO;
      const previous = new Set(meeting?.participant_ids ?? []);
      if (previous.size !== participants.length || participants.some((id) => !previous.has(id)))
        p.participant_ids = participants;
    }
    for (const field of Object.keys(linkTypes) as LinkField[]) {
      if (links[field] !== (record?.[field] ?? '')) p[field] = links[field] || null;
    }
    return p;
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    if (active.current) return;
    let payload: Payload;
    try {
      payload = changes();
    } catch (cause) {
      setError(errorMessage(cause));
      return;
    }
    if (!Object.keys(payload).length) {
      onClose();
      return;
    }
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      const saved = await request<ActivityRecord>(
        '/crm/workspaces/' + workspace.id + '/' + kind + (record ? '/' + record.id : ''),
        {
          method: record ? 'PATCH' : 'POST',
          body: JSON.stringify(payload),
          signal: controller.signal,
        },
      );
      if (!controller.signal.aborted) {
        if (isTask) window.dispatchEvent(new Event(taskChangedEvent));
        onSaved(saved);
      }
    } catch (cause) {
      if (!controller.signal.aborted) {
        setError(errorMessage(cause));
        if (accessError(cause)) onAccessChanged?.();
        else if (elevated || !isTask) setRosterRevision((n) => n + 1);
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <form className="customer-form activity-form" onSubmit={save}>
      <fieldset className="customer-field-grid" disabled={busy}>
        <label className="customer-wide">
          Title
          <input
            required
            autoFocus
            aria-label="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>
        <label className="customer-wide">
          {isTask ? 'Description' : 'Goal'}
          <textarea
            aria-label={isTask ? 'Description' : 'Goal'}
            rows={3}
            required={!isTask}
            value={body}
            onChange={(e) => setBody(e.target.value)}
          />
        </label>
        {elevated && (
          <MemberOptions
            caption={isTask ? 'Owner' : 'Organizer'}
            id={owner}
            setId={setOwner}
            members={roster?.data}
          />
        )}
        {isTask && (
          <label>
            Priority
            <select
              aria-label="Priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as Priority)}
            >
              <option value="low">Low</option>
              <option value="normal">Normal</option>
              <option value="high">High</option>
            </select>
          </label>
        )}
        <label>
          {isTask ? 'Due' : 'Starts'}
          <input
            type="datetime-local"
            required
            min="0001-01-01T00:00"
            max="9999-12-31T23:59"
            value={start}
            onChange={(e) => setStart(e.target.value)}
          />
        </label>
        {!isTask && (
          <label>
            Ends
            <input
              type="datetime-local"
              required
              min="0001-01-01T00:00"
              max="9999-12-31T23:59"
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </label>
        )}
        {isTask && <TaskReminderField value={reminder} onChange={setReminder} />}
        {!isTask && (
          <Participants
            selected={participants}
            onChange={setParticipants}
            owner={owner}
            members={roster?.data}
          />
        )}
        {(elevated || !isTask) && (
          <AccessNotice loading={!roster} error={roster?.error} retry={() => setRosterRevision((n) => n + 1)} />
        )}
        <div className="activity-link-fields customer-wide">
          {(Object.keys(linkTypes) as LinkField[]).map((field) => (
            <ActivityLinkPicker
              key={field}
              field={field}
              value={links[field]}
              initialLabel={
                initialLink?.field === field && initialLink.id === links[field] ? initialLink.label : undefined
              }
              onChange={(value) => setLinks((before) => ({ ...before, [field]: value }))}
            />
          ))}
        </div>
      </fieldset>
      {error && (
        <p role="alert" className="customer-error">
          {error}
        </p>
      )}
      <div className="customer-actions">
        <button className="ws-action" disabled={busy}>
          {busy ? 'Saving…' : record ? 'Save changes' : 'Create ' + (isTask ? 'task' : 'meeting')}
        </button>
        <button type="button" className="ws-pill" onClick={onClose}>
          Cancel
        </button>
      </div>
    </form>
  );
}

export function CreateActivityDialog(props: Omit<FormProps, 'record'>) {
  const { workspace } = useCrm();
  if (workspace.role === 'support') return null;
  return (
    <RecordDialog title={props.kind === 'tasks' ? 'New task' : 'New meeting'} onClose={props.onClose}>
      <div className="customer-card">
        <ActivityForm {...props} />
      </div>
    </RecordDialog>
  );
}
