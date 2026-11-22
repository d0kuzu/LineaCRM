import { useEffect, useRef } from 'react';
import { useLocale } from './i18n';
import { useRoute } from './routes';
import { Sidebar } from './shell/Sidebar';
import { Toolbar } from './shell/Toolbar';
import { Overview } from './overview/Overview';
import { WorkspaceSection } from './workspace';
import { DetailPage } from './details/DetailPage';

export function App() {
  const route = useRoute();
  const { language, messages: { shell } } = useLocale();
  const main = useRef<HTMLElement>(null);
  const current = route.kind === 'section' ? shell.nav[route.page] : shell.pages[route.page];
  const activeSection = route.kind === 'section' ? route.page
    : route.page === 'deal' ? 'sales' : route.page === 'meeting' ? 'calendar' : undefined;
  const path = window.location.hash;
  const lastPath = useRef(path);

  useEffect(() => {
    document.documentElement.lang = language;
    document.title = `${current} · ${shell.product}`;
    if (lastPath.current !== path) {
      lastPath.current = path;
      window.scrollTo({ top: 0, behavior: 'instant' });
      if (route.page !== 'search') main.current?.focus({ preventScroll: true });
    }
  }, [current, language, path, route.page, shell.product]);

  useEffect(() => {
    function handleShortcut(event: KeyboardEvent) {
      if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        if (window.location.hash === '#/search') {
          main.current?.querySelector<HTMLInputElement>('input')?.focus();
        } else {
          window.location.hash = '/search';
        }
      }
    }
    window.addEventListener('keydown', handleShortcut);
    return () => window.removeEventListener('keydown', handleShortcut);
  }, []);

  return (
    <div className="app-shell">
      <button className="skip-link" onClick={() => main.current?.focus()}>{shell.skipNavigation}</button>
      <Sidebar current={activeSection} />
      <div className="workspace-body">
        <Toolbar current={current} />
        <main className="workspace-main" ref={main} tabIndex={-1}>
          {route.kind === 'section' && (route.page === 'overview'
            ? <Overview /> : <WorkspaceSection key={route.page} section={route.page} />)}
          {route.kind === 'detail' && <DetailPage key={path} page={route.page} id={route.id} />}
          {route.kind === 'missing' && <><h1>{shell.pages.missing}</h1><a href="#/overview">{shell.returnHome}</a></>}
        </main>
      </div>
    </div>
  );
}
