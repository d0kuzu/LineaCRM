// Sample records used by the remaining prototype modules.
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
