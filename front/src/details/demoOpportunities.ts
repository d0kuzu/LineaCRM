import { useLocale } from '../i18n';
import { deals, meetings } from '../overview/data';
import { dateLabel, timeLabel } from '../workspace/format';

const stages = ['new', 'discovery', 'negotiation', 'contract'] as const;
type Stage = typeof stages[number];

const additionalDeals = [
  ['new', 'Mayak', 'integration', 850000, 'qualification'],
  ['new', 'Alma Group', 'training', 600000, 'intro'],
  ['new', 'Nord Service', 'office', 720000, 'call', '2026-11-06'],
  ['discovery', 'Orbita', 'training', 420000, 'approval', '2026-11-05'],
  ['discovery', 'Qala Studio', 'workstations', 1400000, 'demo', '2026-11-06'],
  ['discovery', 'Sary Lab', 'branch', 1270000, 'meeting', '2026-11-06'],
  ['negotiation', 'Terra Logistics', 'fleet', 950000, 'inactive'],
  ['contract', 'Nord Service', 'support', 910000, 'contract', '2026-11-05'],
] as const;
const featured = {
  orbita: ['negotiation', 'todayDemo'], tochka: ['negotiation', 'todayMeeting'],
  atlas: ['contract', 'todayMeeting'], mayak: ['contract', 'overdue'],
} as const;
const stageOrder = [
  'Mayak-integration', 'Alma Group-training', 'Nord Service-office', 'Orbita-training',
  'Qala Studio-workstations', 'Sary Lab-branch', 'orbita', 'Terra Logistics-fleet',
  'tochka', 'atlas', 'Nord Service-support', 'mayak',
];
interface Opportunity {
  id: string; company: string; title: string; stage: Stage; activity: string; amount: number; risk: boolean; href?: string;
}

export function useDemoOpportunities(): Opportunity[] {
  const { messages: { workspace: { sales: copy }, overview }, locale } = useLocale();
  const pipeline: Opportunity[] = additionalDeals.map(([stage, company, key, amount, activity, date]) => ({
    id: company + '-' + key, stage, company, amount, title: copy.opportunities[key],
    activity: date ? copy.activities[activity].replace('{date}', dateLabel(date, locale, { month: 'short', day: 'numeric' })) : copy.activities[activity],
    risk: activity === 'inactive',
  }));
  for (const deal of deals) {
    const account = overview.accounts[deal.id];
    const [stage, activity] = featured[deal.id];
    const meeting = meetings.find(item => item.id === deal.id);
    const nextStep = meeting
      ? copy.activities[activity].replace('{time}', timeLabel(meeting.startsAt, locale))
      : copy.activities[activity];
    pipeline.push({ id: deal.id, stage, company: account.company, title: account.deal,
      amount: deal.amount, activity: nextStep, risk: true, href: '#/deal/' + deal.id });
  }
  return pipeline.sort((a, b) => stageOrder.indexOf(a.id) - stageOrder.indexOf(b.id));
}
