import { useLocale } from '../i18n';
import { sections, type Section } from '../routes';

export function Sidebar({ current }: { current?: Section }) {
  const { messages: { shell: copy, overview } } = useLocale();
  const initials = overview.owner.split(/\s+/).map(part => part[0]).join('');
  return (
    <aside className="sidebar">
      <a href="#/overview" className="brand" aria-label={copy.product}>
        <img src="/icons/linea-brand-mark.svg" alt="" />
        <span>{copy.brand}</span>
      </a>
      <nav className="primary-nav" aria-label={copy.workspace}>
        <p className="nav-caption">{copy.workspace}</p>
        {sections.map(section => (
          <a key={section} href={`#/${section}`} className="nav-link"
            aria-label={copy.nav[section]} title={copy.nav[section]}
            aria-current={current === section ? 'page' : undefined}>
            <img src={`/icons/nav-${section}${current === section ? '-active' : ''}.svg`} alt="" />
            <span>{copy.nav[section]}</span>
          </a>
        ))}
      </nav>
      <div className="sidebar-bottom">
        <a className="assistant-shortcut" href="#/assistant">
          <span className="assistant-shortcut-title">
            <img src="/icons/sparkles.svg" alt="" />{copy.pages.assistant}
          </span>
          <span className="assistant-shortcut-description">{copy.assistantDescription}</span>
          <span className="assistant-shortcut-action">{copy.openChat}<img src="/icons/arrow-up-right.svg" alt="" /></span>
        </a>
        <div className="rep-profile">
          <span className="rep-avatar" aria-hidden="true">{initials}</span>
          <span className="rep-details"><strong>{overview.owner}</strong><small>{copy.salesRep}</small></span>
        </div>
      </div>
    </aside>
  );
}
