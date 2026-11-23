import { useState } from 'react';
import { canChangeOwner } from './SalesOwner';
import { errorMessage, type Member } from './customerData';

export function AccessNotice({
  loading,
  error,
  retry,
}: {
  loading: boolean;
  error: unknown;
  retry: () => void;
}) {
  if (loading) return <small role="status">Loading team members…</small>;
  if (error != null)
    return (
      <p className="customer-error" role="alert">
        {errorMessage(error)}{' '}
        <button type="button" className="ws-text-button" onClick={retry}>
          Retry members
        </button>
      </p>
    );
  return null;
}
export function MemberOptions({
  caption,
  id,
  setId,
  members,
}: {
  caption: string;
  id: string;
  setId: (id: string) => void;
  members?: Member[];
}) {
  const eligible = (members ?? []).filter((m) => canChangeOwner(m.role) || m.role === 'sales_manager');
  return (
    <label>
      {caption}
      <select aria-label={caption} value={id} disabled={!members} onChange={(e) => setId(e.target.value)}>
        {!eligible.some((m) => m.user_id === id) && (
          <option value={id} disabled>
            {members?.find((m) => m.user_id === id)?.email ?? id}
          </option>
        )}
        {eligible.map((m) => (
          <option key={m.user_id} value={m.user_id}>
            {m.email || m.user_id}
          </option>
        ))}
      </select>
    </label>
  );
}
export function Participants({
  owner,
  members,
  selected,
  onChange,
}: {
  owner: string;
  members?: Member[];
  selected: string[];
  onChange: (ids: string[]) => void;
}) {
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(0);
  const list = (members ?? []).filter(
    (m) =>
      m.user_id !== owner &&
      !selected.includes(m.user_id) &&
      (m.email || m.user_id).toLowerCase().includes(query.toLowerCase().trim()),
  );
  const last = Math.max(0, Math.ceil(list.length / 10) - 1),
    shown = Math.min(last, page);
  return (
    <fieldset className="customer-wide activity-participants">
      <legend>Participants</legend>
      {selected.length > 0 && (
        <ul className="sales-selected">
          {selected.map((id) => {
            const label = members?.find((m) => m.user_id === id)?.email ?? id;
            return (
              <li key={id}>
                {label}{' '}
                <button
                  type="button"
                  aria-label={'Remove participant ' + label}
                  onClick={() => onChange(selected.filter((value) => value !== id))}
                >
                  ×
                </button>
              </li>
            );
          })}
        </ul>
      )}
      <input
        type="search"
        placeholder="Find a team member"
        aria-label="Search participants"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setPage(0);
        }}
      />
      <div className="activity-participant-options">
        {list.slice(shown * 10, shown * 10 + 10).map((m) => (
          <button
            key={m.user_id}
            type="button"
            className="ws-pill"
            disabled={selected.length >= 100}
            aria-label={'Add participant ' + (m.email || m.user_id)}
            onClick={() => onChange([...selected, m.user_id])}
          >
            {m.email || m.user_id}
          </button>
        ))}
      </div>
      {members && !list.length && <small>No matching members.</small>}
      {selected.length >= 100 && <small>100 participants selected.</small>}
      {last > 0 && (
        <div className="customer-pager">
          <button type="button" disabled={!shown} onClick={() => setPage(shown - 1)}>
            Previous members
          </button>
          <button type="button" disabled={shown >= last} onClick={() => setPage(shown + 1)}>
            Next members
          </button>
        </div>
      )}
    </fieldset>
  );
}
