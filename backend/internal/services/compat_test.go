package services

import (
	"testing"

	"atish/internal/models"
)

func prof(gender string, age int, types []string, prefGenders []string) *models.Profile {
	return &models.Profile{
		Gender: gender, Age: age, ConnectionTypes: types,
		Location:    &models.Location{CountryCode: "IT", Region: "Liguria", City: "Genoa", Area: "Albaro"},
		Preferences: models.Preferences{PreferredGenders: prefGenders, AgeMin: 18, AgeMax: 99, DistanceScope: "same_city", RelationshipIntention: "long_term"},
	}
}

func TestIntentCompatibility(t *testing.T) {
	w := DefaultWeights()
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"friends+friends", []string{"friends"}, []string{"friends"}, true},
		{"relationship+relationship", []string{"relationship"}, []string{"relationship"}, true},
		{"both+friends", []string{"friends", "relationship"}, []string{"friends"}, true},
		{"relationship vs friends-only", []string{"relationship"}, []string{"friends"}, false},
	}
	for _, c := range cases {
		a, b := prof("male", 25, c.a, []string{"everyone"}), prof("female", 25, c.b, []string{"everyone"})
		if got := Evaluate(a, b, w, false).Compatible; got != c.want {
			t.Errorf("%s: compatible=%v want %v", c.name, got, c.want)
		}
	}
}

func TestGenderPrefsOnlyAffectRelationship(t *testing.T) {
	w := DefaultWeights()
	a := prof("male", 25, []string{"friends", "relationship"}, []string{"women"})
	b := prof("male", 25, []string{"friends", "relationship"}, []string{"women"})
	r := Evaluate(a, b, w, false)
	if !r.Compatible || len(r.Types) != 1 || r.Types[0] != "friends" {
		t.Fatalf("expected friends-only match, got %+v", r)
	}
	a.ConnectionTypes, b.ConnectionTypes = []string{"relationship"}, []string{"relationship"}
	if Evaluate(a, b, w, false).Compatible {
		t.Fatal("relationship-only with mismatching gender preference must be incompatible")
	}
}

func TestAgeAndDistanceAreMutual(t *testing.T) {
	w := DefaultWeights()
	a, b := prof("male", 25, []string{"friends"}, []string{"everyone"}), prof("female", 40, []string{"friends"}, []string{"everyone"})
	a.Preferences.AgeMax = 30
	if Evaluate(a, b, w, false).Compatible {
		t.Fatal("b is older than a's max age")
	}
	a.Preferences.AgeMax = 99
	b.Preferences.AgeMin = 30
	if Evaluate(a, b, w, false).Compatible {
		t.Fatal("a is outside b's age range")
	}
	b.Preferences.AgeMin = 18
	b.Location = &models.Location{CountryCode: "IT", Region: "Lombardy", City: "Milan"}
	if Evaluate(a, b, w, false).Compatible {
		t.Fatal("same_city scope must exclude other cities")
	}
	a.Preferences.DistanceScope, b.Preferences.DistanceScope = "same_country", "same_country"
	if !Evaluate(a, b, w, false).Compatible {
		t.Fatal("same_country scope should allow Milan")
	}
}

func TestSharedInterestsRaiseScoreAndGiveReasons(t *testing.T) {
	w := DefaultWeights()
	a, b := prof("male", 25, []string{"friends"}, []string{"everyone"}), prof("female", 25, []string{"friends"}, []string{"everyone"})
	base := Evaluate(a, b, w, false)
	a.Interests, b.Interests = []string{"gaming", "space", "books"}, []string{"gaming", "space", "music"}
	r := Evaluate(a, b, w, false)
	if r.Score <= base.Score {
		t.Fatal("shared interests must raise the score")
	}
	found := false
	for _, x := range r.Compat.Reasons {
		if x.Type == "interests" && len(x.Items) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an interests reason with 2 items")
	}
}
