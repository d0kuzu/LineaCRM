import { useEffect, useMemo, useState } from 'react';
import { useActivityRevision, type CrmMeeting, type CrmTask } from '../workspace/activities';

export type Group = 'tasks' | 'meetings' | 'at_risk_deals';
export type Offsets = Record<Group, number>;
export const firstPage: Offsets = { tasks: 0, meetings: 0, at_risk_deals: 0 };
export const pageSize = 5;
export type RiskDeal = {
  id: string;
  title: string;
  owner_id: string;
  pipeline_id: string;
  stage_id: string;
  missing_next_task: boolean;
  stale: boolean;
  last_activity_at: string;
};
type Page<T> = { total: number; items: T[] };
export type Snapshot = {
  as_of: string;
  tasks: Page<CrmTask> & { overdue_total: number };
  meetings: Page<CrmMeeting>;
  at_risk_deals: Page<RiskDeal>;
};

export function useOverviewDay() {
  const [resume, refresh] = useActivityRevision();
  const [minute, setMinute] = useState(0);
  useEffect(() => {
    let timer: number;
    function schedule() {
      // Run at the next minute boundary so that the midnight refresh uses the new local day.
      timer = window.setTimeout(
        () => {
          if (!document.hidden) setMinute((value) => value + 1);
          schedule();
        },
        60_000 - (Date.now() % 60_000) + 10,
      );
    }
    schedule();
    return () => window.clearTimeout(timer);
  }, []);
  const revision = resume + minute;
  const day = useMemo(() => {
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    const end = new Date(start);
    end.setDate(end.getDate() + 1);
    end.setHours(0, 0, 0, 0);
    return { from: start.toISOString(), to: end.toISOString() };
  }, [revision]);
  return { day, revision, refresh };
}
