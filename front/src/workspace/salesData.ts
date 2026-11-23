export type SalesKind = 'leads' | 'deals';
export type StageKind = 'open' | 'won' | 'lost';
export type PipelineStage = { id: string; name: string; kind: StageKind };
export type Pipeline = {
  id: string;
  name: string;
  version: number;
  is_default: boolean;
  stages: PipelineStage[];
};
export const statusLabels: Record<StageKind, string> = { open: 'Open', won: 'Won', lost: 'Lost' };
export type Lead = {
  id: string;
  workspace_id: string;
  owner_id: string;
  title: string;
  description: string;
  deal_id: string | null;
  created_at: string;
  updated_at: string;
};
export type DealParameters = {
  amount: string | null;
  currency: string | null;
  expected_close_date: string | null;
  probability: number | null;
};
export type Deal = DealParameters & {
  id: string;
  workspace_id: string;
  owner_id: string;
  lead_id: string;
  pipeline_id: string;
  stage_id: string;
  status: StageKind;
  title: string;
  company_id: string | null;
  contact_ids: string[];
  created_at: string;
};
export function dealLocation(deal: Deal, pipelines?: Pipeline[]) {
  const pipeline = pipelines?.find((item) => item.id === deal.pipeline_id);
  const fallback = pipelines ? 'Unavailable' : 'Loading…';
  return {
    pipeline: pipeline?.name ?? fallback,
    stage: pipeline?.stages.find((item) => item.id === deal.stage_id)?.name ?? fallback,
  };
}
export const salesLink = (kind: SalesKind, id?: string) =>
  '#/sales/' + kind + (id ? '/' + encodeURIComponent(id) : '');
