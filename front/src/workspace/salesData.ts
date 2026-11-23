export type SalesKind = 'leads' | 'deals';
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
  title: string;
  company_id: string | null;
  contact_ids: string[];
  created_at: string;
};
export const salesLink = (kind: SalesKind, id?: string) =>
  '#/sales/' + kind + (id ? '/' + encodeURIComponent(id) : '');
