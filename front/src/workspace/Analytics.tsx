import { useEffect, useState } from 'react';
import { useCrm } from '../crm/session';
import { useLocale } from '../i18n';
import { useActivityRevision } from './activities';
import { employeeRows, type EmployeeRow, type SalesReport } from './analyticsReport';
import { AnalyticsTarget } from './AnalyticsTarget';
import { amountLabel } from './DealParameters';
import { canChangeOwner } from './SalesOwner';
import { accessError, errorMessage, useCustomerData, type Member } from './customerData';
import { statusLabels } from './salesData';
import { PageTitle, Switcher } from './shared';
import './analytics.css';

type Money = (amount: string) => string;
type Editor = { scope: string; id: string; label: string; amount: string | null };
type Notice = { scope: string; text: string; error: boolean };
const views = [
  ['sales', 'Sales results'],
  ['activity', 'Activity'],
] as const;
function initialMonth() {
  const date = new Date();
  return String(date.getFullYear()).padStart(4, '0') + '-' + String(date.getMonth() + 1).padStart(2, '0');
}
function Summary({ report, money, team }: { report: SalesReport; money: Money; team: boolean }) {
  const row = report.totals;
  return (
    <section className="ws-stats analytics-kpis" aria-label="Monthly totals">
      <article className="ws-stat">
        <span>{team ? 'Team target' : 'Monthly target'}</span>
        <strong>{row.target_amount === null ? 'Not set' : money(row.target_amount)}</strong>
        <small>
          {team
            ? 'Sum of ' + row.target_count + ' set ' + (row.target_count === 1 ? 'target' : 'targets')
            : 'Target for the selected month'}
        </small>
      </article>
      <article className="ws-stat featured">
        <span>Actual sales</span>
        <strong>{money(row.actual_amount)}</strong>
        <small>
          {row.currency_won_count} {row.currency_won_count === 1 ? 'win' : 'wins'} priced in {report.currency}
        </small>
      </article>
      <article className="ws-stat">
        <span>Won deals</span>
        <strong>{row.won_count}</strong>
        <small>All currencies · {row.unpriced_won_count} without an amount</small>
      </article>
    </section>
  );
}
function Funnel({
  report,
  money,
  selected,
  onSelect,
  locale,
}: {
  report: SalesReport;
  money: Money;
  selected: string;
  onSelect: (id: string) => void;
  locale: string;
}) {
  const pipeline = report.funnel.find((p) => p.id === selected) ?? report.funnel[0];
  const largest = pipeline?.stages.reduce((v, s) => Math.max(s.deal_count, v), 1) ?? 1;
  return (
    <section className="ws-panel analytics-panel" aria-labelledby="funnel-heading">
      <header className="ws-section-head">
        <div>
          <h2 id="funnel-heading">Current funnel</h2>
          <p className="ws-subtext">
            Stage counts include every currency. Values use {report.currency}. Updated{' '}
            {new Intl.DateTimeFormat(locale, {
              dateStyle: 'medium',
              timeStyle: 'short',
              timeZone: report.timezone,
            }).format(new Date(report.as_of))}
            .
          </p>
        </div>
        {pipeline && (
          <label className="analytics-select">
            Pipeline
            <select aria-label="Pipeline" value={pipeline.id} onChange={(e) => onSelect(e.target.value)}>
              {report.funnel.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
        )}
      </header>
      {!pipeline ? (
        <p className="ws-empty">No pipelines in this workspace.</p>
      ) : (
        <div className="ws-table-scroll" tabIndex={0} role="region" aria-label="Funnel stages">
          <table className="ws-table analytics-funnel">
            <thead>
              <tr>
                <th scope="col">Stage</th>
                <th scope="col">Deals · all currencies</th>
                <th scope="col">Deals · {report.currency}</th>
                <th scope="col">Value · {report.currency}</th>
              </tr>
            </thead>
            <tbody>
              {pipeline.stages.map((stage) => (
                <tr key={stage.id}>
                  <th scope="row">
                    {stage.name}
                    <small>{statusLabels[stage.kind]}</small>
                  </th>
                  <td>
                    <div className="analytics-count">
                      <b>{stage.deal_count}</b>
                      <span aria-hidden="true">
                        <i style={{ width: (100 * stage.deal_count) / largest + '%' }} />
                      </span>
                    </div>
                  </td>
                  <td>{stage.currency_deal_count}</td>
                  <td>{money(stage.amount)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {pipeline.stages.every((s) => s.deal_count === 0) && (
            <p className="ws-subtext">No visible deals in this pipeline.</p>
          )}
        </div>
      )}
    </section>
  );
}
function Employees({
  report,
  rows,
  money,
  team,
  view,
  onView,
  edit,
}: {
  report: SalesReport;
  rows: EmployeeRow[];
  money: Money;
  team: boolean;
  view: 'sales' | 'activity';
  onView: (view: 'sales' | 'activity') => void;
  edit: (row: EmployeeRow) => void;
}) {
  const total = report.totals;
  const sales = view === 'sales';
  const activity = (row: SalesReport['totals']) => (
    <>
      <td>{row.completed_tasks}</td>
      <td>{row.finished_meetings}</td>
      <td>{row.interactions}</td>
    </>
  );
  return (
    <section className="ws-panel analytics-panel" aria-labelledby="employee-results">
      <header className="ws-section-head">
        <div>
          <h2 id="employee-results">{team ? 'Salespeople' : 'My results'}</h2>
          <p className="ws-subtext">
            {sales
              ? 'Results use the current owner and amount of deals still won after their latest win in this month.'
              : 'Completed tasks belong to the assignee, finished meetings to the organizer, and manual interactions to the author.'}
          </p>
        </div>
        <Switcher values={views} selected={view} label="Employee report" onSelect={onView} />
      </header>
      {rows.length === 0 ? (
        <p className="ws-empty">No people with results or targets for this month.</p>
      ) : (
        <div
          className="ws-table-scroll"
          role="region"
          aria-label={sales ? 'Sales results table' : 'Activity table'}
          tabIndex={0}
        >
          <table className="ws-table analytics-employees">
            <thead>
              <tr>
                <th scope="col">Salesperson</th>
                {sales ? (
                  <>
                    <th scope="col">Target · {report.currency}</th>
                    <th scope="col">Actual · {report.currency}</th>
                    <th scope="col">Wins · all currencies</th>
                    <th scope="col">Wins · {report.currency}</th>
                    <th scope="col">Unpriced wins</th>
                  </>
                ) : (
                  <>
                    <th scope="col">Completed tasks</th>
                    <th scope="col">Finished meetings</th>
                    <th scope="col">Manual interactions</th>
                  </>
                )}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.owner_id}>
                  <th scope="row">
                    {row.label}
                    {row.former && <small>Former member</small>}
                  </th>
                  {sales ? (
                    <>
                      <td>
                        {row.target_amount === null ? 'Not set' : money(row.target_amount)}
                        {team && row.eligible && (
                          <button
                            type="button"
                            className="ws-text-button analytics-edit"
                            aria-label={'Edit target for ' + row.label}
                            onClick={() => edit(row)}
                          >
                            {row.target_amount === null ? 'Set target' : 'Edit target'}
                          </button>
                        )}
                      </td>
                      <td>{money(row.actual_amount)}</td>
                      <td>{row.won_count}</td>
                      <td>{row.currency_won_count}</td>
                      <td>{row.unpriced_won_count}</td>
                    </>
                  ) : (
                    activity(row)
                  )}
                </tr>
              ))}
            </tbody>
            {!sales && team && (
              <tfoot>
                <tr>
                  <th scope="row">Total</th>
                  {activity(total)}
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      )}
      {sales && total.won_count === 0 && <p className="ws-subtext">No won deals for this month.</p>}
      {!sales && !total.interactions && !total.completed_tasks && !total.finished_meetings && (
        <p className="ws-subtext">No activity for this month.</p>
      )}
    </section>
  );
}

export function Analytics() {
  const { workspace, profile } = useCrm();
  const { locale } = useLocale();
  const [month, setMonth] = useState(initialMonth),
    [currency, setCurrency] = useState('KZT');
  const [pipeline, setPipeline] = useState(''),
    [view, setView] = useState<'sales' | 'activity'>('sales');
  const [revision, refresh] = useActivityRevision();
  const [editor, setEditor] = useState<Editor | null>(null),
    [notice, setNotice] = useState<Notice | null>(null);
  const team = canChangeOwner(workspace.role);
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const validMonth = /^\d{4}-(0[1-9]|1[0-2])$/.test(month) && month >= '0001-01';
  const validCurrency = /^[A-Z]{3}$/.test(currency);
  const scope = [workspace.id, profile.id, workspace.role, month, currency, timezone].join('|');
  const query = new URLSearchParams({ month, currency, timezone });
  const result = useCustomerData<SalesReport>(
    validMonth && validCurrency ? '/crm/workspaces/' + workspace.id + '/analytics?' + query : null,
    revision,
  );
  const roster = useCustomerData<Member[]>(
    team ? '/auth/workspaces/' + workspace.id + '/members' : null,
    revision,
  );
  const denied = accessError(result?.error) || accessError(roster?.error);
  const report = result?.data;
  const rows = report
    ? employeeRows(report, team ? roster?.data : undefined, team ? undefined : profile.id, profile, locale)
    : [];
  const money: Money = (amount) => amountLabel({ amount, currency }, locale);
  const activeEditor = editor?.scope === scope ? editor : null;
  const eligible =
    activeEditor &&
    roster?.data?.some(
      (member) =>
        member.user_id === activeEditor.id && (canChangeOwner(member.role) || member.role === 'sales_manager'),
    );
  useEffect(() => {
    if (activeEditor && (denied || (roster?.data && !eligible))) {
      setEditor(null);
      if (!denied)
        setNotice({ scope, text: 'This salesperson is no longer eligible for a target.', error: true });
    }
  }, [activeEditor, denied, roster?.data, eligible, scope]);
  function edit(row: EmployeeRow) {
    if (!team || !row.eligible) return;
    setNotice(null);
    setEditor({ scope, id: row.owner_id, label: row.label, amount: row.target_amount });
  }
  return (
    <div className="analytics-report">
      <div className="customer-page-head">
        <PageTitle title="Sales analytics" description={workspace.name + ' · ' + timezone} />
        <button className="ws-pill" onClick={refresh}>
          Refresh report
        </button>
      </div>
      <div className="ws-toolbar analytics-controls" role="group" aria-label="Report filters">
        <label>
          Reporting month
          <input
            type="month"
            aria-label="Reporting month"
            min="0001-01"
            max="9999-12"
            required
            value={month}
            onChange={(e) => {
              setEditor(null);
              setNotice(null);
              setMonth(e.target.value);
            }}
          />
        </label>
        <label>
          Currency
          <input
            type="text"
            aria-label="Report currency"
            maxLength={3}
            pattern="[A-Za-z]{3}"
            list="analytics-currencies"
            placeholder="KZT"
            value={currency}
            onChange={(e) => {
              setEditor(null);
              setNotice(null);
              setCurrency(e.target.value.toUpperCase());
            }}
          />
        </label>
        <datalist id="analytics-currencies">
          {['KZT', 'USD', 'EUR', 'GBP', 'CNY', 'RUB'].map((code) => (
            <option key={code} value={code} />
          ))}
        </datalist>
      </div>
      {!validMonth || !validCurrency ? (
        <p role="alert" className="ws-panel customer-error">
          {!validMonth ? 'Choose a valid reporting month.' : 'Enter a three-letter currency code.'}
        </p>
      ) : denied || result?.error != null ? (
        <p role="alert" className="ws-panel customer-error">
          {denied ? 'Your access to this report has changed.' : errorMessage(result?.error)}{' '}
          <button className="ws-text-button" onClick={refresh}>
            Retry report
          </button>
        </p>
      ) : !report ? (
        <p className="ws-panel ws-empty" role="status">
          Loading sales report…
        </p>
      ) : (
        <>
          {notice?.scope === scope && (
            <p
              role={notice.error ? 'alert' : 'status'}
              className={notice.error ? 'customer-error' : 'ws-subtext'}
            >
              {notice.text}
            </p>
          )}
          {roster?.error != null && (
            <p role="alert" className="customer-error">
              Could not load the current sales team.{' '}
              <button className="ws-text-button" onClick={refresh}>
                Retry members
              </button>
            </p>
          )}
          <Summary report={report} money={money} team={team} />
          <Employees
            report={report}
            rows={rows}
            money={money}
            team={team}
            view={view}
            onView={setView}
            edit={edit}
          />
          <Funnel report={report} money={money} locale={locale} selected={pipeline} onSelect={setPipeline} />
        </>
      )}
      {activeEditor && team && !denied && (
        <AnalyticsTarget
          key={scope + '|' + activeEditor.id}
          employee={activeEditor}
          month={month}
          currency={currency}
          enabled={!!eligible && !!report}
          onCancel={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            setNotice({ scope, text: 'Monthly target updated.', error: false });
            refresh();
          }}
          onFailure={(cause) => {
            if (accessError(cause)) {
              setEditor(null);
              setNotice({ scope, text: errorMessage(cause), error: true });
            }
            refresh();
          }}
        />
      )}
    </div>
  );
}
