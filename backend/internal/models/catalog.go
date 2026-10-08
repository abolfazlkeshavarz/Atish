package models

// ProfilePatch is a partial update of public profile data. nil = leave unchanged.
type ProfilePatch struct {
	DisplayName     *string         `json:"display_name"`
	BirthDate       *string         `json:"birth_date"` // YYYY-MM-DD
	Gender          *string         `json:"gender"`
	Bio             *string         `json:"bio"`
	LocationID      *int64          `json:"location_id"`
	Occupation      *Occupation     `json:"occupation"`
	Lifestyle       *Lifestyle      `json:"lifestyle"`
	Interests       *[]string       `json:"interests"`
	Languages       *[]UserLanguage `json:"languages"`
	ConnectionTypes *[]string       `json:"connection_types"`
}

// PreferencesPatch is a partial update of matching / private settings.
type PreferencesPatch struct {
	FriendshipKinds *[]string          `json:"friendship_kinds"`
	Preferences     *Preferences       `json:"preferences"`
	Personality     *map[string]string `json:"personality"`
	Availability    *[]Slot            `json:"availability"`
	Privacy         *Privacy           `json:"privacy"`
}

type RefItem struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
	Position int    `json:"position"`
}

type Interest struct {
	ID       int    `json:"id"`
	Slug     string `json:"slug"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
}

type Language struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
}

type QuestionOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type Question struct {
	ID      int              `json:"id"`
	Key     string           `json:"key"`
	Text    string           `json:"text"`
	Options []QuestionOption `json:"options"`
}

// Catalog is the admin-managed reference data the clients render pickers from.
type Catalog struct {
	ConnectionTypes []RefItem  `json:"connection_types"`
	FriendshipKinds []RefItem  `json:"friendship_kinds"`
	Interests       []Interest `json:"interests"`
	Languages       []Language `json:"languages"`
	Questions       []Question `json:"personality_questions"`
}
