import { useState } from 'react';
import { useLocale } from '../i18n';
import { deals, meetings } from '../overview/data';
import { dateLabel, money, timeLabel } from './format';
import { Fields, PageTitle, RecordDialog, Switcher } from './shared';

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
export interface Opportunity {
  id: string; company: string; title: string; stage: Stage; activity: string; amount: number; risk: boolean; href?: string;
}

export function usePipeline(): Opportunity[] {
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

export function PipelinePreview() {
  const { messages: { workspace: { sales: copy, general } }, locale } = useLocale();
  const [view, setView] = useState<'board' | 'list'>('board');
  const [risk, setRisk] = useState(false);
  const [selection, setSelection] = useState<Opportunity | null>(null);
  const pipeline = usePipeline();
  const filtered = risk ? pipeline.filter(deal => deal.risk) : pipeline;
  function card(item: Opportunity) {
    const body = <><small>{item.company}</small><strong>{item.title}</strong><b>{money(item.amount, locale)}</b><span>{item.activity}</span></>;
    return item.href
      ? <a key={item.id} href={item.href} className="ws-deal">{body}</a>
      : <button key={item.id} onClick={() => setSelection(item)} className="ws-deal">{body}</button>;
  }
  return <>
    <PageTitle title={copy.title} description="Demo pipeline with sample deals and stages."/>
    <div className="ws-toolbar"><Switcher label={copy.title} selected={view} onSelect={setView} values={[[ 'board', copy.board ], [ 'list', copy.list ]]}/>
      <button className="ws-pill" aria-pressed={risk} onClick={() => setRisk(value => !value)}>{copy.risk}</button>
    </div>
    {view === 'board'
      ? <div className="ws-pipeline">{stages.map(stage => {
        const items = filtered.filter(item => item.stage === stage);
        return <section className="ws-stage" key={stage}>
          <header><h2>{copy.stages[stage]}</h2><span>{items.length.toLocaleString(locale)}</span></header>
          <p>{money(items.reduce((total, deal) => total + deal.amount, 0), locale)}</p>
          {items.map(card)}
          {!items.length && <small>{general.noResults}</small>}
        </section>;
      })}</div>
      : <section className="ws-panel ws-table-panel"><h2>{risk ? copy.risk : copy.allDeals}</h2><div className="ws-table-scroll"><table className="ws-table">
        <thead><tr>{[copy.deal, general.company, general.stage, general.amount, general.nextStep].map(text => <th key={text}>{text}</th>)}</tr></thead>
        <tbody>{filtered.map(item => <tr key={item.id}><td>
          {item.href ? <a className="ws-text-button" href={item.href}>{item.title}</a> : <button className="ws-text-button" onClick={() => setSelection(item)}>{item.title}</button>}
        </td><td>{item.company}</td><td>{copy.stages[item.stage]}</td><td>{money(item.amount, locale)}</td><td>{item.activity}</td></tr>)}</tbody>
      </table></div></section>}
    {selection && <RecordDialog title={selection.title} onClose={() => setSelection(null)}>
      <Fields rows={[[general.company, selection.company], [general.stage, copy.stages[selection.stage]], [general.amount, money(selection.amount, locale)], [general.nextStep, selection.activity]]}/>
    </RecordDialog>}
  </>;
}
