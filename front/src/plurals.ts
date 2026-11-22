export type CountMessage = { other: string } & Partial<Record<Intl.LDMLPluralRule, string>>;

export function formatCount(forms: CountMessage, count: number, locale: string) {
  const category = new Intl.PluralRules(locale).select(count);
  const template = forms[category] ?? forms.other;
  return template.replaceAll('{count}', count.toLocaleString(locale));
}
