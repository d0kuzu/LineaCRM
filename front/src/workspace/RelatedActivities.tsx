import { useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { CreateActivityDialog } from './ActivityForm';
import {
  activityHref,
  useActivityRevision,
  type ActivityKind,
  type ActivityRecord,
  type InitialLink,
} from './activities';
import { errorMessage, timestamp, useCustomerData } from './customerData';

const pageSize = 5;

function RelatedList({
  kind,
  parent,
  revision,
}: {
  kind: ActivityKind;
  parent: InitialLink;
  revision: number;
}) {
  const { workspace } = useCrm();
  const { locale } = useLocale();
  const [page, setPage] = useState(0);
  const [retry, setRetry] = useState(0);
  const [creating, setCreating] = useState(false);
  const tasks = kind === 'tasks';
  const title = tasks ? 'Tasks' : 'Meetings';
  const params = new URLSearchParams({
    [parent.field]: parent.id,
    limit: String(pageSize + 1),
    offset: String(page * pageSize),
  });
  const result = useCustomerData<ActivityRecord[]>(
    '/crm/workspaces/' + workspace.id + '/' + kind + '?' + params,
    revision + retry,
  );
  const rows = result?.data;
  return (
    <section>
      <div className="ws-section-head">
        <h3>{title}</h3>
        {workspace.role !== 'support' && (
          <button className="ws-text-button" onClick={() => setCreating(true)}>
            {tasks ? 'New task' : 'New meeting'}
          </button>
        )}
      </div>
      {result?.error != null ? (
        <p className="customer-error" role="alert">
          {errorMessage(result.error)}{' '}
          <button className="ws-text-button" onClick={() => setRetry((n) => n + 1)}>
            Retry {kind}
          </button>
        </p>
      ) : !rows ? (
        <p role="status">Loading {kind}…</p>
      ) : rows.length ? (
        <ul className="customer-linked">
          {rows.slice(0, pageSize).map((row) => (
            <li key={row.id}>
              <a className="ws-text-button" href={activityHref(kind, row.id)}>
                {row.title}
              </a>
              <small>
                {'due_at' in row
                  ? [row.status === 'done' ? 'Completed' : 'Due', timestamp(row.due_at, locale)].join(' · ')
                  : timestamp(row.starts_at, locale)}
              </small>
            </li>
          ))}
        </ul>
      ) : (
        <p className="ws-subtext">No linked {kind}.</p>
      )}
      {(page > 0 || (rows?.length ?? 0) > pageSize) && (
        <div className="customer-pager">
          <button disabled={!page} onClick={() => setPage((p) => p - 1)}>
            Previous {kind}
          </button>
          <button disabled={(rows?.length ?? 0) <= pageSize} onClick={() => setPage((p) => p + 1)}>
            More {kind}
          </button>
        </div>
      )}
      {creating && (
        <CreateActivityDialog
          kind={kind}
          initialLink={parent}
          onClose={() => setCreating(false)}
          onSaved={(row) => {
            setCreating(false);
            window.location.hash = activityHref(kind, row.id);
          }}
        />
      )}
    </section>
  );
}

export function RelatedActivities({ parent }: { parent: InitialLink }) {
  const [revision] = useActivityRevision();
  return (
    <>
      <RelatedList key={'tasks/' + parent.id} kind="tasks" parent={parent} revision={revision} />
      <RelatedList key={'meetings/' + parent.id} kind="meetings" parent={parent} revision={revision} />
    </>
  );
}
