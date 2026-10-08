export type ConnType = 'friends' | 'relationship' | string

export interface Photo { id: string; position: number; url: string }
export interface Loc { id: number; country_code: string; country: string; region: string; city: string; area: string }
export interface UserLanguage { code: string; level: 'basic' | 'intermediate' | 'advanced' | 'native'; wants_practice: boolean }
export interface Preferences {
  relationship_intention: string
  preferred_genders: string[]
  age_min: number
  age_max: number
  distance_scope: string
}
export interface Privacy {
  show_area: boolean
  show_gender: boolean
  show_education: boolean
  show_lifestyle: boolean
  discoverable: boolean
  notify_matches: boolean
  notify_messages: boolean
}
export interface Slot { day: number; slot: string }
export interface Occupation {
  occupation_status: string
  university: string
  field_of_study: string
  degree: string
  profession: string
  industry: string
}
export interface Lifestyle {
  smoking: string
  drinking: string
  pets: string
  diet: string
  sleep_schedule: string
  social_level: string
}

export interface Entitlements { premium: boolean; plan_code?: string; until?: string; features: Record<string, boolean> }

export interface AppSettings {
  min_age: number
  max_photos: number
  free_daily_likes: number
  premium_enabled: boolean
  maintenance_mode: boolean
  banner: string
  support_username: string
}

export interface Me {
  id: string
  atish_username: string | null
  role: string
  status: string
  telegram_verified: boolean
  phone_verified: boolean
  photo_verified: boolean
  identity_verified: boolean
  premium: boolean
  has_profile: boolean
  display_name: string
  birth_date: string
  age: number
  gender: string
  bio: string
  location: Loc | null
  occupation: Occupation
  lifestyle: Lifestyle
  photos: Photo[]
  interests: string[]
  languages: UserLanguage[]
  connection_types: ConnType[]
  friendship_kinds: string[]
  preferences: Preferences
  personality: Record<string, string>
  availability: Slot[]
  privacy: Privacy
  onboarding: { complete: boolean; missing: string[] }
  completeness: number
  suggestions: string[]
  entitlements: Entitlements
  app: AppSettings
}

export interface RefItem { slug: string; name: string; emoji: string; position: number }
export interface Interest { id: number; slug: string; category: string; name: string; emoji: string }
export interface Language { code: string; name: string; native_name: string }
export interface Question { id: number; key: string; text: string; options: { key: string; label: string }[] }
export interface Catalog {
  connection_types: RefItem[]
  friendship_kinds: RefItem[]
  interests: Interest[]
  languages: Language[]
  personality_questions: Question[]
}

export interface CompatReason { type: string; items?: string[] }
export interface Compat { strength: 'strong' | 'good' | 'possible'; types: ConnType[]; reasons: CompatReason[] }

export interface PublicProfile {
  id: string
  atish_username: string
  display_name: string
  age: number
  gender?: string
  bio: string
  city: string
  area?: string
  country_code: string
  photos: Photo[]
  interests: string[]
  languages: UserLanguage[]
  connection_types: ConnType[]
  friendship_kinds: string[]
  occupation?: Occupation
  lifestyle?: Lifestyle
  badges: { telegram: boolean; phone: boolean; photo: boolean; identity: boolean; premium: boolean }
  compat?: Compat
  liked_you?: boolean
}

export interface Match { id: string; user: PublicProfile; connection_types: ConnType[]; created_at: string; last_message_at: string | null }
export interface Message { id: number; match_id: string; sender_id: string; body: string; created_at: string; read_at: string | null }
export interface Chat { match: Match; last_message: Message | null; unread: number }

export interface Plan {
  id: number
  code: string
  name: string
  description: string
  interval: 'month' | 'year' | 'lifetime'
  price_cents: number
  currency: string
  stars_price: number | null
  features: Record<string, boolean>
  providers: string[]
}

export interface SwipeResult { matched: boolean; match_id?: string; user?: PublicProfile; likes_left: number | null }
export interface FeedResponse { profiles: PublicProfile[]; likes_left: number | null }
