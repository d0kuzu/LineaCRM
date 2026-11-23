import { useLocale } from '../i18n';
import { useCrm } from '../crm/session';
import { TaskReminders } from './TaskReminders';

export function Toolbar({ current }: { current: string }) {
  const { messages: { shell: copy } } = useLocale();
  const { profile, workspace, selectWorkspace } = useCrm();
  return (
    <header className="workspace-toolbar">
      <nav aria-label={copy.breadcrumb} className="breadcrumbs">
        {profile.workspaces.length > 1
          ? <select className="workspace-picker" aria-label="Current workspace" value={workspace.id}
              onChange={event => { window.location.hash = '/customers'; selectWorkspace(event.target.value); }}>
              {profile.workspaces.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}</select>
          : <a href="#/customers">{workspace.name}</a>}
        <img src="/icons/chevron-right.svg" alt="" />
        <span aria-current="page">{current}</span>
      </nav>
      <div className="toolbar-actions">
        {workspace.role !== 'support' && <TaskReminders />}
        <a href="#/search" className="toolbar-search" aria-keyshortcuts="Control+K Meta+K">
          <img src="/icons/search.svg" alt="" />{copy.pages.search}
          <kbd>{/Mac|iPhone|iPad/i.test(navigator.platform) ? copy.macSearchShortcut : copy.searchShortcut}</kbd>
        </a>
        <a href="#/assistant" className="toolbar-chat">
          <img src="/icons/sparkles1.svg" alt="" />{copy.aiChat}
        </a>
      </div>
    </header>
  );
}
