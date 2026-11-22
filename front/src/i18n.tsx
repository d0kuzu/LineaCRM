import { createContext, useContext, type ReactNode } from 'react';
import { shellEn } from './shell/en';
import { overviewEn } from './overview/en';
import { workspaceEn } from './workspace/en';
import { detailsEn } from './details/en';

const english = { shell: shellEn, overview: overviewEn, workspace: workspaceEn, details: detailsEn };
const catalogs = { en: english } satisfies Record<string, typeof english>;
type Language = keyof typeof catalogs;

function resolveLocale(preferences: readonly string[]): { language: Language; locale: string } {
  for (const preference of preferences) {
    const language = preference.toLowerCase().split('-')[0];
    if (Object.hasOwn(catalogs, language)) return { language: language as Language, locale: preference };
  }
  return { language: 'en', locale: 'en-US' };
}

const { language, locale } = resolveLocale(navigator.languages);
const settings = { language, locale, messages: catalogs[language] };
const LocaleContext = createContext(settings);

export function LocaleProvider({ children }: { children: ReactNode }) {
  return <LocaleContext.Provider value={settings}>{children}</LocaleContext.Provider>;
}

export function useLocale() {
  return useContext(LocaleContext);
}
