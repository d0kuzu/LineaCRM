import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import {
  accessError,
  customerLink,
  errorMessage,
  timestamp,
  useCustomerData,
  type Customer,
} from './customerData';
import { DealLinkFields, LeadForm, type LinkedContact } from './LeadForm';
import { MoveDeal } from './DealStage';
import { canChangeOwner, ChangeDealOwner } from './SalesOwner';
import { EditDealParameters, amountLabel, probabilityLabel } from './DealParameters';
import { RecordHistory } from './RecordHistory';
import {
  dealLocation,
  salesLink,
  statusLabels,
  type Deal,
  type Lead,
  type Pipeline,
  type SalesKind,
} from './salesData';
import { Fields, RecordDialog } from './shared';

function RelatedRecord({
  kind,
  id,
  onUnavailable,
}: {
  kind: 'leads' | 'companies' | 'contacts';
  id: string;
  onUnavailable: (related: string) => void;
}) {
  const { workspace } = useCrm();
  const [revision, setRevision] = useState(0);
  const result = useCustomerData<{ name?: string; title?: string }>(
    '/crm/workspaces/' + workspace.id + '/' + kind + '/' + id,
    revision,
  );
  useEffect(() => {
    if (accessError(result?.error)) onUnavailable(kind + '/' + id);
  }, [result?.error, onUnavailable, kind, id]);
  if (!result) return <span role="status">Loading…</span>;
  if (!result.data)
    return (
      <span role="alert">
        Record unavailable.{' '}
        <button className="ws-text-button" onClick={() => setRevision((n) => n + 1)}>
          Retry
        </button>
      </span>
    );
  return (
    <a className="ws-text-button" href={kind === 'leads' ? salesLink(kind, id) : customerLink({ kind, id })}>
      {result.data.name ?? result.data.title}
    </a>
  );
}

function DealRelations({ deal, onUnavailable }: { deal: Deal; onUnavailable: (related: string) => void }) {
  const [page, setPage] = useState(0);
  const ids = deal.contact_ids ?? [];
  return (
    <section className="sales-relations">
      <h3>Source lead</h3>
      {deal.lead_id ? (
        <RelatedRecord kind="leads" id={deal.lead_id} onUnavailable={onUnavailable} />
      ) : (
        <p className="ws-subtext">Source lead unavailable.</p>
      )}
      <h3>Company</h3>
      {deal.company_id ? (
        <RelatedRecord kind="companies" id={deal.company_id} onUnavailable={onUnavailable} />
      ) : (
        <p className="ws-subtext">No visible company link.</p>
      )}
      <h3>Contacts</h3>
      {ids.length ? (
        <ul className="customer-linked">
          {ids.slice(page * 10, page * 10 + 10).map((id) => (
            <li key={id}>
              <RelatedRecord kind="contacts" id={id} onUnavailable={onUnavailable} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="ws-subtext">No visible contact links.</p>
      )}
      {ids.length > 10 && (
        <div className="customer-pager">
          <button disabled={!page} onClick={() => setPage((n) => n - 1)}>
            Previous contacts
          </button>
          <button disabled={(page + 1) * 10 >= ids.length} onClick={() => setPage((n) => n + 1)}>
            More contacts
          </button>
        </div>
      )}
    </section>
  );
}

function EditDealLinks({
  deal,
  onSaved,
  onCancel,
  onDenied,
}: {
  deal: Deal;
  onSaved: () => void;
  onCancel: () => void;
  onDenied: (error: unknown) => void;
}) {
  const { request, workspace } = useCrm();
  const [company, setCompany] = useState(deal.company_id ?? '');
  const [contacts, setContacts] = useState<LinkedContact[]>(() =>
    (deal.contact_ids ?? []).map((id) => ({ id })),
  );
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  const base = '/crm/workspaces/' + workspace.id;
  const originalCompany = useCustomerData<Customer>(
    deal.company_id ? base + '/companies/' + deal.company_id : null,
  );
  useEffect(() => {
    if (accessError(originalCompany?.error)) onDenied(originalCompany?.error);
  }, [originalCompany?.error, onDenied]);
  useEffect(() => () => active.current?.abort(), []);
  async function save(event: FormEvent) {
    event.preventDefault();
    if (active.current) return;
    const before = new Set(deal.contact_ids);
    const after = new Set(contacts.map((contact) => contact.id));
    const add = [...after].filter((id) => !before.has(id)),
      remove = [...before].filter((id) => !after.has(id));
    const changes: { company_id?: string | null; add_contact_ids?: string[]; remove_contact_ids?: string[] } =
      {};
    if (company !== (deal.company_id ?? '')) changes.company_id = company || null;
    if (add.length) changes.add_contact_ids = add;
    if (remove.length) changes.remove_contact_ids = remove;
    if (!Object.keys(changes).length) {
      onCancel();
      return;
    }
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      await request<Deal>(base + '/deals/' + deal.id, {
        method: 'PATCH',
        body: JSON.stringify(changes),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) onSaved();
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onDenied(cause);
        else setError(errorMessage(cause));
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <form onSubmit={save} className="customer-form sales-form">
      <fieldset className="customer-field-grid" disabled={busy}>
        <DealLinkFields
          company={company}
          companyName={originalCompany?.data?.name}
          setCompany={setCompany}
          contacts={contacts}
          setContacts={setContacts}
          onDenied={onDenied}
        />
      </fieldset>
      {error && (
        <p role="alert" className="customer-error">
          {error}
        </p>
      )}
      <div className="customer-actions">
        <button className="ws-action" disabled={busy}>
          {busy ? 'Saving…' : 'Save links'}
        </button>
        <button className="ws-pill" type="button" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}

export function SalesCard({
  kind,
  id,
  ownerLabel,
  resume,
  pipelines,
  onClose,
  onChanged,
}: {
  kind: SalesKind;
  id: string;
  ownerLabel: (owner: string) => string;
  resume: number;
  pipelines?: Pipeline[];
  onClose: () => void;
  onChanged: () => void;
}) {
  const { workspace } = useCrm();
  const { locale } = useLocale();
  const [mode, setMode] = useState<'view' | 'edit' | 'convert' | 'parameters' | 'move' | 'owner'>('view');
  const [editStarted, setEditStarted] = useState(resume);
  const [accessRetry, setAccessRetry] = useState(0);
  const [revision, setRevision] = useState(0);
  const [denied, setDenied] = useState(false);
  const rechecked = useRef(new Set<string>());
  const base = '/crm/workspaces/' + workspace.id;
  const path = base + '/' + kind + '/' + id;
  const result = useCustomerData<Lead | Deal>(
    denied ? null : path,
    revision + (mode === 'view' ? resume : editStarted),
  );
  // Keep the original values for an active draft, and verify its access separately on return.
  const checkDraft = mode !== 'view' && resume !== editStarted;
  const permission = useCustomerData<Lead | Deal>(checkDraft && !denied ? path : null, resume + accessRetry);
  const convertedWhileEditing = permission?.data && 'converted' in permission.data && permission.data.converted;
  const onDenied = useCallback(() => {
    setMode('view');
    setDenied(true);
    onChanged();
  }, [onChanged]);
  useEffect(() => {
    if (accessError(result?.error)) onDenied();
  }, [result?.error, onDenied]);
  useEffect(() => {
    if (accessError(permission?.error)) onDenied();
    else if (convertedWhileEditing) setMode('view');
  }, [permission?.error, convertedWhileEditing, onDenied]);
  function startEditing(next: Exclude<typeof mode, 'view'>) {
    setEditStarted(resume);
    setMode(next);
  }
  const onRelationUnavailable = useCallback(
    (related: string) => {
      // Recheck the deal itself once if a related record changes visibility between reads.
      if (rechecked.current.has(related)) return;
      rechecked.current.add(related);
      setRevision((n) => n + 1);
      onChanged();
    },
    [onChanged],
  );
  function reload() {
    rechecked.current.clear();
    setMode('view');
    setDenied(false);
    setRevision((n) => n + 1);
    onChanged();
  }
  const row = result?.data,
    lead = row && 'deal_id' in row ? row : null,
    deal = row && 'lead_id' in row ? row : null;
  const canWrite = workspace.role !== 'support';
  const location = deal && dealLocation(deal, pipelines);
  const titles = {
    view: row?.title,
    convert: 'Convert lead',
    edit: lead ? 'Edit lead' : 'Edit deal links',
    parameters: 'Edit deal parameters',
    move: 'Move deal',
    owner: 'Change owner',
  };
  const title = (!denied && titles[mode]) || (kind === 'leads' ? 'Lead' : 'Deal');
  return (
    <RecordDialog title={title} onClose={onClose}>
      {checkDraft && !permission && <p role="status">Checking record access…</p>}
      {checkDraft && permission?.error != null && !accessError(permission.error) && (
        <p role="alert" className="customer-error">
          {errorMessage(permission.error)}{' '}
          <button className="ws-text-button" onClick={() => setAccessRetry((n) => n + 1)}>
            Retry access check
          </button>
        </p>
      )}
      <div className="customer-card" hidden={checkDraft && (!permission?.data || !!convertedWhileEditing)}>
        {denied || result?.error != null ? (
          <p role="alert" className="customer-error">
            {denied || accessError(result?.error)
              ? 'This record or one of its links is no longer available.'
              : errorMessage(result?.error)}{' '}
            <button className="ws-text-button" onClick={reload}>
              Reload record
            </button>
          </p>
        ) : !row ? (
          <p role="status">Loading record…</p>
        ) : canWrite && mode !== 'view' && lead && !lead.converted ? (
          <LeadForm
            lead={lead}
            convert={mode === 'convert'}
            onCancel={() => setMode('view')}
            onDenied={onDenied}
            onReload={reload}
            onSaved={(nextKind, nextID) => {
              reload();
              window.location.hash = salesLink(nextKind, nextID);
            }}
          />
        ) : mode === 'owner' && deal && canChangeOwner(workspace.role) ? (
          <ChangeDealOwner
            deal={deal}
            ownerLabel={ownerLabel}
            onSaved={reload}
            onCancel={() => setMode('view')}
            onDenied={onDenied}
          />
        ) : canWrite && mode === 'edit' && deal ? (
          <EditDealLinks deal={deal} onSaved={reload} onCancel={() => setMode('view')} onDenied={onDenied} />
        ) : canWrite && mode === 'move' && deal ? (
          <MoveDeal deal={deal} onSaved={reload} onCancel={() => setMode('view')} onDenied={onDenied} />
        ) : canWrite && mode === 'parameters' && deal ? (
          <EditDealParameters
            deal={deal}
            onSaved={reload}
            onCancel={() => setMode('view')}
            onDenied={onDenied}
          />
        ) : (
          <>
            <Fields
              rows={[
                ['Owner', ownerLabel(row.owner_id)],
                ['Created', timestamp(row.created_at, locale)],
                ...(lead
                  ? ([
                      ['Status', lead.converted ? 'Converted' : 'Open'],
                      ['Updated', timestamp(lead.updated_at, locale)],
                    ] as [string, string][])
                  : []),
                ...(deal && location
                  ? ([
                      ['Amount', amountLabel(deal, locale)],
                      ['Expected close date', deal.expected_close_date ?? '—'],
                      ['Probability', probabilityLabel(deal.probability)],
                      ['Pipeline', location.pipeline],
                      ['Stage', location.stage],
                      ['Status', statusLabels[deal.status]],
                    ] as [string, string][])
                  : []),
              ]}
            />
            {lead && (
              <>
                {lead.description && (
                  <section>
                    <h3>Description</h3>
                    <p className="sales-description">{lead.description}</p>
                  </section>
                )}
                <div className="customer-actions">
                  {lead.deal_id ? (
                    <a className="ws-action" href={salesLink('deals', lead.deal_id)}>
                      Open deal
                    </a>
                  ) : lead.converted ? (
                    <span className="ws-subtext">Converted deal unavailable.</span>
                  ) : (
                    canWrite && (
                      <>
                        <button className="ws-action" onClick={() => startEditing('convert')}>
                          Convert to deal
                        </button>
                        <button className="ws-pill" onClick={() => startEditing('edit')}>
                          Edit lead
                        </button>
                      </>
                    )
                  )}
                </div>
              </>
            )}
            {deal && (
              <>
                {canWrite && (
                  <div className="customer-actions">
                    <button className="ws-action" onClick={() => startEditing('move')}>
                      Move deal
                    </button>
                    <button className="ws-pill" onClick={() => startEditing('parameters')}>
                      Edit parameters
                    </button>
                    <button className="ws-pill" onClick={() => startEditing('edit')}>
                      Edit links
                    </button>
                    {canChangeOwner(workspace.role) && (
                      <button className="ws-pill" onClick={() => startEditing('owner')}>
                        Change owner
                      </button>
                    )}
                  </div>
                )}
                <DealRelations deal={deal} onUnavailable={onRelationUnavailable} />
                <RecordHistory
                  kind="deal"
                  recordPath={base + '/deals/' + deal.id}
                  participant={ownerLabel}
                  onAccessError={onDenied}
                />
              </>
            )}
          </>
        )}
      </div>
    </RecordDialog>
  );
}
