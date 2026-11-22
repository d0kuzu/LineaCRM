import { useLocale } from '../i18n';

export function Toolbar({ current }: { current: string }) {
  const { messages: { shell: copy } } = useLocale();
  return (
    <header className="workspace-toolbar">
      <nav aria-label={copy.breadcrumb} className="breadcrumbs">
        <a href="#/overview">{copy.workspace}</a>
        <img src="/icons/chevron-right.svg" alt="" />
        <span aria-current="page">{current}</span>
      </nav>
      <div className="toolbar-actions">
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
