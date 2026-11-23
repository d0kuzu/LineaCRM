import { useEffect, useState } from 'react';
import { canChangeOwner } from './SalesOwner';

export type ActivityKind = 'tasks' | 'meetings';
export const linkTypes = {
  company_id: { kind: 'companies', label: 'Company' },
  contact_id: { kind: 'contacts', label: 'Contact' },
  deal_id: { kind: 'deals', label: 'Deal' },
} as const;
export type LinkField = keyof typeof linkTypes;
export type InitialLink = { field: LinkField; id: string; label: string };
type Activity = {
  id: string;
  workspace_id: string;
  owner_id: string;
  title: string;
  created_at: string;
  updated_at: string;
  company_id: string | null;
  contact_id: string | null;
  deal_id: string | null;
};
export type Priority = 'low' | 'normal' | 'high';
export type CrmTask = Activity & {
  description: string;
  due_at: string;
  status: 'open' | 'done';
  priority: Priority;
};
export type CrmMeeting = Activity & {
  goal: string;
  starts_at: string;
  ends_at: string;
  participant_ids: string[];
};
export type ActivityRecord = CrmTask | CrmMeeting;
export const activityHref = (kind: ActivityKind, id: string) =>
  '#/' + (kind === 'tasks' ? 'task' : 'meeting') + '/' + encodeURIComponent(id);
export function canEditActivity(record: ActivityRecord, role: string, userID: string) {
  return canChangeOwner(role) || (role === 'sales_manager' && record.owner_id === userID);
}
export function localDay(date: Date) {
  const pad = (n: number) => String(n).padStart(2, '0');
  return (
    String(date.getFullYear()).padStart(4, '0') + '-' + pad(date.getMonth() + 1) + '-' + pad(date.getDate())
  );
}
export function localDateTime(date: Date) {
  return (
    localDay(date) +
    'T' +
    String(date.getHours()).padStart(2, '0') +
    ':' +
    String(date.getMinutes()).padStart(2, '0')
  );
}
export function shiftDay(day: string, delta: number) {
  const date = new Date(day + 'T12:00');
  date.setDate(date.getDate() + delta);
  if (date.getFullYear() < 1) return '0001-01-01';
  if (date.getFullYear() > 9999) return '9999-12-31';
  return localDay(date);
}
export function useActivityRevision() {
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let away = document.hidden;
    const left = () => {
      away = true;
    };
    const returned = () => {
      if (away && !document.hidden) {
        away = false;
        setRevision((n) => n + 1);
      }
    };
    const visibility = () => {
      if (document.hidden) left();
      else returned();
    };
    window.addEventListener('blur', left);
    window.addEventListener('focus', returned);
    document.addEventListener('visibilitychange', visibility);
    return () => {
      window.removeEventListener('blur', left);
      window.removeEventListener('focus', returned);
      document.removeEventListener('visibilitychange', visibility);
    };
  }, []);
  return [revision, () => setRevision((n) => n + 1)] as const;
}
