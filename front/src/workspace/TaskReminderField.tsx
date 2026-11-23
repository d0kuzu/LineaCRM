import { useState } from 'react';
import { useLocale } from '../i18n';
import { reminderLabel } from './activities';

const presets = [0, 15, 60, 1440];

export function TaskReminderField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { locale } = useLocale();
  const [custom, setCustom] = useState(value !== '' && !presets.includes(Number(value)));
  return (
    <>
      <label>
        Remind assignee
        <select
          aria-label="Remind assignee"
          value={custom ? 'custom' : value}
          onChange={(event) => {
            const next = event.target.value;
            setCustom(next === 'custom');
            if (next !== 'custom') onChange(next);
          }}
        >
          <option value="">Off</option>
          {presets.map((minutes) => (
            <option key={minutes} value={minutes}>
              {reminderLabel(minutes, locale)}
            </option>
          ))}
          <option value="custom">Custom…</option>
        </select>
      </label>
      {custom && (
        <label>
          Minutes before the deadline
          <input
            type="number"
            required
            min="0"
            max="43200"
            step="1"
            value={value}
            onChange={(event) => onChange(event.target.value)}
          />
        </label>
      )}
    </>
  );
}
