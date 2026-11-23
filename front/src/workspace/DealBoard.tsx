import { useEffect, useRef, useState, type DragEvent } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { amountLabel, probabilityLabel } from './DealParameters';
import { accessError, errorMessage, useCustomerData } from './customerData';
import { statusLabels, type Deal, type Pipeline, type PipelineStage } from './salesData';

const pageSize = 20;
type BoardDeal = Pick<
  Deal,
  | 'id'
  | 'title'
  | 'owner_id'
  | 'pipeline_id'
  | 'stage_id'
  | 'status'
  | 'amount'
  | 'currency'
  | 'expected_close_date'
  | 'probability'
>;
type ColumnData = { stage_id: string; total: number; items: BoardDeal[] };
type DragHandlers = {
  start: (event: DragEvent, deal: BoardDeal) => void;
  end: () => void;
  drop: (stage: PipelineStage) => void;
};

function BoardColumn({
  stage,
  initial,
  path,
  params,
  canMove,
  dragging,
  handlers,
  ownerLabel,
  onDenied,
  onOutdated,
}: {
  stage: PipelineStage;
  initial: ColumnData;
  path: string;
  params: string;
  canMove: boolean;
  dragging: BoardDeal | null;
  handlers: DragHandlers;
  ownerLabel: (id: string) => string;
  onDenied: () => void;
  onOutdated: () => void;
}) {
  const { locale } = useLocale();
  const { request } = useCrm();
  const [data, setData] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  const canDrop = canMove && dragging && dragging.stage_id !== stage.id;
  useEffect(() => () => active.current?.abort(), []);
  async function more() {
    if (active.current) return;
    const controller = new AbortController();
    active.current = controller;
    setBusy(true);
    setError('');
    try {
      const query = new URLSearchParams(params);
      query.set('stage_id', stage.id);
      query.set('limit', String(pageSize));
      query.set('offset', String(data.items.length));
      const rows = await request<ColumnData[]>(path + '?' + query, { signal: controller.signal });
      if (controller.signal.aborted) return;
      const next = rows.find((column) => column.stage_id === stage.id);
      const seen = new Set(data.items.map((item) => item.id));
      if (
        !next ||
        next.total !== data.total ||
        next.items.some((item) => seen.has(item.id)) ||
        next.items.length !== Math.min(pageSize, data.total - data.items.length)
      ) {
        onOutdated();
        return;
      }
      setData({ ...next, items: [...data.items, ...next.items] });
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
  return (
    <section
      className={'ws-stage deal-board-column' + (canDrop ? ' deal-drop-target' : '')}
      aria-label={stage.name}
      onDragOver={(event) => {
        if (canDrop) {
          event.preventDefault();
          event.dataTransfer.dropEffect = 'move';
        }
      }}
      onDrop={(event) => {
        if (canDrop) {
          event.preventDefault();
          handlers.drop(stage);
        }
      }}
    >
      <header>
        <h2>{stage.name}</h2>
        <span aria-label={data.total + ' deals'}>{data.total.toLocaleString(locale)}</span>
      </header>
      <p>{statusLabels[stage.kind]}</p>
      {data.items.map((deal) => (
        <article
          key={deal.id}
          className="ws-deal board-card"
          draggable={canMove}
          onDragStart={(event) => handlers.start(event, deal)}
          onDragEnd={handlers.end}
        >
          <a className="board-deal-link" draggable={false} href={'#/sales/pipeline/' + deal.id}>
            {deal.title}
          </a>
          <small>{ownerLabel(deal.owner_id)}</small>
          {deal.amount !== null && <b>{amountLabel(deal, locale)}</b>}
          {(deal.expected_close_date || deal.probability != null) && (
            <span>
              {[deal.expected_close_date, deal.probability == null ? null : probabilityLabel(deal.probability)]
                .filter(Boolean)
                .join(' · ')}
            </span>
          )}
        </article>
      ))}
      {!data.items.length && <small>No matching deals.</small>}
      {error && (
        <p role="alert" className="customer-error">
          {error}
        </p>
      )}
      {data.items.length < data.total && (
        <button className="ws-pill" disabled={busy} onClick={() => void more()}>
          {busy ? 'Loading…' : 'Load more deals'}
        </button>
      )}
    </section>
  );
}

export function DealBoard({
  pipeline,
  path,
  params,
  ownerLabel,
  onDenied,
  onMoved,
  onRefresh,
  onOutdated,
}: {
  pipeline: Pipeline;
  path: string;
  params: string;
  ownerLabel: (id: string) => string;
  onDenied: () => void;
  onMoved: (stage: string) => void;
  onRefresh: () => void;
  onOutdated: () => void;
}) {
  const { request, workspace } = useCrm();
  const [dragging, setDragging] = useState<BoardDeal | null>(null);
  const [moving, setMoving] = useState(false);
  const [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  const board = useCustomerData<ColumnData[]>(path + '?' + params + '&limit=' + pageSize + '&offset=0', 0, 150);
  const canMove = workspace.role !== 'support' && !moving;
  useEffect(() => () => active.current?.abort(), []);
  useEffect(() => {
    if (accessError(board?.error)) onDenied();
  }, [board?.error, onDenied]);
  async function move(stage: PipelineStage) {
    if (active.current || !dragging || !canMove || dragging.stage_id === stage.id) return;
    const deal = dragging;
    setDragging(null);
    setMoving(true);
    setError('');
    const controller = new AbortController();
    active.current = controller;
    try {
      await request<Deal>('/crm/workspaces/' + workspace.id + '/deals/' + deal.id, {
        method: 'PATCH',
        body: JSON.stringify({ stage_id: stage.id }),
        signal: controller.signal,
      });
      if (!controller.signal.aborted) onMoved(stage.name);
    } catch (cause) {
      if (!controller.signal.aborted) {
        if (accessError(cause)) onDenied();
        else setError(errorMessage(cause));
      }
    } finally {
      active.current = null;
      if (!controller.signal.aborted) setMoving(false);
    }
  }
  if (!board)
    return (
      <p className="ws-panel ws-empty" role="status">
        Loading board…
      </p>
    );
  if (!board.data)
    return (
      <p className="ws-panel customer-error" role="alert">
        {errorMessage(board.error)}{' '}
        <button className="ws-text-button" onClick={onRefresh}>
          Retry board
        </button>
      </p>
    );
  const byStage = new Map(board.data.map((column) => [column.stage_id, column]));
  const stageID = new URLSearchParams(params).get('stage_id');
  const stages = pipeline.stages.filter((stage) => !stageID || stage.id === stageID);
  if (!stages.length || stages.length !== byStage.size || stages.some((stage) => !byStage.has(stage.id))) {
    return (
      <p className="ws-panel customer-error" role="alert">
        Stages have changed.{' '}
        <button className="ws-text-button" onClick={onRefresh}>
          Refresh board
        </button>
      </p>
    );
  }
  const handlers: DragHandlers = {
    start(event, deal) {
      if (!canMove) {
        event.preventDefault();
        return;
      }
      event.dataTransfer.effectAllowed = 'move';
      event.dataTransfer.setData('text/plain', deal.id);
      setDragging(deal);
    },
    end() {
      setDragging(null);
    },
    drop(stage) {
      void move(stage);
    },
  };
  return (
    <>
      {error && (
        <p className="customer-error" role="alert">
          {error}{' '}
          <button className="ws-text-button" onClick={onRefresh}>
            Refresh board
          </button>
        </p>
      )}
      {moving && <p role="status">Moving deal…</p>}
      <div className="ws-pipeline deal-board" aria-label="Deal board">
        {stages.map((stage) => (
          <BoardColumn
            key={stage.id}
            stage={stage}
            initial={byStage.get(stage.id)!}
            path={path}
            params={params}
            canMove={canMove}
            dragging={dragging}
            handlers={handlers}
            ownerLabel={ownerLabel}
            onDenied={onDenied}
            onOutdated={onOutdated}
          />
        ))}
      </div>
    </>
  );
}
