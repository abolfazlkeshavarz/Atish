// Package models holds plain data types shared by repositories, services and handlers.
package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusBanned    = "banned"
	StatusDeleted   = "deleted"

	RoleUser      = "user"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
)

type User struct {
	ID               uuid.UUID  `json:"id"`
	TelegramUserID   *int64     `json:"-"`
	TelegramUsername string     `json:"-"`
	TelegramPhotoURL string     `json:"-"`
	AtishUsername    *string    `json:"atish_username"`
	Role             string     `json:"role"`
	Status           string     `json:"status"`
	StatusReason     string     `json:"status_reason,omitempty"`
	TelegramVerified bool       `json:"telegram_verified"`
	PhoneVerified    bool       `json:"phone_verified"`
	PhotoVerified    bool       `json:"photo_verified"`
	IdentityVerified bool       `json:"identity_verified"`
	LanguageCode     string     `json:"-"`
	OnboardedAt      *time.Time `json:"onboarded_at"`
	LastActiveAt     time.Time  `json:"last_active_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Location struct {
	ID          int64  `json:"id"`
	CountryCode string `json:"country_code"`
	Country     string `json:"country"`
	Region      string `json:"region"`
	City        string `json:"city"`
	Area        string `json:"area"`
}

type Photo struct {
	ID         uuid.UUID `json:"id"`
	Position   int       `json:"position"`
	URL        string    `json:"url"`
	StorageKey string    `json:"-"`
}

type UserLanguage struct {
	Code          string `json:"code"`
	Level         string `json:"level"`
	WantsPractice bool   `json:"wants_practice"`
}

type Preferences struct {
	RelationshipIntention string   `json:"relationship_intention"`
	PreferredGenders      []string `json:"preferred_genders"`
	AgeMin                int      `json:"age_min"`
	AgeMax                int      `json:"age_max"`
	DistanceScope         string   `json:"distance_scope"`
}

type Privacy struct {
	ShowArea       bool `json:"show_area"`
	ShowGender     bool `json:"show_gender"`
	ShowEducation  bool `json:"show_education"`
	ShowLifestyle  bool `json:"show_lifestyle"`
	Discoverable   bool `json:"discoverable"`
	NotifyMatches  bool `json:"notify_matches"`
	NotifyMessages bool `json:"notify_messages"`
}

type Slot struct {
	Day  int    `json:"day"`
	Slot string `json:"slot"`
}

type Occupation struct {
	Status       string `json:"occupation_status"`
	University   string `json:"university"`
	FieldOfStudy string `json:"field_of_study"`
	Degree       string `json:"degree"`
	Profession   string `json:"profession"`
	Industry     string `json:"industry"`
}

type Lifestyle struct {
	Smoking       string `json:"smoking"`
	Drinking      string `json:"drinking"`
	Pets          string `json:"pets"`
	Diet          string `json:"diet"`
	SleepSchedule string `json:"sleep_schedule"`
	SocialLevel   string `json:"social_level"`
}

// Profile is the full internal aggregate for one user. It is what the owner
// sees (never sent to other users as-is) and what the matching engine reads.
type Profile struct {
	User
	Premium     bool       `json:"premium"`
	DisplayName string     `json:"display_name"`
	BirthDate   string     `json:"birth_date"`
	Age         int        `json:"age"`
	Gender      string     `json:"gender"`
	Bio         string     `json:"bio"`
	Location    *Location  `json:"location"`
	Occupation  Occupation `json:"occupation"`
	Lifestyle   Lifestyle  `json:"lifestyle"`

	Photos          []Photo           `json:"photos"`
	Interests       []string          `json:"interests"`
	Languages       []UserLanguage    `json:"languages"`
	ConnectionTypes []string          `json:"connection_types"`
	FriendshipKinds []string          `json:"friendship_kinds"`
	Preferences     Preferences       `json:"preferences"`
	Personality     map[string]string `json:"personality"`
	Availability    []Slot            `json:"availability"`
	Privacy         Privacy           `json:"privacy"`

	HasProfile bool `json:"has_profile"` // false until the basic profile row exists
}

type Badges struct {
	Telegram bool `json:"telegram"`
	Phone    bool `json:"phone"`
	Photo    bool `json:"photo"`
	Identity bool `json:"identity"`
	Premium  bool `json:"premium"`
}

type CompatReason struct {
	Type  string   `json:"type"` // intent | interests | languages | location | availability | practice | personality | friendship
	Items []string `json:"items,omitempty"`
}

type Compat struct {
	Strength string         `json:"strength"` // strong | good | possible
	Types    []string       `json:"types"`    // connection types they can connect on
	Reasons  []CompatReason `json:"reasons"`
}

// PublicProfile is the ONLY shape in which another user's data leaves the API.
type PublicProfile struct {
	ID              uuid.UUID      `json:"id"`
	AtishUsername   string         `json:"atish_username"`
	DisplayName     string         `json:"display_name"`
	Age             int            `json:"age"`
	Gender          string         `json:"gender,omitempty"`
	Bio             string         `json:"bio"`
	City            string         `json:"city"`
	Area            string         `json:"area,omitempty"`
	CountryCode     string         `json:"country_code"`
	Photos          []Photo        `json:"photos"`
	Interests       []string       `json:"interests"`
	Languages       []UserLanguage `json:"languages"`
	ConnectionTypes []string       `json:"connection_types"`
	FriendshipKinds []string       `json:"friendship_kinds"`
	Occupation      *Occupation    `json:"occupation,omitempty"`
	Lifestyle       *Lifestyle     `json:"lifestyle,omitempty"`
	Badges          Badges         `json:"badges"`
	Compat          *Compat        `json:"compat,omitempty"`
	LikedYou        bool           `json:"liked_you,omitempty"`
}

type Match struct {
	ID              uuid.UUID      `json:"id"`
	User            *PublicProfile `json:"user"`
	ConnectionTypes []string       `json:"connection_types"`
	CreatedAt       time.Time      `json:"created_at"`
	LastMessageAt   *time.Time     `json:"last_message_at"`
}

type Message struct {
	ID        int64      `json:"id"`
	MatchID   uuid.UUID  `json:"match_id"`
	SenderID  uuid.UUID  `json:"sender_id"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

type Chat struct {
	Match       *Match   `json:"match"`
	LastMessage *Message `json:"last_message"`
	Unread      int      `json:"unread"`
}

type Plan struct {
	ID          int            `json:"id"`
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Interval    string         `json:"interval"`
	PriceCents  int            `json:"price_cents"`
	Currency    string         `json:"currency"`
	StarsPrice  *int           `json:"stars_price"`
	StripePrice *string        `json:"-"`
	Features    map[string]any `json:"features"`
	Position    int            `json:"position"`
	IsActive    bool           `json:"is_active"`
}

type Subscription struct {
	ID               uuid.UUID      `json:"id"`
	UserID           uuid.UUID      `json:"user_id"`
	PlanID           int            `json:"plan_id"`
	PlanCode         string         `json:"plan_code"`
	Provider         string         `json:"provider"`
	ProviderRef      string         `json:"provider_ref"`
	Status           string         `json:"status"`
	StartedAt        time.Time      `json:"started_at"`
	CurrentPeriodEnd *time.Time     `json:"current_period_end"`
	Features         map[string]any `json:"features"`
}

type Entitlements struct {
	Premium  bool            `json:"premium"`
	PlanCode string          `json:"plan_code,omitempty"`
	Until    *time.Time      `json:"until,omitempty"`
	Features map[string]bool `json:"features"`
}

type Report struct {
	ID         int64      `json:"id"`
	ReporterID uuid.UUID  `json:"reporter_id"`
	ReportedID uuid.UUID  `json:"reported_id"`
	MatchID    *uuid.UUID `json:"match_id"`
	Reason     string     `json:"reason"`
	Details    string     `json:"details"`
	Status     string     `json:"status"`
	Resolution string     `json:"resolution"`
	ResolvedBy string     `json:"resolved_by"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
