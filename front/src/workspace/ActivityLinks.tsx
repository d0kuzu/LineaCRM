import { useState } from 'react';
import { useCrm } from '../crm/session';
import { linkTypes, type LinkField } from './activities';
import { errorMessage, useCustomerData } from './customerData';

type LinkedRow = { id: string; name?: string; title?: string };
export function ActivityLink({ field, id }: { field: LinkField; id: string }) {
  const { workspace } = useCrm();
  const { kind, label } = linkTypes[field];
  const [revision, retry] = useState(0);
  const result = useCustomerData<LinkedRow>(
    '/crm/workspaces/' + workspace.id + '/' + kind + '/' + id,
    revision,
  );
  if (!result) return <span role="status">Loading {label.toLowerCase()}…</span>;
  if (!result.data)
    return (
      <span>
        {label} unavailable.{' '}
        <button className="ws-text-button" onClick={() => retry((n) => n + 1)}>
          Retry
        </button>
      </span>
    );
  return (
    <a className="ws-text-button" href={(kind === 'deals' ? '#/sales/' : '#/customers/') + kind + '/' + id}>
      {result.data.name ?? result.data.title}
    </a>
  );
}
export function ActivityLinkPicker({
  field,
  value,
  initialLabel,
  onChange,
}: {
  field: LinkField;
  value: string;
  initialLabel?: string;
  onChange: (id: string) => void;
}) {
  const { workspace } = useCrm();
  const { kind, label } = linkTypes[field];
  const base = '/crm/workspaces/' + workspace.id + '/' + kind;
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(0);
  const [revision, retry] = useState(0);
  const [choice, setChoice] = useState<{ id: string; name: string } | null>(null);
  const chosenName = choice?.id === value ? choice.name : initialLabel;
  const current = useCustomerData<LinkedRow>(value && !chosenName ? base + '/' + value : null, revision);
  const params = new URLSearchParams({ q: query.trim(), limit: '16', offset: String(page * 15) });
  const result = useCustomerData<LinkedRow[]>(open ? base + '?' + params : null, revision, 150);
  function choose(row: LinkedRow) {
    setChoice({ id: row.id, name: row.name ?? row.title ?? label });
    onChange(row.id);
    setOpen(false);
  }
  return (
    <fieldset className="activity-link-field">
      <legend>{label}</legend>
      <div>
        {value && (
          <strong>
            {chosenName ??
              current?.data?.name ??
              current?.data?.title ??
              (current?.error ? 'Unavailable' : 'Loading…')}
          </strong>
        )}
        <button type="button" className="ws-pill" onClick={() => setOpen(!open)}>
          {open
            ? 'Close ' + label.toLowerCase() + ' search'
            : value
              ? 'Change ' + label.toLowerCase()
              : 'Link ' + label.toLowerCase()}
        </button>
        {value && (
          <button
            type="button"
            className="ws-text-button"
            onClick={() => {
              onChange('');
              setChoice(null);
              setOpen(false);
            }}
          >
            Remove {label.toLowerCase()}
          </button>
        )}
      </div>
      {open && (
        <div className="activity-link-search">
          <input
            type="search"
            aria-label={'Search ' + kind}
            placeholder={'Search ' + kind}
            maxLength={200}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setPage(0);
            }}
          />
          {!result ? (
            <small role="status">Loading…</small>
          ) : result.error != null ? (
            <p role="alert" className="customer-error">
              {errorMessage(result.error)}{' '}
              <button type="button" className="ws-text-button" onClick={() => retry((n) => n + 1)}>
                Retry
              </button>
            </p>
          ) : (
            <ul>
              {(result.data ?? []).slice(0, 15).map((row) => (
                <li key={row.id}>
                  <button type="button" className="ws-text-button" onClick={() => choose(row)}>
                    {row.name ?? row.title}
                  </button>
                </li>
              ))}
            </ul>
          )}
          {result?.data?.length === 0 && <small>No matching records.</small>}
          {(page > 0 || (result?.data?.length ?? 0) > 15) && (
            <div className="customer-pager">
              <button type="button" disabled={!page} onClick={() => setPage((n) => n - 1)}>
                Previous
              </button>
              <button
                type="button"
                disabled={(result?.data?.length ?? 0) <= 15}
                onClick={() => setPage((n) => n + 1)}
              >
                Next
              </button>
            </div>
          )}
        </div>
      )}
    </fieldset>
  );
}
