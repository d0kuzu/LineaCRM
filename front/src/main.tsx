import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@fontsource-variable/inter/wght.css';
import './styles.css';
import { LocaleProvider } from './i18n';
import { App } from './App';
import { CrmSession } from './crm/session';

createRoot(document.getElementById('root')!).render(
  <StrictMode><LocaleProvider><CrmSession><App /></CrmSession></LocaleProvider></StrictMode>
);
