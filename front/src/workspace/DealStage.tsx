import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useCrm } from '../crm/session';
import { accessError, errorMessage, useCustomerData } from './customerData';
import { statusLabels, type Deal, type Pipeline } from './salesData';

export function StageSelect({
  value,
  onChange,
  openOnly = false,
  onDenied,
}: {
  value: string;
  onChange: (id: string) => void;
  openOnly?: boolean;
  onDenied: (error: unknown) => void;
}) {
  const { workspace } = useCrm();
  const [revision, setRevision] = useState(0);
  const options = useCustomerData<Pipeline[]>('/crm/workspaces/' + workspace.id + '/pipelines', revision);
  const defaultPipeline = options?.data?.find((item) => item.is_default);
  const firstOpen = defaultPipeline?.stages.find((stage) => stage.kind === 'open');
  useEffect(() => {
    if (accessError(options?.error)) onDenied(options?.error);
  }, [options?.error, onDenied]);
  return (
    <div className="customer-wide">
      <label>
        {openOnly ? 'Starting stage' : 'Destination stage'}
        <select
          value={value}
          required={!openOnly}
          disabled={!options?.data}
          onChange={(event) => onChange(event.target.value)}
        >
          {openOnly && (
            <option value="">
              {defaultPipeline && firstOpen
                ? 'Default — ' + defaultPipeline.name + ' / ' + firstOpen.name
                : 'Default stage'}
            </option>
          )}
          {(options?.data ?? []).map((pipeline) => (
            <optgroup key={pipeline.id} label={pipeline.name}>
              {pipeline.stages
                .filter((stage) => !openOnly || stage.kind === 'open')
                .map((stage) => (
                  <option key={stage.id} value={stage.id}>
                    {stage.name} · {statusLabels[stage.kind]}
                  </option>
                ))}
            </optgroup>
          ))}
        </select>
      </label>
      {!options && <small role="status">Loading stages…</small>}
      {options?.error != null && (
        <p role="alert" className="customer-error">
          {errorMessage(options.error)}{' '}
          <button type="button" className="ws-text-button" onClick={() => setRevision((n) => n + 1)}>
            Retry stages
          </button>
        </p>
      )}
    </div>
  );
}

export function MoveDeal({
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
  const [stage, setStage] = useState(deal.stage_id);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);
  async function move(event: FormEvent) {
    event.preventDefault();
    if (active.current || !stage || stage === deal.stage_id) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      await request<Deal>('/crm/workspaces/' + workspace.id + '/deals/' + deal.id, {
        method: 'PATCH',
        body: JSON.stringify({ stage_id: stage }),
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
    <form className="customer-form sales-form" onSubmit={move}>
      <fieldset className="customer-field-grid" disabled={busy}>
        <StageSelect value={stage} onChange={setStage} onDenied={onDenied} />
      </fieldset>
      {error && (
        <p className="customer-error" role="alert">
          {error}{' '}
          <button type="button" className="ws-text-button" onClick={onSaved}>
            Reload deal
          </button>
        </p>
      )}
      <div className="customer-actions">
        <button className="ws-action" disabled={busy || !stage || stage === deal.stage_id}>
          {busy ? 'Moving…' : 'Move deal'}
        </button>
        <button type="button" className="ws-pill" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}
