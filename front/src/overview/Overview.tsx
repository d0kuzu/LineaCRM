import { useLocale } from '../i18n';
import { dealActivity, deals, meetings, metrics, workspaceDate } from './data';
import './overview.css';

function DailySummary() {
  const { messages: { overview: copy }, locale } = useLocale();
  const number = new Intl.NumberFormat(locale);

  return (
    <section className="overview-metrics" aria-labelledby="daily-summary">
      <div className="overview-focus">
        <h2 id="daily-summary">{copy.summary}</h2>
        <p className="overview-focus-title">{copy.focus}</p>
        <p className="overview-focus-description">{copy.pipelineToday}</p>
      </div>
      {metrics.map(metric => (
        <a className="overview-metric" href={`#/${metric.route}`} key={metric.id}>
          <img src={`/icons/${metric.icon}`} width="32" height="32" alt="" />
          <span className="overview-metric-value">{number.format(metric.value)}</span>
          <span className="overview-metric-label">{copy.metrics[metric.id]}</span>
        </a>
      ))}
    </section>
  );
}

function DealActivity() {
  const { messages: { overview: { activity: copy } }, locale } = useLocale();
  const dateFormat = new Intl.DateTimeFormat(locale, { month: '2-digit', day: '2-digit', timeZone: 'UTC' });
  const number = new Intl.NumberFormat(locale);
  const chartMax = 16;
  const ticks = [16, 12, 8, 4, 0];

  return (
    <section className="overview-panel overview-activity" aria-labelledby="deal-activity">
      <header className="overview-panel-header">
        <h2 id="deal-activity">{copy.title}</h2>
        <p>{copy.subtitle}</p>
      </header>
      <div className="overview-chart-legend" aria-hidden="true">
        <span>{copy.new}</span>
        <span>{copy.won}</span>
      </div>
      <div className="overview-chart" aria-hidden="true">
        <div className="overview-chart-grid">
          {ticks.map(tick => (
            <div key={tick} className="overview-chart-gridline">
              <span>{number.format(tick)}</span>
            </div>
          ))}
        </div>
        <div className="overview-chart-weeks">
          {dealActivity.map(week => (
            <div key={week.date} className="overview-chart-week">
              <span className="overview-bar" style={{ height: `${100 * week.new / chartMax}%` }} />
              <span className="overview-bar overview-bar-won" style={{ height: `${100 * week.won / chartMax}%` }} />
              <time dateTime={week.date}>{dateFormat.format(new Date(week.date))}</time>
            </div>
          ))}
        </div>
      </div>
      <div className="overview-sr-only">
        <table aria-labelledby="deal-activity">
          <thead>
            <tr><th scope="col">{copy.week}</th><th scope="col">{copy.new}</th><th scope="col">{copy.won}</th></tr>
          </thead>
          <tbody>
            {dealActivity.map(week => (
              <tr key={week.date}>
                <th scope="row">{dateFormat.format(new Date(week.date))}</th>
                <td>{number.format(week.new)}</td><td>{number.format(week.won)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function UpcomingMeetings() {
  const { messages: { overview: copy }, locale } = useLocale();
  const time = new Intl.DateTimeFormat(locale, {
    hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone: 'UTC',
  });

  return (
    <section className="overview-panel overview-meetings" aria-labelledby="upcoming-meetings">
      <header className="overview-panel-header">
        <h2 id="upcoming-meetings">{copy.upcoming}</h2>
        <p>{copy.agendas}</p>
      </header>
      <ul className="overview-agenda">
        {meetings.map(meeting => (
          <li key={meeting.id}>
            <a href={`#/meeting/${meeting.id}`}>
              <time dateTime={meeting.startsAt}>{time.format(new Date(meeting.startsAt))}</time>
              <span className="overview-meeting-details">
                <strong>{copy.accounts[meeting.id].company}</strong>
                <span>{copy.accounts[meeting.id].agenda}</span>
              </span>
              <img src="/icons/arrow-up-right1.svg" alt="" width="8" height="8" />
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}

function NeedsAttention() {
  const { messages: { overview: copy }, locale } = useLocale();
  const money = new Intl.NumberFormat(locale, {
    style: 'currency', currency: 'KZT', currencyDisplay: 'narrowSymbol', maximumFractionDigits: 0,
  });

  return (
    <section className="overview-panel overview-attention" aria-labelledby="needs-attention">
      <header>
        <h2 id="needs-attention">{copy.attention}</h2>
        <a className="overview-button" href="#/sales">{copy.allDeals}</a>
      </header>
      <div className="overview-table-wrap">
        <table className="overview-deals" aria-labelledby="needs-attention">
          <colgroup>
            <col className="overview-col-deal" /><col className="overview-col-company" />
            <col className="overview-col-amount" /><col className="overview-col-alert" /><col />
          </colgroup>
          <thead>
            <tr>
              <th scope="col">{copy.columns.deal}</th>
              <th scope="col">{copy.columns.company}</th>
              <th scope="col">{copy.columns.amount}</th>
              <th scope="col">{copy.columns.alert}</th>
              <th scope="col">{copy.columns.owner}</th>
            </tr>
          </thead>
          <tbody>
            {deals.map(deal => (
              <tr key={deal.id}>
                <th scope="row"><a href={`#/deal/${deal.id}`}>{copy.accounts[deal.id].deal}</a></th>
                <td>{copy.accounts[deal.id].company}</td>
                <td className="overview-amount">{money.format(deal.amount)}</td>
                <td>{copy.accounts[deal.id].alert}</td>
                <td>{copy.owner}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export function Overview() {
  const { messages: { overview: copy }, locale } = useLocale();
  const date = new Intl.DateTimeFormat(locale, {
    weekday: 'short', month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC',
  }).format(new Date(workspaceDate));

  return (
    <div className="work-overview">
      <header className="overview-heading">
        <div>
          <h1>{copy.title}</h1>
          <p>{copy.workspaceDate.replace('{date}', date)}</p>
        </div>
        <a className="overview-button overview-button-dark" href="#/tasks">{copy.todaysPlan}</a>
      </header>
      <DailySummary />
      <div className="overview-middle">
        <DealActivity />
        <UpcomingMeetings />
      </div>
      <NeedsAttention />
    </div>
  );
}
