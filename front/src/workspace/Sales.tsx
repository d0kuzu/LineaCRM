import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import type { SalesLocation } from '../routes';
import { LeadForm } from './LeadForm';
import { amountLabel, probabilityLabel } from './DealParameters';
import { DealBoard } from './DealBoard';
import { DealFilterControls, emptyFilters, filterError, type DealFilters } from './DealFilters';
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
  ['pipeline', 'Board'],
  ['pipelines', 'Pipelines'],
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
  const kind = view === 'leads' ? 'leads' : 'deals';
  return (
    <SalesDirectory key={kind} kind={kind} board={view === 'pipeline'} id={location?.id} switcher={switcher} />
  );
}

function SalesDirectory({
  kind,
  board,
  id,
  switcher,
}: {
  kind: SalesKind;
  board: boolean;
  id?: string;
  switcher: ReactNode;
}) {
  const { workspace, profile } = useCrm();
  const { locale } = useLocale();
  const [query, setQuery] = useState('');
  const [filters, setFilters] = useState<DealFilters>(emptyFilters);
  const [boardDenied, setBoardDenied] = useState(false);
  const [boardStatus, setBoardStatus] = useState('');
  const [page, setPage] = useState(0);
  const [creating, setCreating] = useState(false);
  const [revision, setRevision] = useState(0);
  const [notice, setNotice] = useState('');
  const refresh = useCallback(() => setRevision((n) => n + 1), []);
  const unavailable = useCallback(() => {
    setBoardDenied(true);
    setBoardStatus('');
  }, []);
  const deniedOnCreate = useCallback(
    (cause: unknown) => {
      setCreating(false);
      setNotice(errorMessage(cause));
      refresh();
    },
    [refresh],
  );
  const base = '/crm/workspaces/' + workspace.id;
  const team = useCustomerData<Member[]>('/auth/workspaces/' + workspace.id + '/members', revision);
  const pipelines = useCustomerData<Pipeline[]>(kind === 'deals' ? base + '/pipelines' : null, revision);
  const selectedPipeline =
    pipelines?.data?.find((item) => item.id === filters.pipeline_id) ??
    (board && !filters.pipeline_id
      ? (pipelines?.data?.find((item) => item.stages.some((stage) => stage.id === filters.stage_id)) ??
        pipelines?.data?.find((item) => item.is_default))
      : undefined);
  const effectivePipeline = selectedPipeline?.id || filters.pipeline_id;
  useEffect(() => {
    if (board && !filters.pipeline_id && effectivePipeline) {
      setFilters((current) => ({ ...current, pipeline_id: effectivePipeline }));
    }
  }, [board, filters.pipeline_id, effectivePipeline]);
  const params = new URLSearchParams({ q: query.trim() });
  if (kind === 'deals') {
    for (const [key, value] of Object.entries({ ...filters, pipeline_id: effectivePipeline })) {
      if (value) params.set(key, value);
    }
  }
  const invalid = kind === 'deals' ? filterError(filters) : '';
  const listPath =
    !board && !invalid
      ? base + '/' + kind + '?' + params + '&limit=' + (pageSize + 1) + '&offset=' + page * pageSize
      : null;
  const list = useCustomerData<(Lead | Deal)[]>(listPath, revision, 160);
  const title = board ? 'Deal board' : kind === 'leads' ? 'Leads' : 'Deals';
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
    window.location.hash = board ? '/sales/pipeline' : salesLink(kind);
  }
  function retry() {
    setBoardDenied(false);
    closed();
    refresh();
  }
  function filterChanged(next: DealFilters) {
    setFilters(next);
    setPage(0);
    setBoardStatus('');
  }
  const dates = new Intl.DateTimeFormat(locale, { dateStyle: 'medium' });
  if (
    (board && boardDenied) ||
    accessError(list?.error) ||
    accessError(team?.error) ||
    accessError(pipelines?.error)
  )
    return (
      <>
        <PageTitle title={title} description={workspace.name} />
        <p className="ws-panel customer-error" role="alert">
          Your workspace access has changed.{' '}
          <button className="ws-text-button" onClick={retry}>
            Retry sales
          </button>
        </p>
      </>
    );
  return (
    <>
      <div className="customer-page-head">
        <PageTitle title={title} description={workspace.name} />
        {board && (
          <button className="ws-pill" onClick={refresh}>
            Refresh board
          </button>
        )}
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
            setBoardStatus('');
          }}
        />
        {kind === 'deals' && (
          <DealFilterControls
            filters={filters}
            pipelineID={effectivePipeline}
            board={board}
            pipelines={pipelines?.data ?? []}
            members={team?.data ?? []}
            onChange={filterChanged}
            onClear={() => {
              setQuery('');
              filterChanged(emptyFilters);
            }}
          />
        )}
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
      {board && boardStatus && (
        <p role="status" className="ws-subtext">
          {boardStatus}
        </p>
      )}
      {invalid ? (
        <p role="alert" className="ws-panel customer-error">
          {invalid}
        </p>
      ) : board ? (
        selectedPipeline ? (
          <DealBoard
            key={params.toString() + ':' + revision}
            pipeline={selectedPipeline}
            path={base + '/deals/board'}
            params={params.toString()}
            ownerLabel={ownerLabel}
            onDenied={unavailable}
            onMoved={(name) => {
              setBoardStatus('Deal moved to ' + name + '.');
              refresh();
            }}
            onRefresh={refresh}
            onOutdated={() => {
              setBoardStatus('The deals in this stage changed. The board has been refreshed.');
              refresh();
            }}
          />
        ) : !pipelines ? (
          <p className="ws-panel ws-empty" role="status">
            Loading pipelines…
          </p>
        ) : (
          !pipelines.error && (
            <p className="ws-panel ws-empty" role="status">
              Choose an available pipeline.
            </p>
          )
        )
      ) : (
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
              {query.trim() || page || Object.values(filters).some(Boolean)
                ? 'No matching records.'
                : 'No ' + kind + ' in this workspace yet.'}
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
      )}
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
