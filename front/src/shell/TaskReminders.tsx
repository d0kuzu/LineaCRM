import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError, errorText } from '../crm/http';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { activityHref, taskChangedEvent } from '../workspace/activities';
import { timestamp } from '../workspace/customerData';
import { RecordDialog } from '../workspace/shared';

type Reminder = { task_id: string; title: string; due_at: string; scheduled_at: string; token: string };
type Result = { url: string; rows: Reminder[] } | { url: string; error: string };
const pageSize = 10;

export function TaskReminders() {
  const { workspace, request } = useCrm();
  const { locale } = useLocale();
  const [open, setOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [revision, setRevision] = useState(0);
  const [result, setResult] = useState<Result | null>(null);
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const write = useRef<AbortController | null>(null);
  const base = '/crm/workspaces/' + workspace.id;
  const url = base + '/task-reminders?limit=' + (pageSize + 1) + '&offset=' + page * pageSize;

  useEffect(() => () => write.current?.abort(), []);
  useEffect(() => {
    let pending: AbortController | undefined;
    async function read() {
      if (document.hidden) return;
      pending?.abort();
      const controller = new AbortController();
      pending = controller;
      try {
        const rows = await request<Reminder[]>(url, { signal: controller.signal });
        if (!controller.signal.aborted) setResult({ url, rows });
      } catch (error) {
        if (!controller.signal.aborted) setResult({ url, error: errorText(error) });
      } finally {
        if (pending === controller) pending = undefined;
      }
    }
    function refresh() {
      void read();
    }
    function resume() {
      if (!document.hidden) {
        setResult(null);
        refresh();
      }
    }
    refresh();
    const timer = window.setInterval(() => {
      if (!pending) refresh();
    }, 30_000);
    document.addEventListener('visibilitychange', resume);
    window.addEventListener('focus', resume);
    window.addEventListener(taskChangedEvent, refresh);
    return () => {
      pending?.abort();
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', resume);
      window.removeEventListener('focus', resume);
      window.removeEventListener(taskChangedEvent, refresh);
    };
  }, [url, revision, request]);

  const current = result?.url === url ? result : null;
  const rows = current && 'rows' in current ? current.rows : undefined;
  const count = rows ? page * pageSize + rows.length : 0;
  const countLabel = count > pageSize ? pageSize + '+' : String(count);
  useEffect(() => {
    if (page > 0 && rows?.length === 0) setPage(0);
  }, [page, rows]);

  const close = useCallback(() => {
    setOpen(false);
    setNotice('');
    setPage(0);
  }, []);
  useEffect(() => {
    if (!open) return;
    window.addEventListener('hashchange', close);
    return () => window.removeEventListener('hashchange', close);
  }, [open, close]);
  async function dismiss(reminder: Reminder) {
    if (write.current) return;
    const controller = new AbortController();
    write.current = controller;
    setBusy(true);
    setNotice('');
    try {
      await request(base + '/tasks/' + reminder.task_id + '/reminder/dismiss', {
        method: 'POST',
        body: JSON.stringify({ token: reminder.token }),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) setRevision((n) => n + 1);
    } catch (error) {
      if (!controller.signal.aborted) {
        if (error instanceof ApiError && [403, 404, 409].includes(error.status)) {
          setNotice('This reminder has changed. The list has been refreshed.');
          setRevision((n) => n + 1);
        } else setNotice(errorText(error));
      }
    } finally {
      write.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }

  return (
    <>
      <button
        type="button"
        className="toolbar-reminders"
        aria-haspopup="dialog"
        onClick={() => {
          setResult(null);
          setOpen(true);
          setNotice('');
          setRevision((n) => n + 1);
        }}
      >
        Reminders{' '}
        <span aria-live="polite">{count > 0 && <span className="reminder-count">{countLabel}</span>}</span>
      </button>
      {open && (
        <RecordDialog title="Task reminders" onClose={close}>
          {notice && <p role="status">{notice}</p>}
          {current && 'error' in current ? (
            <p role="alert">
              {current.error}{' '}
              <button type="button" className="ws-text-button" onClick={() => setRevision((n) => n + 1)}>
                Retry
              </button>
            </p>
          ) : !rows ? (
            <p role="status">Loading reminders…</p>
          ) : rows.length === 0 ? (
            <p>No reminders due.</p>
          ) : (
            <ul className="reminder-list">
              {rows.slice(0, pageSize).map((row) => (
                <li key={row.token}>
                  <div>
                    <a href={activityHref('tasks', row.task_id)} onClick={close}>
                      {row.title}
                    </a>
                    <small>
                      Due <time dateTime={row.due_at}>{timestamp(row.due_at, locale)}</time>
                    </small>
                  </div>
                  <button
                    type="button"
                    className="ws-text-button"
                    disabled={busy}
                    aria-label={'Dismiss ' + row.title}
                    onClick={() => void dismiss(row)}
                  >
                    Dismiss
                  </button>
                </li>
              ))}
            </ul>
          )}
          {(page > 0 || (rows?.length ?? 0) > pageSize) && (
            <nav className="customer-pager" aria-label="Reminder pages">
              <button type="button" disabled={busy || !page} onClick={() => setPage((n) => n - 1)}>
                Previous
              </button>
              <span>Page {page + 1}</span>
              <button
                type="button"
                disabled={busy || (rows?.length ?? 0) <= pageSize}
                onClick={() => setPage((n) => n + 1)}
              >
                Next
              </button>
            </nav>
          )}
        </RecordDialog>
      )}
    </>
  );
}
