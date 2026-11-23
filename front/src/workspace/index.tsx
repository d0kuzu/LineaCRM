import { Analytics } from './Analytics';
import { Calendar } from './Calendar';
import { Inbox } from './Inbox';
import { Tasks } from './Tasks';
import { useCrm } from '../crm/session';
import { canViewSection } from '../routes';
import './workspace.css';

const sections = { tasks: Tasks, calendar: Calendar, inbox: Inbox, analytics: Analytics };
export function WorkspaceSection({ section }: { section: keyof typeof sections }) {
  const { workspace } = useCrm();
  if (!canViewSection(section, workspace.role)) {
    return <section className="workspace-section">
      <h1>Sales analytics</h1><p role="status">Sales reports are available to the sales team and workspace administrators.</p>
      <a className="ws-text-button" href="#/overview">Back to overview</a>
    </section>;
  }
  const Content = sections[section];
  return <div className="workspace-section"><Content/></div>;
}
