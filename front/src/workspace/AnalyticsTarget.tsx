import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { validAmount } from './DealParameters';
import { errorMessage } from './customerData';
import { RecordDialog } from './shared';

type TargetSelection = { id: string; label: string; amount: string | null };
export function AnalyticsTarget({
  employee,
  month,
  currency,
  enabled,
  onCancel,
  onSaved,
  onFailure,
}: {
  employee: TargetSelection;
  month: string;
  currency: string;
  enabled: boolean;
  onCancel: () => void;
  onSaved: () => void;
  onFailure: (cause: unknown) => void;
}) {
  const { workspace, request } = useCrm();
  const [value, setValue] = useState(employee.amount ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  async function write(amount: string | null) {
    if (!enabled || active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      await request(
        '/crm/workspaces/' + workspace.id + '/analytics/targets/' + encodeURIComponent(employee.id),
        {
          method: 'PUT',
          body: JSON.stringify({ month, currency, amount }),
          signal: controller.signal,
        },
      );
      if (!controller.signal.aborted) onSaved();
    } catch (cause) {
      if (!controller.signal.aborted) {
        setError(errorMessage(cause));
        onFailure(cause);
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  function submit(event: FormEvent) {
    event.preventDefault();
    const amount = value.trim();
    if (!validAmount.test(amount)) {
      setError('Enter a non-negative amount with up to 14 whole digits and 4 decimal places.');
      return;
    }
    void write(amount);
  }
  return (
    <RecordDialog title="Monthly sales target" onClose={onCancel}>
      <p className="ws-subtext">
        {employee.label} · {month} · {currency}
      </p>
      <form className="customer-form" onSubmit={submit}>
        <fieldset className="customer-field-grid" disabled={!enabled || busy}>
          <label className="customer-wide">
            Target amount
            <input
              aria-label="Target amount"
              inputMode="decimal"
              type="text"
              autoFocus
              required
              pattern="[0-9]{1,14}([.][0-9]{1,4})?"
              title="Up to 14 whole digits and 4 decimal places"
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
          </label>
        </fieldset>
        {error && (
          <p role="alert" className="customer-error">
            {error}
          </p>
        )}
        <div className="customer-actions">
          <button type="submit" className="ws-action" disabled={!enabled || busy}>
            {busy ? 'Saving…' : 'Save target'}
          </button>
          {employee.amount !== null && (
            <button
              type="button"
              className="ws-pill"
              disabled={!enabled || busy}
              onClick={() => void write(null)}
            >
              Clear target
            </button>
          )}
          <button type="button" className="ws-pill" onClick={onCancel}>
            Cancel
          </button>
        </div>
      </form>
    </RecordDialog>
  );
}
