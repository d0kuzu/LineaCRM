import { useCallback, useState, type ReactNode } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import type { SalesLocation } from '../routes';
import { LeadForm } from './LeadForm';
import { amountLabel, probabilityLabel } from './DealParameters';
import { PipelinePreview } from './PipelinePreview';
import { PipelineSettings } from './PipelineSettings';
import { SalesCard } from './SalesCard';
import { accessError, errorMessage, useCustomerData, type Member } from './customerData';
import {
  dealLocation,
  salesLink,
  statusLabels,
  type Deal,
  type Lead,
  type Pipeline,
  type SalesKind,
} from './salesData';
import { PageTitle, RecordDialog, Switcher } from './shared';
import './customers.css';
import './sales.css';

const tabs = [
  ['leads', 'Leads'],
  ['deals', 'Deals'],
  ['pipelines', 'Pipelines'],
  ['pipeline', 'Pipeline demo'],
] as const;
const pageSize = 20;

export function Sales({ location }: { location?: SalesLocation }) {
  const view = location?.view ?? 'leads';
  const switcher = (
    <Switcher<SalesLocation['view']>
      values={tabs}
      selected={view}
      label="Sales views"
      onSelect={(next) => {
        window.location.hash = '/sales/' + next;
      }}
    />
  );
  if (view === 'pipelines') return <PipelineSettings switcher={switcher} />;
  if (view === 'pipeline')
    return (
      <>
        <div className="ws-toolbar">{switcher}</div>
        <PipelinePreview />
      </>
    );
  return <SalesDirectory key={view} kind={view} id={location?.id} switcher={switcher} />;
}

function SalesDirectory({ kind, id, switcher }: { kind: SalesKind; id?: string; switcher: ReactNode }) {
  const { workspace, profile } = useCrm();
  const { locale } = useLocale();
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(0);
  const [creating, setCreating] = useState(false);
  const [revision, setRevision] = useState(0);
  const [notice, setNotice] = useState('');
  const refresh = useCallback(() => setRevision((n) => n + 1), []);
  const deniedOnCreate = useCallback(
    (cause: unknown) => {
      setCreating(false);
      setNotice(errorMessage(cause));
      refresh();
    },
    [refresh],
  );
  const base = '/crm/workspaces/' + workspace.id;
  const params = new URLSearchParams({
    q: query.trim(),
    limit: String(pageSize + 1),
    offset: String(page * pageSize),
  });
  const list = useCustomerData<(Lead | Deal)[]>(base + '/' + kind + '?' + params, revision, 160);
  const team = useCustomerData<Member[]>('/auth/workspaces/' + workspace.id + '/members', revision);
  const pipelines = useCustomerData<Pipeline[]>(kind === 'deals' ? base + '/pipelines' : null, revision);
  const title = kind === 'leads' ? 'Leads' : 'Deals';
  const canCreate = kind === 'leads' && workspace.role !== 'support';
  const items = list?.data ?? [];
  function ownerLabel(owner: string) {
    return (
      team?.data?.find((member) => member.user_id === owner)?.email ||
      (owner === profile.id && profile.email) ||
      owner
    );
  }
  function closed() {
    setCreating(false);
    refresh();
    window.location.hash = salesLink(kind);
  }
  const dates = new Intl.DateTimeFormat(locale, { dateStyle: 'medium' });
  if (accessError(list?.error) || accessError(team?.error) || accessError(pipelines?.error))
    return (
      <>
        <PageTitle title={title} description={workspace.name} />
        <p className="ws-panel customer-error" role="alert">
          Your workspace access has changed.{' '}
          <button className="ws-text-button" onClick={closed}>
            Retry sales
          </button>
        </p>
      </>
    );
  return (
    <>
      <div className="customer-page-head">
        <PageTitle title={title} description={workspace.name} />
        {canCreate && (
          <button
            className="ws-action"
            onClick={() => {
              setCreating(true);
              setNotice('');
            }}
          >
            New lead
          </button>
        )}
      </div>
      <div className="ws-toolbar customer-toolbar">
        {switcher}
        <input
          type="search"
          className="ws-search"
          aria-label={'Search ' + kind}
          placeholder={'Search ' + kind}
          value={query}
          maxLength={200}
          onChange={(event) => {
            setQuery(event.target.value);
            setPage(0);
          }}
        />
      </div>
      {notice && (
        <p role="alert" className="customer-error">
          {notice}
        </p>
      )}
      {team?.error != null && (
        <p role="alert" className="customer-error">
          Could not load team members.{' '}
          <button className="ws-text-button" onClick={refresh}>
            Retry members
          </button>
        </p>
      )}
      {pipelines?.error != null && (
        <p role="alert" className="customer-error">
          Could not load pipelines.{' '}
          <button className="ws-text-button" onClick={refresh}>
            Retry pipelines
          </button>
        </p>
      )}
      <section className="ws-panel ws-table-panel">
        <div className="ws-section-head">
          <h2>{kind === 'leads' ? 'Lead directory' : 'Deal directory'}</h2>
          {items.length > 0 && (
            <small>
              {(page * pageSize + 1).toLocaleString(locale)}–
              {(page * pageSize + Math.min(pageSize, items.length)).toLocaleString(locale)}
            </small>
          )}
        </div>
        {!list ? (
          <p className="ws-empty" role="status">
            Loading {kind}…
          </p>
        ) : list.error != null ? (
          <p className="customer-error" role="alert">
            {errorMessage(list.error)}{' '}
            <button className="ws-text-button" onClick={refresh}>
              Retry list
            </button>
          </p>
        ) : !items.length ? (
          <p role="status" className="ws-empty">
            {query.trim() || page ? 'No matching records.' : 'No ' + kind + ' in this workspace yet.'}
          </p>
        ) : (
          <div className="ws-table-scroll">
            <table className="ws-table customer-table">
              <thead>
                <tr>
                  <th>{kind === 'leads' ? 'Lead' : 'Deal'}</th>
                  <th>Owner</th>
                  {kind === 'deals' && (
                    <>
                      <th>Amount</th>
                      <th>Expected close date</th>
                      <th>Probability</th>
                      <th>Pipeline / stage</th>
                      <th>Status</th>
                    </>
                  )}
                  <th>{kind === 'leads' ? 'Status' : 'Source'}</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {items.slice(0, pageSize).map((row) => {
                  const location =
                    'stage_id' in row ? dealLocation(row, pipelines?.error ? [] : pipelines?.data) : null;
                  return (
                    <tr key={row.id}>
                      <td>
                        <a className="ws-text-button" href={salesLink(kind, row.id)}>
                          {row.title}
                        </a>
                      </td>
                      <td>{ownerLabel(row.owner_id)}</td>
                      {'lead_id' in row && (
                        <>
                          <td>{amountLabel(row, locale)}</td>
                          <td>{row.expected_close_date ?? '—'}</td>
                          <td>{probabilityLabel(row.probability)}</td>
                          <td>
                            {location?.pipeline} / {location?.stage}
                          </td>
                          <td>{statusLabels[row.status]}</td>
                        </>
                      )}
                      <td>
                        {'deal_id' in row ? (
                          row.deal_id ? (
                            'Converted'
                          ) : (
                            'Open'
                          )
                        ) : (
                          <a className="customer-relation-link" href={salesLink('leads', row.lead_id)}>
                            View lead
                          </a>
                        )}
                      </td>
                      <td>
                        <time dateTime={row.created_at}>{dates.format(new Date(row.created_at))}</time>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        {(page > 0 || items.length > pageSize) && (
          <nav className="customer-pager" aria-label="Sales pages">
            <button className="ws-pill" disabled={!page} onClick={() => setPage((n) => n - 1)}>
              Previous
            </button>
            <span>Page {(page + 1).toLocaleString(locale)}</span>
            <button
              className="ws-pill"
              disabled={items.length <= pageSize}
              onClick={() => setPage((n) => n + 1)}
            >
              Next
            </button>
          </nav>
        )}
      </section>
      {id ? (
        <SalesCard
          key={id}
          kind={kind}
          id={id}
          pipelines={pipelines?.error ? [] : pipelines?.data}
          ownerLabel={ownerLabel}
          onClose={closed}
          onChanged={refresh}
        />
      ) : (
        creating &&
        canCreate && (
          <RecordDialog title="New lead" onClose={() => setCreating(false)}>
            <div className="customer-card">
              <LeadForm
                onCancel={() => setCreating(false)}
                onDenied={deniedOnCreate}
                onSaved={(nextKind, nextId) => {
                  setCreating(false);
                  refresh();
                  window.location.hash = salesLink(nextKind, nextId);
                }}
              />
            </div>
          </RecordDialog>
        )
      )}
    </>
  );
}
