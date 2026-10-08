import type { CompatReason, Catalog } from '@/api/types'

export const GENDERS = [
  { v: 'male', label: 'Man' },
  { v: 'female', label: 'Woman' },
  { v: 'non_binary', label: 'Non-binary' },
  { v: 'other', label: 'Other' },
  { v: 'prefer_not_to_say', label: 'Prefer not to say' },
]

export const INTENTIONS = [
  { v: 'long_term', label: 'Long-term relationship' },
  { v: 'short_term', label: 'Short-term dating' },
  { v: 'casual', label: 'Casual dating' },
  { v: 'getting_to_know', label: 'Getting to know someone' },
  { v: 'not_sure', label: 'Not sure yet' },
]

export const PREF_GENDERS = [
  { v: 'men', label: 'Men' },
  { v: 'women', label: 'Women' },
  { v: 'non_binary', label: 'Non-binary' },
  { v: 'everyone', label: 'Everyone' },
]

export const SCOPES = [
  { v: 'same_area', label: 'My area' },
  { v: 'same_city', label: 'My city' },
  { v: 'nearby', label: 'Nearby cities' },
  { v: 'same_region', label: 'My region' },
  { v: 'same_country', label: 'My country' },
  { v: 'anywhere', label: 'Anywhere' },
]

export const LEVELS = [
  { v: 'basic', label: 'Basic' },
  { v: 'intermediate', label: 'Intermediate' },
  { v: 'advanced', label: 'Advanced' },
  { v: 'native', label: 'Native' },
]

export const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
export const SLOTS = [
  { v: 'morning', label: 'Morning', emoji: '🌅' },
  { v: 'afternoon', label: 'Afternoon', emoji: '☀️' },
  { v: 'evening', label: 'Evening', emoji: '🌆' },
  { v: 'late_night', label: 'Late night', emoji: '🌙' },
]

export const OCCUPATION = [
  { v: 'student', label: 'Student' },
  { v: 'employed', label: 'Employed' },
  { v: 'entrepreneur', label: 'Entrepreneur' },
  { v: 'other', label: 'Other' },
]

export const LIFESTYLE: { key: string; label: string; options: { v: string; label: string }[] }[] = [
  { key: 'smoking', label: 'Smoking', options: [{ v: 'never', label: 'Never' }, { v: 'sometimes', label: 'Sometimes' }, { v: 'regularly', label: 'Regularly' }] },
  { key: 'drinking', label: 'Drinking', options: [{ v: 'never', label: 'Never' }, { v: 'socially', label: 'Socially' }, { v: 'regularly', label: 'Regularly' }] },
  { key: 'pets', label: 'Pets', options: [{ v: 'have', label: 'Have pets' }, { v: 'want', label: 'Want pets' }, { v: 'none', label: 'No pets' }, { v: 'allergic', label: 'Allergic' }] },
  { key: 'diet', label: 'Diet', options: [{ v: 'omnivore', label: 'Omnivore' }, { v: 'vegetarian', label: 'Vegetarian' }, { v: 'vegan', label: 'Vegan' }, { v: 'other', label: 'Other' }] },
  { key: 'sleep_schedule', label: 'Sleep', options: [{ v: 'early', label: 'Early bird' }, { v: 'night', label: 'Night owl' }, { v: 'flexible', label: 'Flexible' }] },
  { key: 'social_level', label: 'Social energy', options: [{ v: 'introvert', label: 'Introvert' }, { v: 'ambivert', label: 'Ambivert' }, { v: 'extrovert', label: 'Extrovert' }] },
]

export const REPORT_REASONS = [
  { v: 'fake_profile', label: 'Fake profile' },
  { v: 'inappropriate_photo', label: 'Inappropriate photo' },
  { v: 'harassment', label: 'Harassment or abuse' },
  { v: 'spam_scam', label: 'Spam or scam' },
  { v: 'underage', label: 'Looks underage' },
  { v: 'hate_speech', label: 'Hate speech' },
  { v: 'other', label: 'Something else' },
]

export function timeAgo(iso: string | null): string {
  if (!iso) return ''
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return 'now'
  if (s < 3600) return `${Math.floor(s / 60)}m`
  if (s < 86400) return `${Math.floor(s / 3600)}h`
  if (s < 604800) return `${Math.floor(s / 86400)}d`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function money(cents: number, currency: string): string {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency.toUpperCase() }).format(cents / 100)
  } catch {
    return `${(cents / 100).toFixed(2)} ${currency.toUpperCase()}`
  }
}

export function countryName(code: string): string {
  try {
    return new Intl.DisplayNames(undefined, { type: 'region' }).of(code.toUpperCase()) ?? code
  } catch {
    return code
  }
}

export function flag(code: string): string {
  if (!/^[A-Za-z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...code.toUpperCase().split('').map((c) => 127397 + c.charCodeAt(0)))
}

const SLOT_LABEL: Record<string, string> = { morning: 'mornings', afternoon: 'afternoons', evening: 'evenings', late_night: 'late nights' }

/** Turns a structured compatibility reason from the API into friendly, localisable text. */
export function reasonText(r: CompatReason, cat: Catalog | null): string {
  const items = r.items ?? []
  const interest = (s: string) => cat?.interests.find((i) => i.slug === s)
  const lang = (c: string) => cat?.languages.find((l) => l.code === c)?.name ?? c
  switch (r.type) {
    case 'intent':
      if (items.length > 1) return 'You are both open to friendship and romance'
      return items[0] === 'relationship' ? 'You are both looking for a relationship' : 'You are both looking for friendship'
    case 'friendship':
      return 'You both want: ' + items.map((s) => cat?.friendship_kinds.find((k) => k.slug === s)?.name.toLowerCase() ?? s).join(', ')
    case 'interests':
      return 'You both like ' + items.slice(0, 3).map((s) => interest(s)?.name ?? s).join(', ') + (items.length > 3 ? ` +${items.length - 3}` : '')
    case 'languages':
      return 'You both speak ' + items.map(lang).join(', ')
    case 'practice':
      return 'Great for language exchange: ' + items.map(lang).join(', ')
    case 'location':
      return items.length > 1 ? `You are both in ${items[0]} · ${items[1]}` : `You are both in ${items[0]}`
    case 'availability': {
      const first = items[0]?.split(':')
      return first ? `You are both free on ${DAYS[+first[0]]} ${SLOT_LABEL[first[1]] ?? ''}`.trim() : ''
    }
  }
  return ''
}

export const STRENGTH: Record<string, { label: string; cls: string }> = {
  strong: { label: 'Strong match', cls: 'brand-gradient text-white' },
  good: { label: 'Good match', cls: 'bg-friend/90 text-white' },
  possible: { label: 'Worth a look', cls: 'bg-black/45 text-white' },
}

export function connIcon(t: string): string {
  return t === 'friends' ? '🤝' : t === 'relationship' ? '❤️' : '✨'
}

/** All ISO 3166-1 alpha-2 country codes (names come from Intl.DisplayNames). */
export const COUNTRY_CODES = (
  'AD AE AF AG AL AM AO AR AT AU AZ BA BB BD BE BF BG BH BI BJ BN BO BR BS BT BW BY BZ CA CD CF CG CH CI CL CM CN CO CR CU CV CY CZ DE DJ DK DM DO DZ EC EE EG ER ES ET FI FJ FM FR GA GB GD GE GH GM GN GQ GR GT GW GY HN HR HT HU ID IE IL IN IQ IR IS IT JM JO JP KE KG KH KI KM KN KP KR KW KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MG MH MK ML MM MN MR MT MU MV MW MX MY MZ NA NE NG NI NL NO NP NR NZ OM PA PE PG PH PK PL PS PT PW PY QA RO RS RU RW SA SB SC SD SE SG SI SK SL SM SN SO SR SS ST SV SY SZ TD TG TH TJ TL TM TN TO TR TT TV TW TZ UA UG US UY UZ VA VC VE VN VU WS YE ZA ZM ZW'
).split(' ')
