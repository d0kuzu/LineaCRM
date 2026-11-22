import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@fontsource-variable/inter/wght.css';
import './styles.css';
import { LocaleProvider } from './i18n';
import { App } from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode><LocaleProvider><App /></LocaleProvider></StrictMode>
);
