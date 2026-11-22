import { useState } from 'react';
import { useLocale } from '../i18n';
import { deals, workspaceDate } from '../overview/data';
import { dateLabel, money, timeLabel } from './format';
import { Fields, PageTitle, Switcher } from './shared';

const messages = [
  ['Aliya Kim', 'email', '2026-11-04T10:24:00Z'], ['Roman Titov', 'calls', '2026-11-04T10:20:00Z'],
  ['Saule Zhan', 'messaging', '2026-11-04T10:18:00Z'], ['Irina Davydova', 'email', '2026-11-03'],
  ['Marat Bai', 'messaging', '2026-11-03'], ['Nina Sokolova', 'email', '2026-11-02'],
] as const;
type Channel = 'all' | 'email' | 'calls' | 'messaging';

export function Inbox() {
  const { messages: { workspace: { inbox: copy, general, sales }, overview }, locale } = useLocale();
  const [channel, setChannel] = useState<Channel>('all');
  const [index, setIndex] = useState(0);
  const visible = messages.map((item, id) => ({ item, id })).filter(({ item }) => channel === 'all' || item[1] === channel);
  const active = visible.find(item => item.id === index) ?? visible[0];
  const [sender, type, date] = active.item;
  const orbita = overview.accounts.orbita;
  const dayLabel = (value: string) => dateLabel(value, locale, { month: 'short', day: 'numeric' });
  return <>
    <PageTitle title={copy.title} description={copy.description}/>
    <div className="ws-filters"><Switcher label={copy.channels} selected={channel} onSelect={setChannel}
      values={[[ 'all', copy.all ], [ 'email', copy.email ], [ 'calls', copy.calls ], [ 'messaging', copy.messaging ]]}/></div>
    <div className="ws-inbox">
      <section className="ws-panel"><span className="ws-kicker">{copy.channels} / {dayLabel(workspaceDate)}</span><h2>{copy.title}</h2>
        <div className="ws-messages">{visible.map(({ item: [name,, stamp], id }) => <button className="ws-message" key={id} aria-pressed={id === active.id} onClick={() => setIndex(id)}>
          <span className="ws-avatar">{name.split(' ').map(word => word[0]).join('')}</span>
          <span className="ws-message-summary"><strong>{name}</strong><span>{copy.subjects[id]}</span></span>
          <small>{stamp.startsWith(workspaceDate) ? timeLabel(stamp, locale) : dayLabel(stamp)}</small>
        </button>)}</div>
      </section>
      <article className="ws-panel ws-reader">
        <div className="ws-section-head"><div><span className="ws-kicker">{copy[type]}{active.id === 0 && ' / ' + orbita.company}</span><h2>{copy.subjects[active.id]}</h2></div><span className="ws-tag">{copy.incoming}</span></div>
        <h3>{sender}</h3>
        {active.id === 0 ? <>
          <small>{copy.role}</small><div className="ws-letter"><p>{copy.greeting}</p><p>{copy.proposal}</p><p>{copy.timeline}</p></div>
          <div className="ws-context"><h3>{copy.dealContext}</h3><a className="ws-text-button" href="#/deal/orbita">{orbita.deal} · {sales.stages.negotiation} · {money(deals[0].amount, locale)}</a><p>{general.owner} · {overview.owner}</p></div>
          <div className="ws-context"><strong>{copy.assistant}</strong><p>{copy.note}</p></div>
        </> : <Fields rows={[[copy.channels, copy[type]], [general.date, dayLabel(date)], ...(date.includes('T') ? [[general.time, timeLabel(date, locale)] as [string, string]] : [])]}/>}
      </article>
    </div>
  </>;
}
