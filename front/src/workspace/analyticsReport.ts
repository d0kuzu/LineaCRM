import type { Member } from './customerData';
import type { StageKind } from './salesData';

export type SalesResults = {
  target_amount: string | null;
  target_count: number;
  actual_amount: string;
  won_count: number;
  currency_won_count: number;
  unpriced_won_count: number;
  completed_tasks: number;
  finished_meetings: number;
  interactions: number;
};
type Employee = SalesResults & { owner_id: string };
export type FunnelStage = {
  id: string;
  name: string;
  kind: StageKind;
  deal_count: number;
  currency_deal_count: number;
  amount: string;
};
export type SalesReport = {
  month: string;
  timezone: string;
  currency: string;
  from: string;
  to: string;
  as_of: string;
  totals: SalesResults;
  employees: Employee[];
  funnel: { id: string; name: string; stages: FunnelStage[] }[];
};
export type EmployeeRow = Employee & { label: string; eligible: boolean; former: boolean };
const empty: SalesResults = {
  target_amount: null,
  target_count: 0,
  actual_amount: '0.0000',
  won_count: 0,
  currency_won_count: 0,
  unpriced_won_count: 0,
  completed_tasks: 0,
  finished_meetings: 0,
  interactions: 0,
};

export function canHaveTarget(member: Member) {
  return ['admin', 'head_of_sales', 'sales_manager'].includes(member.role);
}

export function employeeRows(
  report: SalesReport,
  members: Member[] | undefined,
  managerID: string | undefined,
  own: { id: string; email: string },
  locale: string,
): EmployeeRow[] {
  const known = new Map((members ?? []).map((member) => [member.user_id, member]));
  const results = new Map(
    report.employees
      .filter((item) => !managerID || item.owner_id === managerID)
      .map((item) => [item.owner_id, item]),
  );
  for (const member of members ?? []) {
    if (canHaveTarget(member) && !results.has(member.user_id))
      results.set(member.user_id, { ...empty, owner_id: member.user_id });
  }
  if (managerID && !results.has(managerID)) results.set(managerID, { ...empty, owner_id: managerID });
  return [...results.values()]
    .map((row) => {
      const member = known.get(row.owner_id);
      return {
        ...row,
        label: member?.email || (own.id === row.owner_id && own.email) || row.owner_id,
        eligible: !!member && canHaveTarget(member),
        former: !!members && !member,
      };
    })
    .sort(
      (left, right) =>
        left.label.localeCompare(right.label, locale) || left.owner_id.localeCompare(right.owner_id),
    );
}
