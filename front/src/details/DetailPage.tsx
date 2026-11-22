import { useLocale } from '../i18n';
import { deals, meetings, workspaceDate } from '../overview/data';
import { dateLabel, money, timeLabel } from '../workspace/format';
import { Fields, PageTitle } from '../workspace/shared';
import { Assistant } from './Assistant';
import { Search } from './Search';
import { fill } from './format';
import './details.css';

function MissingRecord() {
  const { messages: { details: { missing } } } = useLocale();
  return <section className="detail-card"><h1>{missing.title}</h1><p>{missing.body}</p>
    <a className="detail-action" href="#/overview">{missing.back}</a></section>;
}

function Meeting({ meeting }: { meeting: typeof meetings[number] }) {
  const { messages: { details: copy, overview }, locale } = useLocale();
  const { id, startsAt } = meeting;
  const account = overview.accounts[id];
  const orbita = id === 'orbita';
  const start = timeLabel(startsAt, locale);
  const day = dateLabel(startsAt, locale, { weekday: 'long', month: 'short', day: 'numeric' });
  const deal = deals.find(item => item.id === id)!;
  const time = orbita ? start + '–' + timeLabel(workspaceDate + 'T11:45:00Z', locale) : start;
  const subtitle = orbita
    ? fill(copy.meeting.orbitaSubtitle, { time, owner: overview.owner, contact: copy.contacts.aliya })
    : fill(copy.meeting.shortTime, { time });
  const notes = [
    [copy.meeting.goal, copy.meeting.goals[id]],
    [copy.meeting.facts, orbita ? fill(copy.meeting.pilotFacts, { reps: (12).toLocaleString(locale), amount: money(deal.amount, locale) }) : account.alert],
    [copy.meeting.points, copy.meeting.pointsByAccount[id]],
  ];
  const fields: [string, string][] = [
    [copy.search.deal, [account.deal, copy.deal.stages[id === 'atlas' ? 3 : 2]].join(' · ')],
    [copy.meeting.next, fill(copy.meeting.nextByAccount[id], { date: dateLabel('2026-11-06', locale, { month: 'short', day: 'numeric' }) })],
  ];
  if (orbita) fields.push([copy.meeting.attendee, [copy.contacts.aliya, copy.contacts.aliyaRole].join(' · ')]);
  return <>
    <PageTitle title={copy.meeting.title} description={fill(copy.meeting.subtitle, { date: day, name: copy.meeting.names[id] })} />
    <div className="detail-brief-grid">
      <section className="detail-card detail-brief">
        <h2>{[account.company, copy.meeting.names[id]].join(' · ')}</h2><p className="detail-muted">{subtitle}</p>
        {notes.map(([title, note]) => <div className="detail-brief-note" key={title}><h3>{title}</h3><p>{note}</p></div>)}
        {orbita && <div className="detail-badges">
          <span>{fill(copy.meeting.call, { time: timeLabel(workspaceDate + 'T09:40:00Z', locale) })}</span>
          <span>{fill(copy.meeting.email, { date: dateLabel('2026-10-30', locale, { month: 'short', day: 'numeric' }) })}</span>
        </div>}
      </section>
      <aside className="detail-card detail-materials"><h2>{copy.meeting.materials}</h2>
        <Fields rows={fields} /><a href={'#/deal/' + id} className="detail-action">{copy.meeting.openDeal}</a>
      </aside>
    </div>
  </>;
}

function Deal({ deal }: { deal: typeof deals[number] }) {
  const { messages: { details: copy, overview }, locale } = useLocale();
  const { id, amount } = deal;
  const account = overview.accounts[id];
  const orbita = id === 'orbita';
  const meeting = meetings.find(item => item.id === id);
  const stage = orbita || id === 'tochka' ? 2 : 3;
  const fields: [string, string][] = [[orbita ? copy.deal.beforeReview : copy.deal.amount, money(orbita ? 2_000_000 : amount, locale)]];
  if (orbita) {
    fields.push([copy.deal.probability, new Intl.NumberFormat(locale, { style: 'percent' }).format(0.7)],
      [copy.deal.contact, [copy.contacts.aliya, copy.contacts.aliyaRole].join(' · ')],
      [copy.deal.closeDate, dateLabel('2026-11-20', locale, { month: 'short', day: 'numeric', year: 'numeric' })]);
  }
  if (meeting) {
    fields.push([copy.deal.next, [copy.meeting.names[meeting.id], fill(copy.meeting.shortTime, { time: timeLabel(meeting.startsAt, locale) })].join(' · ')]);
  }
  const callTime = timeLabel(workspaceDate + 'T09:40:00Z', locale);
  const events = [
    { time: workspaceDate + 'T09:40:00Z', label: copy.deal.callEnded, info: fill(copy.deal.callDuration, { minutes: (18).toLocaleString(locale) }) },
    { time: '2026-11-03T16:30:00Z', label: copy.deal.scheduled, info: fill(copy.deal.attendees, { owner: overview.owner, contact: copy.contacts.aliya }) },
  ];
  return <>
    <PageTitle title={account.deal} description={fill(copy.deal.subtitle, { company: account.company })} />
    <section className="detail-card detail-stage"><h2>{copy.deal.stages[stage]}</h2>
      <p className="detail-muted">{fill(copy.deal.pipeline, { owner: overview.owner })}</p>
      <ol>{copy.deal.stages.map((name, index) => <li key={name} aria-current={index === stage ? 'step' : undefined}>{name}</li>)}</ol>
    </section>
    <div className="detail-deal-grid">
      <section className="detail-card detail-deal-fields"><h2>{copy.deal.title}</h2><Fields rows={fields} /></section>
      <div className="detail-stack">
        {orbita ? <>
          <section className="detail-card"><h2>{copy.deal.postCall}</h2><p className="detail-muted">{copy.deal.reviewIntro}</p>
            <div className="detail-badges"><span>{fill(copy.meeting.call, { time: callTime })}</span></div>
            <details className="detail-review"><summary className="detail-action">{copy.deal.review}</summary>
              <Fields rows={[[copy.deal.budget, money(amount, locale)], [copy.deal.decisionMaker, copy.contacts.aliya],
                [copy.deal.next, fill(copy.meeting.nextByAccount.orbita, { date: dateLabel('2026-11-06', locale, { month: 'short', day: 'numeric' }) })]]} />
            </details>
          </section>
          <section className="detail-card detail-activity"><h2>{copy.deal.recent}</h2><ul>{events.map(event => <li key={event.time}>
            <time dateTime={event.time}>{[dateLabel(event.time, locale, { month: 'short', day: 'numeric' }), timeLabel(event.time, locale)].join(' · ')}</time>
            <strong>{event.label}</strong><small>{event.info}</small></li>)}</ul></section>
        </> : <section className="detail-card"><h2>{copy.deal.alert}</h2><p>{account.alert}</p>
          {meeting && <a className="detail-action" href={'#/meeting/' + id}>{copy.deal.openMeeting}</a>}
        </section>}
      </div>
    </div>
  </>;
}

type DetailRoute = { page: 'search' | 'assistant' | 'meeting' | 'deal'; id?: string };

function RecordView({ page, id }: DetailRoute) {
  if (page === 'search') return <Search />;
  if (page === 'assistant') return <Assistant />;
  if (page === 'meeting') {
    const meeting = meetings.find(item => item.id === id);
    return meeting ? <Meeting meeting={meeting} /> : <MissingRecord />;
  }
  const deal = deals.find(item => item.id === id);
  return deal ? <Deal key={id} deal={deal} /> : <MissingRecord />;
}

export function DetailPage(route: DetailRoute) {
  return <div className="workspace-section detail-root"><RecordView {...route} /></div>;
}
