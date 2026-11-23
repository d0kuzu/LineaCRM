import { useEffect, useRef, useState, type Dispatch, type FormEvent, type SetStateAction } from 'react';
import { useCrm } from '../crm/session';
import { accessError, errorMessage } from './customerData';
import type { Deal, DealParameters } from './salesData';

type Draft = { amount: string; currency: string; expectedCloseDate: string; probability: string };

export function parameterDraft(deal?: DealParameters): Draft {
  return {
    amount: deal?.amount ?? '',
    currency: deal?.currency ?? 'KZT',
    expectedCloseDate: deal?.expected_close_date ?? '',
    probability: deal?.probability?.toString() ?? '',
  };
}

const amountPattern = '[0-9]{1,14}([.][0-9]{1,4})?';
export const validAmount = new RegExp('^' + amountPattern + '$');

// PostgreSQL returns four fraction digits. Compare decimal text exactly, so
// editing "12.0000" to "12" does not overwrite a concurrent amount change.
function comparableAmount(value: string | null): string | null {
  if (value === null) return null;
  const [whole, fraction = ''] = value.split('.');
  return whole.replace(/^0+(?=\d)/, '') + '.' + fraction.padEnd(4, '0');
}

export function parameterChanges(draft: Draft, original?: DealParameters): Partial<DealParameters> {
  const amount = draft.amount.trim() || null;
  const currency = amount === null ? null : draft.currency.trim().toUpperCase();
  const date = draft.expectedCloseDate || null;
  const chance = draft.probability.trim();
  if (amount !== null && !validAmount.test(amount)) {
    throw new Error('Enter a non-negative amount with up to 14 whole digits and 4 decimal places.');
  }
  if (amount !== null && !/^[A-Z]{3}$/.test(currency ?? ''))
    throw new Error('Enter a three-letter currency code.');
  if (chance && (!/^\d+$/.test(chance) || Number(chance) > 100)) {
    throw new Error('Probability must be a whole number from 0 to 100.');
  }
  const probability = chance === '' ? null : Number(chance);
  const changes: Partial<DealParameters> = {};
  if (comparableAmount(amount) !== comparableAmount(original?.amount ?? null)) {
    changes.amount = amount;
  }
  if (currency !== (original?.currency ?? null)) changes.currency = currency;
  if (date !== (original?.expected_close_date ?? null)) changes.expected_close_date = date;
  if (probability !== (original?.probability ?? null)) changes.probability = probability;
  return changes;
}

export function amountLabel(deal: DealParameters, locale: string): string {
  if (deal.amount == null || !deal.currency) return '—';
  const [whole, fraction = ''] = deal.amount.split('.');
  const formatter = new Intl.NumberFormat(locale, {
    numberingSystem: 'latn',
    minimumFractionDigits: fraction.length,
  });
  const value = formatter
    .formatToParts(BigInt(whole))
    .map((part) => (part.type === 'fraction' ? fraction : part.value))
    .join('');
  return value + ' ' + deal.currency;
}

export function probabilityLabel(value: number | null): string {
  return value == null ? '—' : value + '%';
}

export function DealParameterFields({
  draft,
  setDraft,
}: {
  draft: Draft;
  setDraft: Dispatch<SetStateAction<Draft>>;
}) {
  function update(key: keyof Draft, value: string) {
    setDraft((previous) => ({ ...previous, [key]: value }));
  }
  return (
    <>
      <label>
        Amount
        <input
          type="text"
          inputMode="decimal"
          pattern={amountPattern}
          value={draft.amount}
          title="Up to 14 whole digits and 4 decimal places"
          onChange={(e) => update('amount', e.target.value)}
        />
      </label>
      <label>
        Currency
        <input
          type="text"
          aria-label="Currency"
          maxLength={3}
          pattern="[A-Za-z]{3}"
          disabled={!draft.amount}
          required={Boolean(draft.amount)}
          value={draft.amount ? draft.currency : ''}
          placeholder="KZT"
          onChange={(e) => update('currency', e.target.value.toUpperCase())}
        />
      </label>
      <label>
        Expected close date
        <input
          type="date"
          min="0001-01-01"
          max="9999-12-31"
          value={draft.expectedCloseDate}
          onChange={(e) => update('expectedCloseDate', e.target.value)}
        />
      </label>
      <label>
        Probability (%)
        <input
          type="number"
          min="0"
          max="100"
          step="1"
          value={draft.probability}
          onChange={(e) => update('probability', e.target.value)}
        />
      </label>
    </>
  );
}

export function EditDealParameters({
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
  const { workspace, request } = useCrm();
  const [draft, setDraft] = useState(() => parameterDraft(deal));
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);

  async function save(event: FormEvent) {
    event.preventDefault();
    if (active.current) return;
    let changes: Partial<DealParameters>;
    try {
      changes = parameterChanges(draft, deal);
    } catch (cause) {
      setError(errorMessage(cause));
      return;
    }
    if (!Object.keys(changes).length) {
      onCancel();
      return;
    }
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      await request<Deal>('/crm/workspaces/' + workspace.id + '/deals/' + deal.id, {
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
        <DealParameterFields draft={draft} setDraft={setDraft} />
      </fieldset>
      {error && (
        <p role="alert" className="customer-error">
          {error}{' '}
          <button type="button" className="ws-text-button" onClick={onSaved}>
            Reload deal
          </button>
        </p>
      )}
      <div className="customer-actions">
        <button type="submit" disabled={busy} className="ws-action">
          {busy ? 'Saving…' : 'Save parameters'}
        </button>
        <button type="button" onClick={onCancel} className="ws-pill">
          Cancel
        </button>
      </div>
    </form>
  );
}
