import { useState } from 'react';
import { useLocale } from '../i18n';
import { meetings, workspaceDate } from '../overview/data';
import { dateLabel, timeLabel } from './format';
import { Fields, PageTitle, RecordDialog, Switcher } from './shared';

const days = Array.from({ length: 7 }, (_, i) => '2026-11-' + String(i + 2).padStart(2, '0'));
const hours = Array.from({ length: 8 }, (_, i) => i + 9);
export const calendarActivities = [
  ['2026-11-02T10:00:00Z', 'sync', null], ['2026-11-03T13:00:00Z', 'meeting', 'Qala Studio'],
  ['2026-11-04T14:00:00Z', 'call', 'Terra Logistics'], ['2026-11-05T12:00:00Z', 'pilot', 'Sary Lab'],
  ['2026-11-06T15:00:00Z', 'proposal', 'Orbita'],
] as const;
const types = { orbita: 'demo', tochka: 'terms', atlas: 'procurement' } as const;
interface Appointment { start: string; title: string; company: string; href?: string }

export function Calendar() {
  const { messages: { workspace: { calendar: copy, general }, overview }, locale } = useLocale();
  const [view, setView] = useState<'week' | 'day'>('week');
  const [day, setDay] = useState(workspaceDate);
  const [selected, setSelected] = useState<Appointment | null>(null);
  const appointments: Appointment[] = [
    ...calendarActivities.map(([start, key, company]) => ({ start, title: copy.names[key], company: company ?? copy.team })),
    ...meetings.map(m => ({ start: m.startsAt, title: copy.names[types[m.id]], company: overview.accounts[m.id].company, href: '#/meeting/' + m.id })),
  ];
  const visibleDays = view === 'week' ? days : [day];
  const weekRange = new Intl.DateTimeFormat(locale, { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }).formatRange(new Date(days[0]), new Date(days[6]));
  return <>
    <PageTitle title={copy.title} description={view === 'week' ? copy.weekOf + ' ' + weekRange : dateLabel(day, locale, { dateStyle: 'full' })}/>
    <div className="ws-toolbar">
      <Switcher label={copy.title} selected={view} onSelect={setView} values={[[ 'week', copy.week ], [ 'day', copy.day ]]}/>
      <button className="ws-pill" onClick={() => { setDay(workspaceDate); setView('day'); }}>{copy.today}</button>
      {view === 'day' && <select aria-label={copy.selectedDay} value={day} onChange={e => setDay(e.target.value)}>{days.map(value => <option key={value} value={value}>{dateLabel(value, locale, { month: 'short', day: 'numeric', weekday: 'short' })}</option>)}</select>}
    </div>
    <section aria-label={copy.events} className={'ws-calendar ' + (view === 'week' ? 'ws-week' : 'ws-day')}>
      <div className="ws-time-column"><div className="ws-date-head"/>{hours.map(hour =>
        <time key={hour}>{timeLabel(day + 'T' + String(hour).padStart(2, '0') + ':00:00Z', locale)}</time>)}</div>
      {visibleDays.map(date => <div className="ws-cal-col" key={date}>
        <button className={'ws-date-head ' + (date === workspaceDate ? 'current' : '')} onClick={() => { setDay(date); setView('day'); }}>
          <small>{dateLabel(date, locale, { weekday: 'short' })}</small><b>{dateLabel(date, locale, { day: 'numeric' })}</b>
        </button><div className="ws-cal-hours">
          {appointments.filter(event => event.start.startsWith(date)).map(event => {
            const start = new Date(event.start);
            const position = (start.getUTCHours() - 9 + start.getUTCMinutes() / 60) / hours.length * 100;
            const inner = <><strong>{timeLabel(event.start, locale)} · {event.title}</strong><span>{event.company}</span></>;
            return event.href
              ? <a key={event.start} href={event.href} className={'ws-event ' + (event.href === '#/meeting/orbita' ? 'featured' : '')} style={{ top: position + '%' }}>{inner}</a>
              : <button key={event.start} className="ws-event" style={{ top: position + '%' }} onClick={() => setSelected(event)}>{inner}</button>;
          })}
        </div></div>)}
    </section>
    {selected && <RecordDialog title={selected.title} onClose={() => setSelected(null)}>
      <Fields rows={[[general.company, selected.company], [general.date, dateLabel(selected.start, locale, { dateStyle: 'long' })], [general.time, timeLabel(selected.start, locale)]]}/>
    </RecordDialog>}
  </>;
}
