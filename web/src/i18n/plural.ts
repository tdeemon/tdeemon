import type { Locale } from './locale'

// PluralForms are a word's forms by count, as CLDR's plural rules name
// them (hr has one, few and other: 1 minuta, 2 minute, 5 minuta); other
// is the one every language has.
export type PluralForms = Partial<Record<Intl.LDMLPluralRule, string>> & { other: string }

export function plural(locale: Locale, n: number, forms: PluralForms): string {
  return forms[new Intl.PluralRules(locale).select(n)] ?? forms.other
}

// Case is the grammatical case a counted word takes in its sentence: a
// subject's or a label's (nominative: 1 porcija), or a verb's object
// (accusative: Upiši 1 porciju). English has one form for both.
export type Case = 'nominative' | 'accusative'

// CasedForms are a word's forms by count in each case; a case the word
// does not change in (en, or a masculine thing in hr: 1 recept) is its
// nominative.
export type CasedForms = { nominative: PluralForms } & Partial<Record<Case, PluralForms>>

// pluralIn is a word's form by count in a case: hr's porcija is "1
// porciju, 2 porcije, 5 porcija" as a verb's object.
export function pluralIn(
  locale: Locale,
  n: number,
  grammaticalCase: Case,
  forms: CasedForms,
): string {
  return plural(locale, n, forms[grammaticalCase] ?? forms.nominative)
}
