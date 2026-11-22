export function money(value: number, locale: string, compact = false) {
  return new Intl.NumberFormat(locale, {
    style: 'currency', currency: 'KZT', currencyDisplay: 'narrowSymbol',
    notation: compact ? 'compact' : 'standard',
    minimumFractionDigits: compact ? 1 : 0, maximumFractionDigits: compact ? 1 : 0,
  }).format(value);
}

export function dateLabel(value: string, locale: string, options: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat(locale, { ...options, timeZone: 'UTC' }).format(new Date(value));
}

export function timeLabel(value: string, locale: string) {
  return dateLabel(value, locale, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
}
