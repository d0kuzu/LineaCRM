import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useCrm } from '../crm/session';
import { accessError, errorMessage, useCustomerData } from './customerData';
import { statusLabels, type Pipeline, type StageKind } from './salesData';
import { PageTitle, RecordDialog } from './shared';

type DraftStage = { key: string; id?: string; name: string; kind: StageKind };
function initialStages(pipeline?: Pipeline): DraftStage[] {
  if (pipeline) return pipeline.stages.map((stage) => ({ ...stage, key: stage.id }));
  return [
    { key: 'open', name: 'New', kind: 'open' },
    { key: 'won', name: 'Won', kind: 'won' },
    { key: 'lost', name: 'Lost', kind: 'lost' },
  ];
}

function checkName(name: string) {
  return (
    name.trim().length > 0 &&
    Array.from(name.trim()).length <= 100 &&
    !/[\u0000-\u001f\u007f-\u009f]/u.test(name.trim())
  );
}

function PipelineEditor({
  pipeline,
  onClose,
  onSaved,
  onDenied,
}: {
  pipeline?: Pipeline;
  onClose: () => void;
  onSaved: () => void;
  onDenied: () => void;
}) {
  const { request, workspace } = useCrm();
  const path = '/crm/workspaces/' + workspace.id + '/pipelines';
  const [name, setName] = useState(pipeline?.name ?? '');
  const [version, setVersion] = useState(pipeline?.version);
  const [stages, setStages] = useState(() => initialStages(pipeline));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const nextKey = useRef(0);
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), []);

  function updateStage(key: string, change: Partial<Pick<DraftStage, 'name' | 'kind'>>) {
    setStages((current) => current.map((stage) => (stage.key === key ? { ...stage, ...change } : stage)));
  }
  function move(index: number, by: -1 | 1) {
    setStages((current) => {
      const reordered = [...current];
      [reordered[index], reordered[index + by]] = [reordered[index + by], reordered[index]];
      return reordered;
    });
  }
  function addStage() {
    const key = 'draft-' + nextKey.current++;
    setStages((current) => [...current, { key, name: '', kind: 'open' }]);
  }
  async function run(operation: (signal: AbortSignal) => Promise<void>) {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      await operation(controller.signal);
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onDenied();
        else setError(errorMessage(cause));
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  function reload() {
    if (!pipeline) return;
    void run(async (signal) => {
      const list = await request<Pipeline[]>(path, { signal });
      if (signal.aborted) return;
      const saved = list.find((item) => item.id === pipeline.id);
      if (!saved) {
        onDenied();
        return;
      }
      setName(saved.name);
      setStages(initialStages(saved));
      setVersion(saved.version);
    });
  }
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!checkName(name) || stages.some((stage) => !checkName(stage.name))) {
      setError('Enter a pipeline name and stage names of 1 to 100 characters, without control characters.');
      return;
    }
    if (
      !stages.some((stage) => stage.kind === 'open') ||
      stages.filter((stage) => stage.kind === 'won').length !== 1 ||
      stages.filter((stage) => stage.kind === 'lost').length !== 1
    ) {
      setError(
        'Include at least one open stage, one won stage and one lost stage. Each closing stage must appear once.',
      );
      return;
    }
    void run(async (signal) => {
      const body = {
        name: name.trim(),
        version,
        stages: stages.map(({ id, name, kind }) => ({ id, name: name.trim(), kind })),
      };
      await request<Pipeline>(path + (pipeline ? '/' + pipeline.id : ''), {
        method: pipeline ? 'PUT' : 'POST',
        body: JSON.stringify(body),
        signal,
      });
      if (!signal.aborted) onSaved();
    });
  }
  return (
    <RecordDialog title={pipeline ? 'Edit pipeline' : 'New pipeline'} onClose={onClose}>
      <form className="customer-card customer-form" onSubmit={submit}>
        <fieldset disabled={busy} className="customer-field-grid">
          <label className="customer-wide">
            Pipeline name
            <input required autoFocus value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <div className="customer-wide">
            <h3>Stages</h3>
            <ol className="pipeline-editor-stages">
              {stages.map((stage, index) => (
                <li key={stage.key}>
                  <div className="pipeline-stage-fields">
                    <label>
                      Stage name
                      <input
                        required
                        aria-label={'Stage ' + (index + 1) + ' name'}
                        value={stage.name}
                        onChange={(event) => updateStage(stage.key, { name: event.target.value })}
                      />
                    </label>
                    <label>
                      Outcome
                      <select
                        aria-label={'Stage ' + (index + 1) + ' outcome'}
                        disabled={Boolean(stage.id)}
                        value={stage.kind}
                        onChange={(event) => updateStage(stage.key, { kind: event.target.value as StageKind })}
                      >
                        {Object.entries(statusLabels).map(([value, label]) => (
                          <option key={value} value={value}>
                            {label}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                  <div className="pipeline-stage-actions">
                    <button
                      type="button"
                      className="ws-pill"
                      disabled={index === 0}
                      aria-label={'Move stage ' + (index + 1) + ' up'}
                      onClick={() => move(index, -1)}
                    >
                      Move up
                    </button>
                    <button
                      type="button"
                      className="ws-pill"
                      disabled={index === stages.length - 1}
                      aria-label={'Move stage ' + (index + 1) + ' down'}
                      onClick={() => move(index, 1)}
                    >
                      Move down
                    </button>
                    <button
                      type="button"
                      className="ws-text-button"
                      aria-label={'Remove stage ' + (index + 1)}
                      onClick={() => setStages((current) => current.filter((item) => item.key !== stage.key))}
                    >
                      Remove
                    </button>
                  </div>
                </li>
              ))}
            </ol>
            <button type="button" className="ws-pill" onClick={addStage}>
              Add stage
            </button>
          </div>
        </fieldset>
        {error && (
          <p role="alert" className="customer-error">
            {error}{' '}
            {pipeline && (
              <button type="button" className="ws-text-button" disabled={busy} onClick={reload}>
                Reload saved version
              </button>
            )}
          </p>
        )}
        <div className="customer-actions">
          <button type="submit" className="ws-action" disabled={busy}>
            {busy ? 'Saving…' : 'Save pipeline'}
          </button>
          <button type="button" className="ws-pill" onClick={onClose}>
            Cancel
          </button>
        </div>
      </form>
    </RecordDialog>
  );
}

export function PipelineSettings({ switcher }: { switcher: ReactNode }) {
  const { workspace } = useCrm();
  const [revision, setRevision] = useState(0);
  const [editing, setEditing] = useState<Pipeline | 'new' | null>(null);
  const [notice, setNotice] = useState('');
  const list = useCustomerData<Pipeline[]>('/crm/workspaces/' + workspace.id + '/pipelines', revision);
  const admin = workspace.role === 'admin';
  function closeAndRefresh() {
    setEditing(null);
    setRevision((value) => value + 1);
  }
  return (
    <>
      <div className="customer-page-head">
        <PageTitle title="Pipelines" description={workspace.name} />
        {admin && list?.data && (
          <button
            className="ws-action"
            onClick={() => {
              setNotice('');
              setEditing('new');
            }}
          >
            New pipeline
          </button>
        )}
      </div>
      <div className="ws-toolbar">{switcher}</div>
      {notice && (
        <p className="customer-error" role="alert">
          {notice}
        </p>
      )}
      {!list ? (
        <p className="ws-panel ws-empty" role="status">
          Loading pipelines…
        </p>
      ) : list.error != null ? (
        <p className="ws-panel customer-error" role="alert">
          {errorMessage(list.error)}{' '}
          <button className="ws-text-button" onClick={closeAndRefresh}>
            Retry pipelines
          </button>
        </p>
      ) : list.data?.length ? (
        list.data.map((pipeline) => (
          <section className="ws-panel pipeline-summary" key={pipeline.id}>
            <div className="ws-section-head">
              <h2>
                {pipeline.name}
                {pipeline.is_default && <small>Default</small>}
              </h2>
              {admin && (
                <button
                  className="ws-pill"
                  aria-label={'Edit ' + pipeline.name}
                  onClick={() => {
                    setNotice('');
                    setEditing(pipeline);
                  }}
                >
                  Edit pipeline
                </button>
              )}
            </div>
            <ol>
              {pipeline.stages.map((stage) => (
                <li key={stage.id}>
                  <span>{stage.name}</span>
                  <small>{statusLabels[stage.kind]}</small>
                </li>
              ))}
            </ol>
          </section>
        ))
      ) : (
        <p className="ws-panel ws-empty">No pipelines yet.</p>
      )}
      {admin && list?.data && editing && (
        <PipelineEditor
          pipeline={editing === 'new' ? undefined : editing}
          onClose={() => setEditing(null)}
          onSaved={closeAndRefresh}
          onDenied={() => {
            setNotice('Your workspace access has changed.');
            closeAndRefresh();
          }}
        />
      )}
    </>
  );
}
