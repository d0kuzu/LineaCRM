import { Analytics } from './Analytics';
import { Calendar } from './Calendar';
import { Customers } from './Customers';
import { Inbox } from './Inbox';
import { Sales } from './Sales';
import { Tasks } from './Tasks';
import './workspace.css';

const sections = { customers: Customers, sales: Sales, tasks: Tasks, calendar: Calendar, inbox: Inbox, analytics: Analytics };
export function WorkspaceSection({ section }: { section: keyof typeof sections }) {
  const Content = sections[section];
  return <div className="workspace-section"><Content/></div>;
}
