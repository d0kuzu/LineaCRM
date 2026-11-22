import { useSyncExternalStore } from 'react';

export const sections = ['overview', 'customers', 'sales', 'tasks', 'calendar', 'inbox', 'analytics'] as const;
export type Section = (typeof sections)[number];
export type Route =
  | { kind: 'section'; page: Section }
  | { kind: 'detail'; page: 'assistant' | 'search' | 'deal' | 'meeting'; id?: string }
  | { kind: 'missing'; page: 'missing' };

function parseRoute(hash: string): Route {
  const path = hash.slice(1).replace(/^\/+|\/+$/g, '');
  if (!path) return { kind: 'section', page: 'overview' };
  const [page, id, ...extra] = path.split('/');
  if (extra.length) return { kind: 'missing', page: 'missing' };
  if (!id) {
    const section = sections.find(item => item === page);
    if (section) return { kind: 'section', page: section };
    if (page === 'assistant' || page === 'search') return { kind: 'detail', page };
  } else if (page === 'deal' || page === 'meeting') {
    return { kind: 'detail', page, id };
  }
  return { kind: 'missing', page: 'missing' };
}

function subscribe(callback: () => void) {
  window.addEventListener('hashchange', callback);
  return () => window.removeEventListener('hashchange', callback);
}

export function useRoute() {
  return parseRoute(useSyncExternalStore(subscribe, () => window.location.hash));
}
