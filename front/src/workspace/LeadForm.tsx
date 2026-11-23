import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { ApiError } from '../crm/http';
import { CompanyPicker } from './CompanyPicker';
import { DealParameterFields, parameterChanges, parameterDraft } from './DealParameters';
import { accessError, errorMessage, useCustomerData, type Customer, type Member } from './customerData';
import type { Lead, SalesKind } from './salesData';

export type LinkedContact = { id: string; name?: string };

function SelectedContact({
  contact,
  onRemove,
  onDenied,
}: {
  contact: LinkedContact;
  onRemove: () => void;
  onDenied: (error: unknown) => void;
}) {
  const { workspace } = useCrm();
  const lookup = useCustomerData<Customer>(
    contact.name ? null : '/crm/workspaces/' + workspace.id + '/contacts/' + contact.id,
  );
  useEffect(() => {
    if (accessError(lookup?.error)) onDenied(lookup?.error);
  }, [lookup?.error, onDenied]);
  const label = contact.name ?? lookup?.data?.name;
  if (lookup?.error != null) return <li role="alert">Contact unavailable</li>;
  return (
    <li>
      {label ?? 'Loading contact…'}{' '}
      <button type="button" aria-label={'Remove ' + (label ?? 'contact')} onClick={onRemove}>
        ×
      </button>
    </li>
  );
}

function ContactPicker({
  selected,
  onChange,
  onDenied,
}: {
  selected: LinkedContact[];
  onChange: (value: LinkedContact[]) => void;
  onDenied: (error: unknown) => void;
}) {
  const { workspace } = useCrm();
  const [q, setQ] = useState('');
  const [offset, setOffset] = useState(0);
  const [selectedPage, setSelectedPage] = useState(0);
  const [revision, setRevision] = useState(0);
  const params = new URLSearchParams({ q: q.trim(), limit: '11', offset: String(offset) });
  const result = useCustomerData<Customer[]>(
    '/crm/workspaces/' + workspace.id + '/contacts?' + params,
    revision,
    160,
  );
  useEffect(() => {
    if (accessError(result?.error)) onDenied(result?.error);
  }, [result?.error, onDenied]);
  if (accessError(result?.error)) return <p role="alert">Contact directory unavailable.</p>;
  const lastPage = Math.max(0, Math.ceil(selected.length / 10) - 1);
  const page = Math.min(selectedPage, lastPage);
  return (
    <fieldset className="sales-contact-picker customer-wide">
      <legend>Contacts</legend>
      {selected.length > 0 && (
        <ul className="sales-selected">
          {selected.slice(page * 10, page * 10 + 10).map((contact) => (
            <SelectedContact
              key={contact.id}
              contact={contact}
              onDenied={onDenied}
              onRemove={() => onChange(selected.filter((v) => v.id !== contact.id))}
            />
          ))}
        </ul>
      )}
      {lastPage > 0 && (
        <div className="customer-pager">
          <button type="button" disabled={!page} onClick={() => setSelectedPage(page - 1)}>
            Previous selected
          </button>
          <button type="button" disabled={page >= lastPage} onClick={() => setSelectedPage(page + 1)}>
            More selected
          </button>
        </div>
      )}
      <input
        type="search"
        value={q}
        aria-label="Search contacts to link"
        placeholder="Find contacts"
        maxLength={200}
        onChange={(e) => {
          setQ(e.target.value);
          setOffset(0);
        }}
      />
      {!result ? (
        <small role="status">Loading contacts…</small>
      ) : result.error != null ? (
        <p role="alert" className="customer-error">
          {errorMessage(result.error)}{' '}
          <button type="button" className="ws-text-button" onClick={() => setRevision((n) => n + 1)}>
            Retry contacts
          </button>
        </p>
      ) : result.data?.length ? (
        <div className="sales-options">
          {result.data.slice(0, 10).map((contact) => {
            const checked = selected.some((item) => item.id === contact.id);
            return (
              <label key={contact.id}>
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={!checked && selected.length >= 100}
                  onChange={(e) =>
                    onChange(
                      e.target.checked
                        ? [...selected, { id: contact.id, name: contact.name }]
                        : selected.filter((item) => item.id !== contact.id),
                    )
                  }
                />
                <span>
                  {contact.name}
                  {contact.email && <small>{contact.email}</small>}
                </span>
              </label>
            );
          })}
        </div>
      ) : (
        <small>No matching contacts.</small>
      )}
      {selected.length >= 100 && <small role="status">100 contacts selected.</small>}
      {(offset > 0 || (result?.data?.length ?? 0) > 10) && (
        <div className="customer-pager">
          <button type="button" disabled={!offset} onClick={() => setOffset((n) => n - 10)}>
            Previous contacts
          </button>
          <button
            type="button"
            disabled={(result?.data?.length ?? 0) <= 10}
            onClick={() => setOffset((n) => n + 10)}
          >
            More contacts
          </button>
        </div>
      )}
    </fieldset>
  );
}

export function DealLinkFields({
  company,
  companyName,
  setCompany,
  contacts,
  setContacts,
  onDenied,
}: {
  company: string;
  companyName?: string;
  setCompany: (id: string) => void;
  contacts: LinkedContact[];
  setContacts: (values: LinkedContact[]) => void;
  onDenied: (error: unknown) => void;
}) {
  return (
    <>
      <CompanyPicker value={company} currentName={companyName} onChange={setCompany} onAccessError={onDenied} />
      <ContactPicker selected={contacts} onChange={setContacts} onDenied={onDenied} />
    </>
  );
}

export function LeadForm({
  lead,
  convert = false,
  onSaved,
  onCancel,
  onDenied,
  onReload,
}: {
  lead?: Lead;
  convert?: boolean;
  onSaved: (kind: SalesKind, id: string) => void;
  onCancel: () => void;
  onDenied: (error: unknown) => void;
  onReload?: () => void;
}) {
  const { request, workspace, profile } = useCrm();
  const canAssign = !convert && (workspace.role === 'admin' || workspace.role === 'head_of_sales');
  const [title, setTitle] = useState(lead?.title ?? '');
  const [description, setDescription] = useState(lead?.description ?? '');
  const [owner, setOwner] = useState(lead?.owner_id ?? profile.id);
  const [company, setCompany] = useState('');
  const [parameters, setParameters] = useState(() => parameterDraft());
  const [contacts, setContacts] = useState<LinkedContact[]>([]);
  const [teamRevision, setTeamRevision] = useState(0);
  const roster = useCustomerData<Member[]>(
    canAssign ? '/auth/workspaces/' + workspace.id + '/members' : null,
    teamRevision,
  );
  const team = (roster?.data ?? []).filter((member) =>
    ['admin', 'head_of_sales', 'sales_manager'].includes(member.role),
  );
  const [error, setError] = useState('');
  const [conflict, setConflict] = useState(false);
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (accessError(roster?.error)) onDenied(roster?.error);
  }, [roster?.error, onDenied]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (active.current) return;
    const name = title.trim();
    if (!name || Array.from(name).length > 200) {
      setError('Enter a title of 1 to 200 characters.');
      return;
    }
    const text = description.trim();
    if (!convert && Array.from(text).length > 4000) {
      setError('Description must be 4,000 characters or fewer.');
      return;
    }
    const payload: Record<string, string | string[] | number | null> = {};
    if (!lead || convert || name !== lead.title) payload.title = name;
    if (convert) {
      try {
        Object.assign(payload, parameterChanges(parameters));
      } catch (cause) {
        setError(errorMessage(cause));
        return;
      }
      if (company) payload.company_id = company;
      if (contacts.length) payload.contact_ids = contacts.map((contact) => contact.id);
    } else {
      if ((!lead && text) || (lead && text !== lead.description)) payload.description = text;
      if (canAssign && owner !== (lead?.owner_id ?? profile.id)) payload.owner_id = owner;
    }
    if (!Object.keys(payload).length) {
      onCancel();
      return;
    }
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    setConflict(false);
    try {
      const base = '/crm/workspaces/' + workspace.id + '/leads';
      const url = base + (lead ? '/' + lead.id : '') + (convert ? '/convert' : '');
      const result = await request<{ id: string }>(url, {
        method: lead && !convert ? 'PATCH' : 'POST',
        body: JSON.stringify(payload),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) onSaved(convert ? 'deals' : 'leads', result.id);
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onDenied(cause);
        else {
          setError(errorMessage(cause));
          setConflict(cause instanceof ApiError && cause.status === 409);
        }
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  return (
    <form onSubmit={submit} className="customer-form sales-form">
      <fieldset disabled={busy} className="customer-field-grid">
        <label className="customer-wide">
          {convert ? 'Deal title' : 'Lead title'}
          <input autoFocus required value={title} onChange={(e) => setTitle(e.target.value)} />
        </label>
        {convert ? (
          <>
            <DealParameterFields draft={parameters} setDraft={setParameters} />
            <DealLinkFields
              company={company}
              setCompany={setCompany}
              contacts={contacts}
              setContacts={setContacts}
              onDenied={onDenied}
            />
          </>
        ) : (
          <>
            <label className="customer-wide">
              Description
              <textarea
                aria-label="Description"
                rows={4}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </label>
            {canAssign && (
              <label className="customer-wide">
                Owner
                <select
                  aria-label="Owner"
                  value={owner}
                  disabled={!roster?.data}
                  onChange={(e) => setOwner(e.target.value)}
                >
                  {!team.some((member) => member.user_id === owner) && (
                    <option value={owner}>{owner === profile.id ? profile.email : owner}</option>
                  )}
                  {team.map((member) => (
                    <option key={member.user_id} value={member.user_id}>
                      {member.email || member.user_id}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {canAssign && !roster && <small role="status">Loading team members…</small>}
            {roster?.error != null && (
              <p role="alert" className="customer-error">
                Could not load team members.{' '}
                <button type="button" className="ws-text-button" onClick={() => setTeamRevision((n) => n + 1)}>
                  Retry members
                </button>
              </p>
            )}
          </>
        )}
      </fieldset>
      {error && (
        <p role="alert" className="customer-error">
          {error}{' '}
          {conflict && onReload && (
            <button type="button" className="ws-text-button" onClick={onReload}>
              Reload lead
            </button>
          )}
        </p>
      )}
      <div className="customer-actions">
        <button type="submit" className="ws-action" disabled={busy || conflict}>
          {busy ? 'Saving…' : convert ? 'Create deal' : lead ? 'Save changes' : 'Create lead'}
        </button>
        <button type="button" className="ws-pill" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}
