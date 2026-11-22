import { useState } from 'react';
import { useLocale } from '../i18n';
import { workspaceDate } from '../overview/data';
import { dateLabel, money } from './format';
import { PageTitle } from './shared';

const kpis = [24000000, 5600000, 20400000, 3600000];
const trend = [[5.6, 6], [10.2, 12], [15.3, 18], [20.4, 24]] as const;
const reps = [
  ['Maria Orlova', 10000000, 2100000, 8400000],
  ['Timur Amanov', 8000000, 2000000, 7000000],
  ['Nina Sokolova', 6000000, 1500000, 5000000],
] as const;

export function Analytics() {
  const { messages: { workspace: { analytics: copy, general }, overview }, locale } = useLocale();
  const [teamView, setTeamView] = useState(false);
  return <>
    <PageTitle title={copy.title} description={copy.description}/>
    <div className="ws-filters ws-switcher" role="group" aria-label={copy.title}>
      <button className="ws-pill" aria-pressed={!teamView} onClick={() => setTeamView(false)}>{copy.summary}</button>
      <a className="ws-pill" href="#/sales">{copy.pipeline}</a>
      <button className="ws-pill" aria-pressed={teamView} onClick={() => setTeamView(true)}>{copy.team}</button>
    </div>
    {!teamView && <>
      <div className="ws-stats">{kpis.map((value, index) => <article key={value} className={'ws-stat ' + (index === 2 ? 'featured' : '')}>
        <span>{copy.metrics[index]}</span><strong>{money(value, locale, true)}</strong><small>{copy.notes[index]}</small></article>)}</div>
      <div className="ws-analytics"><section className="ws-panel"><span className="ws-kicker">{dateLabel(workspaceDate, locale, { month: 'long' })} / {copy.byWeek}</span>
        <h2>{copy.revenue}</h2>
        <div className="ws-legend"><span>{general.forecast}</span><span>{general.target}</span></div>
        <div className="ws-revenue-chart">
          <div className="ws-ticks">{[24,18,12,6,0].map(value => <span key={value}>{money(value * 1000000, locale, true)}</span>)}</div>
          <div className="ws-bars">{trend.map(([forecast, target], index) =>
            <div className="ws-bar-pair" key={target}>
              <div style={{height: forecast / 24 * 100 + '%'}} title={general.forecast + ' · ' + money(forecast * 1000000, locale)} />
              <div style={{height: target / 24 * 100 + '%'}} title={general.target + ' · ' + money(target * 1000000, locale)} />
              <span>{copy.week} {(index + 1).toLocaleString(locale)}</span>
            </div>)}</div></div>
        <table className="ws-visually-hidden"><caption>{copy.revenue}</caption><thead><tr><th>{copy.week}</th><th>{general.forecast}</th><th>{general.target}</th></tr></thead>
          <tbody>{trend.map(([forecast, target], i) => <tr key={target}><td>{i + 1}</td><td>{money(forecast * 1000000, locale)}</td><td>{money(target * 1000000, locale)}</td></tr>)}</tbody></table>
      </section><aside className="ws-panel ws-risk"><span className="ws-kicker">{copy.assistant}</span><h2>{copy.riskTitle}</h2>
        <h3>{copy.count}</h3><p>{copy.cause}</p><div className="ws-context"><strong>{copy.sources}</strong><p>{copy.sourceNote}</p></div>
        <details><summary className="ws-action">{copy.review}</summary><ul>
          {[overview.accounts.orbita.company, 'Aspan', 'Meridian'].map((name, index) => <li key={name}>{index === 0 ? <a className="ws-text-button" href="#/deal/orbita">{name}</a> : <strong>{name}</strong>}<span>{copy.risks[index]}</span></li>)}
        </ul></details>
      </aside></div>
    </>}
    <section className="ws-panel ws-table-panel"><h2>{copy.reps}</h2><div className="ws-table-scroll"><table className="ws-table">
      <thead><tr>{[copy.rep, general.target, copy.actual, general.forecast, copy.atRisk].map(name => <th key={name}>{name}</th>)}</tr></thead>
      <tbody>{reps.map(([name, target, actual, forecast]) => <tr key={name}><td><strong>{name}</strong></td><td>{money(target, locale, true)}</td><td>{money(actual, locale, true)}</td><td>{money(forecast, locale, true)}</td><td>{copy.singleDeal}</td></tr>)}</tbody>
    </table></div></section>
  </>;
}
