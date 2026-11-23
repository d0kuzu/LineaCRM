import { validAmount } from './DealParameters';
import type { Member } from './customerData';
import type { Pipeline } from './salesData';

export type DealFilters = {
  pipeline_id: string;
  stage_id: string;
  owner_id: string;
  currency: string;
  amount_min: string;
  amount_max: string;
};
export const emptyFilters: DealFilters = {
  pipeline_id: '',
  stage_id: '',
  owner_id: '',
  currency: '',
  amount_min: '',
  amount_max: '',
};

function decimalUnits(value: string) {
  const [whole, fraction = ''] = value.split('.');
  return BigInt(whole + fraction.padEnd(4, '0'));
}
export function filterError(filters: DealFilters) {
  if (filters.currency && !/^[A-Z]{3}$/.test(filters.currency)) return 'Enter a three-letter currency code.';
  if ((filters.amount_min || filters.amount_max) && !filters.currency)
    return 'Enter a currency for the amount range.';
  if ([filters.amount_min, filters.amount_max].some((value) => value && !validAmount.test(value))) {
    return 'Amounts can have up to 14 whole digits and 4 decimal places.';
  }
  if (
    filters.amount_min &&
    filters.amount_max &&
    decimalUnits(filters.amount_min) > decimalUnits(filters.amount_max)
  ) {
    return 'Minimum amount must be less than or equal to maximum amount.';
  }
  return '';
}

export function DealFilterControls({
  filters,
  pipelineID,
  board,
  pipelines,
  members,
  onChange,
  onClear,
}: {
  filters: DealFilters;
  pipelineID: string;
  board: boolean;
  pipelines: Pipeline[];
  members: Member[];
  onChange: (filters: DealFilters) => void;
  onClear: () => void;
}) {
  const selected = pipelines.find((item) => item.id === pipelineID);
  function change(field: keyof DealFilters, value: string) {
    onChange({ ...filters, [field]: value });
  }
  return (
    <fieldset className="deal-filters customer-field-grid" aria-label="Deal filters">
      <label>
        Pipeline
        <select
          aria-label="Pipeline"
          value={pipelineID}
          onChange={(e) => onChange({ ...filters, pipeline_id: e.target.value, stage_id: '' })}
        >
          {!board && <option value="">All pipelines</option>}
          {pipelines.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Stage
        <select
          aria-label="Stage"
          value={filters.stage_id}
          onChange={(e) => change('stage_id', e.target.value)}
        >
          <option value="">All stages</option>
          {(selected ? [selected] : pipelines).map((item) => (
            <optgroup key={item.id} label={item.name}>
              {item.stages.map((stage) => (
                <option key={stage.id} value={stage.id}>
                  {stage.name}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </label>
      <label>
        Owner
        <select
          aria-label="Owner"
          value={filters.owner_id}
          onChange={(e) => change('owner_id', e.target.value)}
        >
          <option value="">All visible owners</option>
          {members.map((member) => (
            <option key={member.user_id} value={member.user_id}>
              {member.email || member.user_id}
            </option>
          ))}
        </select>
      </label>
      <label>
        Currency
        <input
          value={filters.currency}
          maxLength={3}
          list="deal-filter-currencies"
          placeholder="Any currency"
          onChange={(e) => change('currency', e.target.value.toUpperCase())}
        />
      </label>
      <datalist id="deal-filter-currencies">
        {['KZT', 'USD', 'EUR', 'GBP', 'CNY', 'RUB'].map((code) => (
          <option key={code} value={code} />
        ))}
      </datalist>
      <label>
        Minimum amount
        <input
          type="text"
          inputMode="decimal"
          value={filters.amount_min}
          onChange={(e) => change('amount_min', e.target.value)}
        />
      </label>
      <label>
        Maximum amount
        <input
          type="text"
          inputMode="decimal"
          value={filters.amount_max}
          onChange={(e) => change('amount_max', e.target.value)}
        />
      </label>
      <button type="button" className="ws-pill" onClick={onClear}>
        Clear filters
      </button>
    </fieldset>
  );
}
