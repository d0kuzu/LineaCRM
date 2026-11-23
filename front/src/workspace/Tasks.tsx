import { useState } from 'react';
import { useLocale } from '../i18n';
import { useCrm } from '../crm/session';
import { formatCount } from '../plurals';
import { workspaceDate } from '../overview/data';
import { dateLabel, timeLabel } from './format';
import { PageTitle, Switcher } from './shared';

const taskTimes = ['10:30', '11:00', '14:00', '16:30'];
function savedTasks(storageKey: string): string[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(storageKey) ?? '[]');
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && taskTimes.includes(item)) : [];
  } catch { return []; }
}

export function Tasks() {
  const { messages: { workspace: { tasks: copy, general } }, locale } = useLocale();
  const { profile, workspace } = useCrm();
  const storageKey = 'linea.tasks.2026-11-04.completed.' + profile.id + '.' + workspace.id;
  const [closed, setClosed] = useState(() => savedTasks(storageKey));
  const [view, setView] = useState<'today' | 'completed'>('today');
  const rows = taskTimes.filter(time => closed.includes(time) === (view === 'completed'));
  function toggle(time: string) {
    const next = closed.includes(time) ? closed.filter(item => item !== time) : [...closed, time];
    setClosed(next);
    try { localStorage.setItem(storageKey, JSON.stringify(next)); } catch { /* Keep changes in memory when storage is unavailable. */ }
  }
  const displayTime = (time: string) => timeLabel(workspaceDate + 'T' + time + ':00Z', locale);
  return <>
    <PageTitle title={copy.title} description={[dateLabel(workspaceDate, locale, { weekday: 'long', month: 'short', day: 'numeric', year: 'numeric' }), copy.description].join(' · ')}/>
    <div className="ws-toolbar"><Switcher label={copy.title} selected={view} onSelect={setView} values={[[ 'today', copy.today ], [ 'completed', copy.completed ]]}/></div>
    <div className="ws-task-layout"><section className="ws-panel"><h2>{view === 'today' ? copy.dueToday : copy.completedTitle}</h2>
      {view === 'today' && <p className="ws-subtext">{closed.length === 0 ? copy.firstFour
        : formatCount(copy.remaining, rows.length, locale)}</p>}
      {rows.length === 0 && <p className="ws-empty">{copy.empty}</p>}
      {rows.map(time => { const index = taskTimes.indexOf(time); return <label className="ws-task" key={time}>
        <input type="checkbox" checked={closed.includes(time)} onChange={() => toggle(time)}/>
        <span><strong>{copy.names[index]}</strong><small>{copy.contacts[index]}</small></span><time>{displayTime(time)}</time>
      </label>; })}
    </section>
    <aside className="ws-panel ws-next"><h2>{copy.nextMeeting}</h2><p className="ws-subtext">{copy.today} · {displayTime('11:00')}–{displayTime('11:45')}</p>
      <h3>{copy.demo}</h3><p>{copy.contacts[1]}</p><hr/><p>{copy.aim}</p>
      <a className="ws-action" href="#/meeting/orbita">{general.openBrief}</a>
    </aside></div>
  </>;
}
