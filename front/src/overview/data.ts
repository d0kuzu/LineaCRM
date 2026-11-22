// The prototype follows the sales team's working day on November 4, 2026.
export const workspaceDate = '2026-11-04';

export const deals = [
  { id: 'orbita', amount: 2_400_000 },
  { id: 'tochka', amount: 1_850_000 },
  { id: 'atlas', amount: 960_000 },
  { id: 'mayak', amount: 1_200_000 },
] as const;

export const meetings = [
  { id: 'orbita', startsAt: '2026-11-04T11:00:00Z' },
  { id: 'tochka', startsAt: '2026-11-04T12:30:00Z' },
  { id: 'atlas', startsAt: '2026-11-04T16:00:00Z' },
] as const;

export const metrics = [
  { id: 'active', value: 12, icon: 'metric-icon.svg', route: 'sales' },
  { id: 'meetings', value: meetings.length, icon: 'metric-icon1.svg', route: 'calendar' },
  { id: 'tasks', value: 8, icon: 'metric-icon2.svg', route: 'tasks' },
  { id: 'inactive', value: 4, icon: 'metric-icon3.svg', route: 'sales' },
  { id: 'suggestions', value: 2, icon: 'metric-icon4.svg', route: 'assistant' },
] as const;

export const dealActivity = [
  { date: '2026-09-16', new: 8, won: 4 },
  { date: '2026-09-23', new: 10, won: 6 },
  { date: '2026-09-30', new: 7, won: 3 },
  { date: '2026-10-07', new: 12, won: 7 },
  { date: '2026-10-14', new: 11, won: 5 },
  { date: '2026-10-21', new: 15, won: 8 },
  { date: '2026-10-28', new: 9, won: 6 },
  { date: workspaceDate, new: 14, won: 9 },
] as const;
