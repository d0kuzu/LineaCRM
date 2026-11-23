import { useEffect, useRef, type ReactNode } from 'react';
import { useLocale } from '../i18n';

export function PageTitle({ title, description }: { title: string; description: string }) {
  return <header className="ws-title"><h1>{title}</h1><p>{description}</p></header>;
}

export function Switcher<T extends string>({ values, selected, onSelect, label }: {
  values: readonly (readonly [T, string])[]; selected: T; onSelect: (id: T) => void; label: string;
}) {
  return <div className="ws-switcher" role="group" aria-label={label}>
    {values.map(([id, text]) => <button className="ws-pill" key={id} aria-pressed={selected === id} onClick={() => onSelect(id)}>{text}</button>)}
  </div>;
}

export function RecordDialog({ title, children, onClose }: {
  title: string; children: ReactNode; onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const { messages: { workspace: copy } } = useLocale();
  useEffect(() => {
    const dialog = ref.current;
    if (dialog && !dialog.open) dialog.showModal();
    return () => { dialog?.close(); };
  }, []);
  return <dialog className="ws-dialog" ref={ref} aria-label={title} onCancel={event => { event.stopPropagation(); onClose(); }}
    onClick={event => { if (event.currentTarget === event.target) onClose(); }}>
    <div className="ws-dialog-inner"><header><h2>{title}</h2><button onClick={onClose} aria-label={copy.general.close} className="ws-x">×</button></header>{children}</div>
  </dialog>;
}

export function Fields({ rows }: { rows: [string, string][] }) {
  return <dl className="ws-fields">{rows.map(([name, value]) => <div key={name}><dt>{name}</dt><dd>{value}</dd></div>)}</dl>;
}
