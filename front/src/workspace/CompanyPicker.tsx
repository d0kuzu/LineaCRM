import { useEffect, useState } from 'react';
import { useCrm } from '../crm/session';
import { accessError, errorMessage, useCustomerData, type Customer } from './customerData';

export function CompanyPicker({ value, onChange, currentName, label = 'Company', required, onAccessError }: {
  value: string; onChange: (id: string) => void; currentName?: string; label?: string; required?: boolean; onAccessError?: (error: unknown) => void;
}) {
  const { workspace } = useCrm();
  const [query, setQuery] = useState('');
  const [offset, setOffset] = useState(0);
  const [revision, setRevision] = useState(0);
  const [choice, setChoice] = useState<{ id: string; name: string } | null>(null);
  const params = new URLSearchParams({ q: query.trim(), offset: String(offset), limit: '21' });
  const companies = useCustomerData<Customer[]>('/crm/workspaces/' + workspace.id + '/companies?' + params, revision, 180);
  useEffect(() => {
    if (accessError(companies?.error)) onAccessError?.(companies?.error);
  }, [companies?.error, onAccessError]);
  if (accessError(companies?.error)) return <p role="alert" className="customer-error">Company directory unavailable.</p>;
  const rows = companies?.data?.slice(0, 20) ?? [];
  const selectedName = choice?.id === value ? choice.name : currentName;
  return <fieldset className="customer-company-picker"><legend>{label}{required && <span aria-hidden="true"> *</span>}</legend>
    <input aria-label="Search companies to link" type="search" value={query} placeholder="Find a company"
      onChange={event => { setQuery(event.target.value); setOffset(0); }} />
    <select aria-label={label} required={required} value={value} onChange={event => {
      const id = event.target.value;
      setChoice(rows.find(row => row.id === id) ?? null);
      onChange(id);
    }}>
      <option value="">No company selected</option>
      {value && !rows.some(row => row.id === value) && <option value={value}>{selectedName || value}</option>}
      {rows.map(row => <option key={row.id} value={row.id}>{row.name}</option>)}
    </select>
    {!companies && <small role="status">Loading companies…</small>}
    {companies?.error != null && <small role="alert">{errorMessage(companies.error)} <button type="button"
      className="ws-text-button" onClick={() => setRevision(n => n + 1)}>Retry</button></small>}
    {companies?.data?.length === 0 && <small>No matching companies.</small>}
    {(offset > 0 || (companies?.data?.length ?? 0) > 20) && <div className="customer-pager">
      <button type="button" disabled={!offset} onClick={() => setOffset(n => Math.max(0, n - 20))}>Previous companies</button>
      <button type="button" disabled={(companies?.data?.length ?? 0) <= 20} onClick={() => setOffset(n => n + 20)}>More companies</button>
    </div>}
  </fieldset>;
}
