package repository

import (
	"context"
	"encoding/json"
	"time"

	"atish/internal/models"
	"atish/internal/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LoadProfiles batch-loads full profile aggregates. Users without a profile
// row are returned with HasProfile=false.
func (r *Repo) LoadProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*models.Profile, error) {
	out := make(map[uuid.UUID]*models.Profile, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+userCols+`,
			p.user_id IS NOT NULL, COALESCE(p.display_name,''), p.birth_date, COALESCE(p.gender,''), COALESCE(p.bio,''),
			COALESCE(p.occupation_status,''), COALESCE(p.university,''), COALESCE(p.field_of_study,''), COALESCE(p.degree,''),
			COALESCE(p.profession,''), COALESCE(p.industry,''),
			COALESCE(p.smoking,''), COALESCE(p.drinking,''), COALESCE(p.pets,''), COALESCE(p.diet,''),
			COALESCE(p.sleep_schedule,''), COALESCE(p.social_level,''),
			l.id, COALESCE(l.country_code,''), COALESCE(l.country,''), COALESCE(l.region,''), COALESCE(l.city,''), COALESCE(l.area,'')
		FROM users u
		LEFT JOIN profiles p ON p.user_id=u.id
		LEFT JOIN locations l ON l.id=p.location_id
		WHERE u.id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for rows.Next() {
		p := &models.Profile{
			Photos: []models.Photo{}, Interests: []string{}, Languages: []models.UserLanguage{},
			ConnectionTypes: []string{}, FriendshipKinds: []string{}, Personality: map[string]string{}, Availability: []models.Slot{},
			Preferences: models.Preferences{PreferredGenders: []string{"everyone"}, AgeMin: 18, AgeMax: 99, DistanceScope: "same_city"},
			Privacy:     models.Privacy{ShowArea: true, ShowGender: true, Discoverable: true, NotifyMatches: true, NotifyMessages: true},
		}
		var birth *time.Time
		var locID *int64
		var loc models.Location
		u := &p.User
		if err := rows.Scan(&u.ID, &u.TelegramUserID, &u.TelegramUsername, &u.TelegramPhotoURL, &u.AtishUsername, &u.Role,
			&u.Status, &u.StatusReason, &u.TelegramVerified, &u.PhoneVerified, &u.PhotoVerified, &u.IdentityVerified,
			&u.LanguageCode, &u.OnboardedAt, &u.LastActiveAt, &u.CreatedAt,
			&p.HasProfile, &p.DisplayName, &birth, &p.Gender, &p.Bio,
			&p.Occupation.Status, &p.Occupation.University, &p.Occupation.FieldOfStudy, &p.Occupation.Degree,
			&p.Occupation.Profession, &p.Occupation.Industry,
			&p.Lifestyle.Smoking, &p.Lifestyle.Drinking, &p.Lifestyle.Pets, &p.Lifestyle.Diet,
			&p.Lifestyle.SleepSchedule, &p.Lifestyle.SocialLevel,
			&locID, &loc.CountryCode, &loc.Country, &loc.Region, &loc.City, &loc.Area); err != nil {
			rows.Close()
			return nil, err
		}
		if birth != nil {
			p.BirthDate = birth.Format("2006-01-02")
			p.Age = util.AgeOn(*birth, now)
		}
		if locID != nil {
			loc.ID = *locID
			p.Location = &loc
		}
		out[u.ID] = p
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Child collections: one query each, batched across all requested users.
	q := func(sql string, scan func(rows pgx.Rows) error) error {
		rs, err := r.pool.Query(ctx, sql, ids)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			if err := scan(rs); err != nil {
				return err
			}
		}
		return rs.Err()
	}

	if err := q(`SELECT user_id, id, storage_key, position FROM user_photos WHERE user_id=ANY($1) AND status='active' ORDER BY position, created_at`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var ph models.Photo
			if err := rs.Scan(&uid, &ph.ID, &ph.StorageKey, &ph.Position); err != nil {
				return err
			}
			out[uid].Photos = append(out[uid].Photos, ph)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT ui.user_id, i.slug FROM user_interests ui JOIN interests i ON i.id=ui.interest_id WHERE ui.user_id=ANY($1) AND i.is_active ORDER BY i.position`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var s string
			if err := rs.Scan(&uid, &s); err != nil {
				return err
			}
			out[uid].Interests = append(out[uid].Interests, s)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT user_id, language_code, level, wants_practice FROM user_languages WHERE user_id=ANY($1) ORDER BY language_code`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var l models.UserLanguage
			if err := rs.Scan(&uid, &l.Code, &l.Level, &l.WantsPractice); err != nil {
				return err
			}
			out[uid].Languages = append(out[uid].Languages, l)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT user_id, type_slug FROM user_connection_types WHERE user_id=ANY($1) ORDER BY type_slug`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var s string
			if err := rs.Scan(&uid, &s); err != nil {
				return err
			}
			out[uid].ConnectionTypes = append(out[uid].ConnectionTypes, s)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT user_id, kind FROM user_friendship_kinds WHERE user_id=ANY($1) ORDER BY kind`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var s string
			if err := rs.Scan(&uid, &s); err != nil {
				return err
			}
			out[uid].FriendshipKinds = append(out[uid].FriendshipKinds, s)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT user_id, COALESCE(relationship_intention,''), preferred_genders, age_min, age_max, distance_scope FROM preferences WHERE user_id=ANY($1)`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var pr models.Preferences
			if err := rs.Scan(&uid, &pr.RelationshipIntention, &pr.PreferredGenders, &pr.AgeMin, &pr.AgeMax, &pr.DistanceScope); err != nil {
				return err
			}
			out[uid].Preferences = pr
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT a.user_id, q.key, a.option_key FROM personality_answers a JOIN personality_questions q ON q.id=a.question_id WHERE a.user_id=ANY($1)`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var k, v string
			if err := rs.Scan(&uid, &k, &v); err != nil {
				return err
			}
			out[uid].Personality[k] = v
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT user_id, day, slot FROM availability WHERE user_id=ANY($1) ORDER BY day, slot`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var s models.Slot
			if err := rs.Scan(&uid, &s.Day, &s.Slot); err != nil {
				return err
			}
			out[uid].Availability = append(out[uid].Availability, s)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT user_id, show_area, show_gender, show_education, show_lifestyle, discoverable, notify_matches, notify_messages FROM privacy_settings WHERE user_id=ANY($1)`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			var pv models.Privacy
			if err := rs.Scan(&uid, &pv.ShowArea, &pv.ShowGender, &pv.ShowEducation, &pv.ShowLifestyle, &pv.Discoverable, &pv.NotifyMatches, &pv.NotifyMessages); err != nil {
				return err
			}
			out[uid].Privacy = pv
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q(`SELECT DISTINCT user_id FROM subscriptions WHERE user_id=ANY($1) AND status='active' AND (current_period_end IS NULL OR current_period_end>now())`,
		func(rs pgx.Rows) error {
			var uid uuid.UUID
			if err := rs.Scan(&uid); err != nil {
				return err
			}
			out[uid].Premium = true
			return nil
		}); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repo) LoadProfile(ctx context.Context, id uuid.UUID) (*models.Profile, error) {
	m, err := r.LoadProfiles(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	p, ok := m[id]
	if !ok {
		return nil, NotFound(pgx.ErrNoRows)
	}
	return p, nil
}

// CreateProfile inserts the base row plus default preference / privacy rows.
func (r *Repo) CreateProfile(ctx context.Context, uid uuid.UUID, name string, birth time.Time, gender string) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO profiles (user_id, display_name, birth_date, gender) VALUES ($1,$2,$3,$4)
			ON CONFLICT (user_id) DO NOTHING`, uid, name, birth, gender); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO preferences (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, uid); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO privacy_settings (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, uid)
		return err
	})
}

// ApplyProfilePatch writes a validated patch. Slice fields replace the whole relation.
func (r *Repo) ApplyProfilePatch(ctx context.Context, uid uuid.UUID, p models.ProfilePatch, birth *time.Time) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE profiles SET
				display_name=COALESCE($2,display_name), birth_date=COALESCE($3,birth_date), gender=COALESCE($4,gender),
				bio=COALESCE($5,bio), location_id=COALESCE($6,location_id), updated_at=now()
			WHERE user_id=$1`, uid, p.DisplayName, birth, p.Gender, p.Bio, p.LocationID); err != nil {
			return err
		}
		if o := p.Occupation; o != nil {
			if _, err := tx.Exec(ctx, `UPDATE profiles SET occupation_status=NULLIF($2,''), university=NULLIF($3,''),
				field_of_study=NULLIF($4,''), degree=NULLIF($5,''), profession=NULLIF($6,''), industry=NULLIF($7,'') WHERE user_id=$1`,
				uid, o.Status, o.University, o.FieldOfStudy, o.Degree, o.Profession, o.Industry); err != nil {
				return err
			}
		}
		if l := p.Lifestyle; l != nil {
			if _, err := tx.Exec(ctx, `UPDATE profiles SET smoking=NULLIF($2,''), drinking=NULLIF($3,''), pets=NULLIF($4,''),
				diet=NULLIF($5,''), sleep_schedule=NULLIF($6,''), social_level=NULLIF($7,'') WHERE user_id=$1`,
				uid, l.Smoking, l.Drinking, l.Pets, l.Diet, l.SleepSchedule, l.SocialLevel); err != nil {
				return err
			}
		}
		if p.Interests != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM user_interests WHERE user_id=$1`, uid); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_interests (user_id, interest_id) SELECT $1, id FROM interests WHERE slug=ANY($2) AND is_active`, uid, *p.Interests); err != nil {
				return err
			}
		}
		if p.ConnectionTypes != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM user_connection_types WHERE user_id=$1`, uid); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_connection_types (user_id, type_slug) SELECT $1, slug FROM connection_types WHERE slug=ANY($2) AND is_active`, uid, *p.ConnectionTypes); err != nil {
				return err
			}
		}
		if p.Languages != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM user_languages WHERE user_id=$1`, uid); err != nil {
				return err
			}
			for _, l := range *p.Languages {
				if _, err := tx.Exec(ctx, `INSERT INTO user_languages (user_id, language_code, level, wants_practice) VALUES ($1,$2,$3,$4)
					ON CONFLICT (user_id, language_code) DO UPDATE SET level=EXCLUDED.level, wants_practice=EXCLUDED.wants_practice`,
					uid, l.Code, l.Level, l.WantsPractice); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Repo) ApplyPreferencesPatch(ctx context.Context, uid uuid.UUID, p models.PreferencesPatch) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if p.FriendshipKinds != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM user_friendship_kinds WHERE user_id=$1`, uid); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_friendship_kinds (user_id, kind) SELECT $1, slug FROM friendship_kinds WHERE slug=ANY($2) AND is_active`, uid, *p.FriendshipKinds); err != nil {
				return err
			}
		}
		if pr := p.Preferences; pr != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO preferences (user_id, relationship_intention, preferred_genders, age_min, age_max, distance_scope)
				VALUES ($1, NULLIF($2,''), $3, $4, $5, $6)
				ON CONFLICT (user_id) DO UPDATE SET relationship_intention=EXCLUDED.relationship_intention,
					preferred_genders=EXCLUDED.preferred_genders, age_min=EXCLUDED.age_min, age_max=EXCLUDED.age_max,
					distance_scope=EXCLUDED.distance_scope`,
				uid, pr.RelationshipIntention, pr.PreferredGenders, pr.AgeMin, pr.AgeMax, pr.DistanceScope); err != nil {
				return err
			}
		}
		if p.Personality != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM personality_answers WHERE user_id=$1`, uid); err != nil {
				return err
			}
			keys, opts := make([]string, 0, len(*p.Personality)), make([]string, 0, len(*p.Personality))
			for k, v := range *p.Personality {
				keys, opts = append(keys, k), append(opts, v)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO personality_answers (user_id, question_id, option_key)
				SELECT $1, q.id, x.opt FROM unnest($2::text[], $3::text[]) AS x(k, opt) JOIN personality_questions q ON q.key=x.k`, uid, keys, opts); err != nil {
				return err
			}
		}
		if p.Availability != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM availability WHERE user_id=$1`, uid); err != nil {
				return err
			}
			for _, s := range *p.Availability {
				if _, err := tx.Exec(ctx, `INSERT INTO availability (user_id, day, slot) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, uid, s.Day, s.Slot); err != nil {
					return err
				}
			}
		}
		if pv := p.Privacy; pv != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO privacy_settings (user_id, show_area, show_gender, show_education, show_lifestyle, discoverable, notify_matches, notify_messages)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT (user_id) DO UPDATE SET show_area=EXCLUDED.show_area, show_gender=EXCLUDED.show_gender,
					show_education=EXCLUDED.show_education, show_lifestyle=EXCLUDED.show_lifestyle, discoverable=EXCLUDED.discoverable,
					notify_matches=EXCLUDED.notify_matches, notify_messages=EXCLUDED.notify_messages`,
				uid, pv.ShowArea, pv.ShowGender, pv.ShowEducation, pv.ShowLifestyle, pv.Discoverable, pv.NotifyMatches, pv.NotifyMessages); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- photos ----

func (r *Repo) CountPhotos(ctx context.Context, uid uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM user_photos WHERE user_id=$1 AND status='active'`, uid).Scan(&n)
	return n, err
}

func (r *Repo) AddPhoto(ctx context.Context, id, uid uuid.UUID, key string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO user_photos (id, user_id, storage_key, position)
		VALUES ($1,$2,$3,(SELECT COALESCE(max(position)+1,0) FROM user_photos WHERE user_id=$2 AND status='active'))`, id, uid, key)
	return err
}

// RemovePhoto deletes a photo row owned by uid (or any owner when uid is uuid.Nil) and returns its storage key.
func (r *Repo) RemovePhoto(ctx context.Context, photoID, uid uuid.UUID) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx, `DELETE FROM user_photos WHERE id=$1 AND ($2::uuid = '00000000-0000-0000-0000-000000000000' OR user_id=$2) RETURNING storage_key`, photoID, uid).Scan(&key)
	return key, NotFound(err)
}

func (r *Repo) PhotoKey(ctx context.Context, photoID uuid.UUID) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx, `SELECT storage_key FROM user_photos WHERE id=$1 AND status='active'`, photoID).Scan(&key)
	return key, NotFound(err)
}

// ReorderPhotos sets positions from the given ordered ids (ids not owned by uid are ignored).
func (r *Repo) ReorderPhotos(ctx context.Context, uid uuid.UUID, ids []uuid.UUID) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		for i, id := range ids {
			if _, err := tx.Exec(ctx, `UPDATE user_photos SET position=$3 WHERE id=$1 AND user_id=$2`, id, uid, i); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- reference data ----

func (r *Repo) LoadCatalog(ctx context.Context) (*models.Catalog, error) {
	c := &models.Catalog{ConnectionTypes: []models.RefItem{}, FriendshipKinds: []models.RefItem{},
		Interests: []models.Interest{}, Languages: []models.Language{}, Questions: []models.Question{}}
	ref := func(table string) ([]models.RefItem, error) {
		rows, err := r.pool.Query(ctx, `SELECT slug, name, emoji, position FROM `+table+` WHERE is_active ORDER BY position, name`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []models.RefItem{}
		for rows.Next() {
			var i models.RefItem
			if err := rows.Scan(&i.Slug, &i.Name, &i.Emoji, &i.Position); err != nil {
				return nil, err
			}
			out = append(out, i)
		}
		return out, rows.Err()
	}
	var err error
	if c.ConnectionTypes, err = ref("connection_types"); err != nil {
		return nil, err
	}
	if c.FriendshipKinds, err = ref("friendship_kinds"); err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `SELECT id, slug, category, name, emoji FROM interests WHERE is_active ORDER BY category, position, name`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var i models.Interest
		if err := rows.Scan(&i.ID, &i.Slug, &i.Category, &i.Name, &i.Emoji); err != nil {
			rows.Close()
			return nil, err
		}
		c.Interests = append(c.Interests, i)
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `SELECT code, name, native_name FROM languages WHERE is_active ORDER BY name`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l models.Language
		if err := rows.Scan(&l.Code, &l.Name, &l.NativeName); err != nil {
			rows.Close()
			return nil, err
		}
		c.Languages = append(c.Languages, l)
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `SELECT id, key, text, options FROM personality_questions WHERE is_active ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var q models.Question
		var raw []byte
		if err := rows.Scan(&q.ID, &q.Key, &q.Text, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		_ = json.Unmarshal(raw, &q.Options)
		c.Questions = append(c.Questions, q)
	}
	rows.Close()
	return c, nil
}

// ---- locations ----

func (r *Repo) GetLocation(ctx context.Context, id int64) (*models.Location, error) {
	var l models.Location
	err := r.pool.QueryRow(ctx, `SELECT id, country_code, country, region, city, area FROM locations WHERE id=$1 AND is_approved`, id).
		Scan(&l.ID, &l.CountryCode, &l.Country, &l.Region, &l.City, &l.Area)
	return &l, NotFound(err)
}

func (r *Repo) SearchCities(ctx context.Context, country, q string, limit int) ([]models.Location, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, country_code, country, region, city, area FROM locations
		WHERE country_code=$1 AND area='' AND is_approved AND ($2='' OR city ILIKE $2||'%' OR city ILIKE '% '||$2||'%')
		ORDER BY (SELECT count(*) FROM profiles p JOIN locations l2 ON l2.id=p.location_id WHERE l2.country_code=locations.country_code AND lower(l2.city)=lower(locations.city)) DESC, city
		LIMIT $3`, country, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Location{}
	for rows.Next() {
		var l models.Location
		if err := rows.Scan(&l.ID, &l.CountryCode, &l.Country, &l.Region, &l.City, &l.Area); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) ListAreas(ctx context.Context, country, city string) ([]models.Location, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, country_code, country, region, city, area FROM locations
		WHERE country_code=$1 AND lower(city)=lower($2) AND area<>'' AND is_approved ORDER BY area LIMIT 200`, country, city)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Location{}
	for rows.Next() {
		var l models.Location
		if err := rows.Scan(&l.ID, &l.CountryCode, &l.Country, &l.Region, &l.City, &l.Area); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// EnsureLocation returns the id of (country, region, city, area), creating the
// row (and the parent city row) if needed.
func (r *Repo) EnsureLocation(ctx context.Context, l models.Location) (int64, error) {
	var id int64
	err := r.InTx(ctx, func(tx pgx.Tx) error {
		upsert := func(region, area string) (int64, error) {
			var id int64
			err := tx.QueryRow(ctx, `
				WITH ins AS (
					INSERT INTO locations (country_code, country, region, city, area) VALUES ($1,$2,$3,$4,$5)
					ON CONFLICT (country_code, lower(region), lower(city), lower(area)) DO NOTHING RETURNING id)
				SELECT id FROM ins
				UNION ALL
				SELECT id FROM locations WHERE country_code=$1 AND lower(region)=lower($3) AND lower(city)=lower($4) AND lower(area)=lower($5)
				LIMIT 1`, l.CountryCode, l.Country, region, l.City, area).Scan(&id)
			return id, err
		}
		cityID, err := upsert(l.Region, "")
		if err != nil {
			return err
		}
		id = cityID
		if l.Area != "" {
			if id, err = upsert(l.Region, l.Area); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}
