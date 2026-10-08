package services

import (
	"sort"
	"strings"

	"atish/internal/models"
	"atish/internal/util"
)

// Weights are admin-tunable. Connection intent dominates by design.
type Weights struct {
	Intent       float64 `json:"intent"`
	Friendship   float64 `json:"friendship"`
	Relationship float64 `json:"relationship"`
	Interests    float64 `json:"interests"`
	Languages    float64 `json:"languages"`
	Location     float64 `json:"location"`
	Personality  float64 `json:"personality"`
	Availability float64 `json:"availability"`
	Practice     float64 `json:"practice"`
	LikedYou     float64 `json:"liked_you"`
}

func DefaultWeights() Weights {
	return Weights{Intent: 40, Friendship: 8, Relationship: 8, Interests: 20, Languages: 8, Location: 10,
		Personality: 8, Availability: 6, Practice: 6, LikedYou: 10}
}

func (w Weights) max() float64 {
	return w.Intent + w.Friendship + w.Relationship + w.Interests + w.Languages + w.Location + w.Personality + w.Availability + w.Practice
}

type CompatResult struct {
	Compatible bool
	Types      []string
	Score      float64
	Compat     models.Compat
}

// Evaluate decides whether two users may be connected and how well they fit.
// It is symmetric in eligibility but the returned reasons are from a's view.
// This is the single source of truth for compatibility (never done client-side).
func Evaluate(a, b *models.Profile, w Weights, bLikedA bool) CompatResult {
	no := CompatResult{}

	shared := util.Intersect(a.ConnectionTypes, b.ConnectionTypes)
	if len(shared) == 0 || a.Location == nil || b.Location == nil {
		return no
	}
	// Age ranges must hold in both directions.
	if b.Age < a.Preferences.AgeMin || b.Age > a.Preferences.AgeMax ||
		a.Age < b.Preferences.AgeMin || a.Age > b.Preferences.AgeMax {
		return no
	}
	// Distance scope must hold in both directions.
	if !withinScope(a.Preferences.DistanceScope, a.Location, b.Location) ||
		!withinScope(b.Preferences.DistanceScope, b.Location, a.Location) {
		return no
	}
	// Gender preferences only apply to romantic connections. If they don't
	// line up, the pair can still connect as friends.
	if util.Contains(shared, "relationship") &&
		!(genderAccepts(a.Preferences.PreferredGenders, b.Gender) && genderAccepts(b.Preferences.PreferredGenders, a.Gender)) {
		rest := make([]string, 0, len(shared))
		for _, t := range shared {
			if t != "relationship" {
				rest = append(rest, t)
			}
		}
		shared = rest
		if len(shared) == 0 {
			return no
		}
	}
	sort.Strings(shared)

	res := CompatResult{Compatible: true, Types: shared}
	var reasons []models.CompatReason
	var score float64

	// Connection intent
	union := len(util.Dedup(append(append([]string{}, a.ConnectionTypes...), b.ConnectionTypes...)))
	score += w.Intent * (0.6 + 0.4*float64(len(shared))/float64(max(union, 1)))
	reasons = append(reasons, models.CompatReason{Type: "intent", Items: shared})

	if util.Contains(shared, "friends") {
		if o := util.Intersect(a.FriendshipKinds, b.FriendshipKinds); len(o) > 0 {
			score += w.Friendship * float64(min(len(o), 3)) / 3
			reasons = append(reasons, models.CompatReason{Type: "friendship", Items: o})
		}
	}
	if util.Contains(shared, "relationship") {
		score += w.Relationship * intentionFit(a.Preferences.RelationshipIntention, b.Preferences.RelationshipIntention)
	}
	if o := util.Intersect(a.Interests, b.Interests); len(o) > 0 {
		score += w.Interests * float64(min(len(o), 5)) / 5
		reasons = append(reasons, models.CompatReason{Type: "interests", Items: o})
	}
	if o := sharedLanguages(a, b); len(o) > 0 {
		score += w.Languages
		reasons = append(reasons, models.CompatReason{Type: "languages", Items: o})
	}
	if o := practiceMatch(a, b); len(o) > 0 {
		score += w.Practice
		reasons = append(reasons, models.CompatReason{Type: "practice", Items: o})
	}
	if f, items := locationFit(a.Location, b.Location); f > 0 {
		score += w.Location * f
		reasons = append(reasons, models.CompatReason{Type: "location", Items: items})
	}
	if f := personalityFit(a.Personality, b.Personality); f > 0 {
		score += w.Personality * f
	}
	if o := availabilityOverlap(a.Availability, b.Availability); len(o) > 0 {
		score += w.Availability * float64(min(len(o), 4)) / 4
		reasons = append(reasons, models.CompatReason{Type: "availability", Items: o})
	}
	if bLikedA {
		score += w.LikedYou
	}

	ratio := score / maxF(w.max(), 1)
	strength := "possible"
	switch {
	case ratio >= 0.55:
		strength = "strong"
	case ratio >= 0.35:
		strength = "good"
	}
	res.Score = score
	res.Compat = models.Compat{Strength: strength, Types: shared, Reasons: reasons}
	return res
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func genderAccepts(prefs []string, gender string) bool {
	if len(prefs) == 0 {
		return true
	}
	for _, p := range prefs {
		switch p {
		case "everyone", "prefer_not_to_say":
			return true
		case "men":
			if gender == "male" {
				return true
			}
		case "women":
			if gender == "female" {
				return true
			}
		case "non_binary":
			if gender == "non_binary" {
				return true
			}
		}
	}
	return false
}

// intentionFit returns 0..1 for how well two relationship intentions align.
func intentionFit(a, b string) float64 {
	if a == "" || b == "" {
		return 0.5
	}
	if a == b {
		return 1
	}
	grp := func(s string) int {
		switch s {
		case "long_term":
			return 0
		case "short_term", "casual":
			return 1
		}
		return 2 // getting_to_know, not_sure: flexible
	}
	ga, gb := grp(a), grp(b)
	switch {
	case ga == gb:
		return 0.9
	case ga == 2 || gb == 2:
		return 0.6
	}
	return 0
}

func withinScope(scope string, a, b *models.Location) bool {
	if a == nil || b == nil {
		return false
	}
	sameCountry := a.CountryCode == b.CountryCode
	sameRegion := sameCountry && (a.Region == "" || b.Region == "" || strings.EqualFold(a.Region, b.Region))
	sameCity := sameCountry && strings.EqualFold(a.City, b.City)
	switch scope {
	case "anywhere":
		return true
	case "same_country":
		return sameCountry
	case "same_region", "nearby": // "nearby cities" = same region until a city-adjacency table exists
		return sameRegion
	case "same_area":
		if a.Area != "" && b.Area != "" {
			return sameCity && strings.EqualFold(a.Area, b.Area)
		}
		return sameCity
	default: // same_city
		return sameCity
	}
}

func locationFit(a, b *models.Location) (float64, []string) {
	switch {
	case a.CountryCode == b.CountryCode && strings.EqualFold(a.City, b.City):
		if a.Area != "" && strings.EqualFold(a.Area, b.Area) {
			return 1, []string{a.City, a.Area}
		}
		return 0.8, []string{a.City}
	case a.CountryCode == b.CountryCode && a.Region != "" && strings.EqualFold(a.Region, b.Region):
		return 0.4, []string{a.Region}
	case a.CountryCode == b.CountryCode:
		return 0.2, []string{a.CountryCode}
	}
	return 0, nil
}

func sharedLanguages(a, b *models.Profile) []string {
	set := map[string]bool{}
	for _, l := range a.Languages {
		set[l.Code] = true
	}
	var out []string
	for _, l := range b.Languages {
		if set[l.Code] {
			out = append(out, l.Code)
		}
	}
	return out
}

// practiceMatch finds languages one wants to practice that the other speaks well.
func practiceMatch(a, b *models.Profile) []string {
	good := func(p *models.Profile, code string) bool {
		for _, l := range p.Languages {
			if l.Code == code && (l.Level == "native" || l.Level == "advanced") {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	var out []string
	for _, l := range a.Languages {
		if l.WantsPractice && good(b, l.Code) && !seen[l.Code] {
			seen[l.Code] = true
			out = append(out, l.Code)
		}
	}
	for _, l := range b.Languages {
		if l.WantsPractice && good(a, l.Code) && !seen[l.Code] {
			seen[l.Code] = true
			out = append(out, l.Code)
		}
	}
	return out
}

func personalityFit(a, b map[string]string) float64 {
	common, same := 0, 0
	for k, v := range a {
		if bv, ok := b[k]; ok {
			common++
			if bv == v {
				same++
			}
		}
	}
	if common < 3 {
		return 0
	}
	return float64(same) / float64(common)
}

func availabilityOverlap(a, b []models.Slot) []string {
	set := map[models.Slot]bool{}
	for _, s := range a {
		set[s] = true
	}
	var out []string
	for _, s := range b {
		if set[s] {
			out = append(out, itoa(s.Day)+":"+s.Slot)
		}
	}
	return out
}

func itoa(i int) string { return string(rune('0' + i)) }
