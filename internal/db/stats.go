package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type MapsKind string

const (
	MapsKindSearch  MapsKind = "search"
	MapsKindLBS     MapsKind = "lbs" // legacy combined LBS (pre-split)
	MapsKindWalk    MapsKind = "walk"
	MapsKindTransit MapsKind = "transit"
)

// LBSKinds share the same Amap/Google LBS monthly quota pool.
var LBSKinds = []MapsKind{MapsKindLBS, MapsKindWalk, MapsKindTransit}

func (s *Store) IncrMapsUsage(ctx context.Context, provider, keyFP string, kind MapsKind, n int) error {
	if n <= 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO maps_usage_daily (day, provider, key_fp, kind, count)
		VALUES (CURRENT_DATE, $1, $2, $3, $4)
		ON CONFLICT (day, provider, key_fp, kind)
		DO UPDATE SET count = maps_usage_daily.count + EXCLUDED.count
	`, provider, keyFP, string(kind), n)
	return err
}

func (s *Store) MapsUsageMonthByKey(ctx context.Context, provider, keyFP string, kind MapsKind) (int, error) {
	var n int
	// Walk/transit share the LBS quota pool (include legacy 'lbs').
	if kind == MapsKindLBS || kind == MapsKindWalk || kind == MapsKindTransit {
		err := s.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(count), 0)::int
			FROM maps_usage_daily
			WHERE provider = $1 AND key_fp = $2
			  AND kind IN ('lbs', 'walk', 'transit')
			  AND day >= date_trunc('month', CURRENT_DATE)::date
		`, provider, keyFP).Scan(&n)
		return n, err
	}
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(count), 0)::int
		FROM maps_usage_daily
		WHERE provider = $1 AND key_fp = $2 AND kind = $3
		  AND day >= date_trunc('month', CURRENT_DATE)::date
	`, provider, keyFP, string(kind)).Scan(&n)
	return n, err
}

type MapsUsageAgg struct {
	SearchMonth  int `json:"searchMonth"`
	LBSMonth     int `json:"lbsMonth"` // walk + transit + legacy lbs
	WalkMonth    int `json:"walkMonth"`
	TransitMonth int `json:"transitMonth"`
	SearchToday  int `json:"searchToday"`
	LBSToday     int `json:"lbsToday"`
	WalkToday    int `json:"walkToday"`
	TransitToday int `json:"transitToday"`
	SearchLast3d int `json:"searchLast3d"`
	WalkLast3d   int `json:"walkLast3d"`
	TransitLast3d int `json:"transitLast3d"`
	LBSLast3d    int `json:"lbsLast3d"`
}

func (s *Store) MapsUsageAggregate(ctx context.Context, provider string) (MapsUsageAgg, error) {
	var a MapsUsageAgg
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'search' AND day >= date_trunc('month', CURRENT_DATE)::date
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind IN ('lbs','walk','transit') AND day >= date_trunc('month', CURRENT_DATE)::date
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'walk' AND day >= date_trunc('month', CURRENT_DATE)::date
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'transit' AND day >= date_trunc('month', CURRENT_DATE)::date
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'search' AND day = CURRENT_DATE
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind IN ('lbs','walk','transit') AND day = CURRENT_DATE
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'walk' AND day = CURRENT_DATE
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'transit' AND day = CURRENT_DATE
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'search' AND day >= CURRENT_DATE - 2
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'walk' AND day >= CURRENT_DATE - 2
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind = 'transit' AND day >= CURRENT_DATE - 2
			), 0)::int,
			COALESCE(SUM(count) FILTER (
				WHERE kind IN ('lbs','walk','transit') AND day >= CURRENT_DATE - 2
			), 0)::int
		FROM maps_usage_daily
		WHERE provider = $1
	`, provider).Scan(
		&a.SearchMonth, &a.LBSMonth, &a.WalkMonth, &a.TransitMonth,
		&a.SearchToday, &a.LBSToday, &a.WalkToday, &a.TransitToday,
		&a.SearchLast3d, &a.WalkLast3d, &a.TransitLast3d, &a.LBSLast3d,
	)
	return a, err
}

type DayCount struct {
	Day   time.Time `json:"day"`
	Count int       `json:"count"`
}

func (s *Store) MapsUsageSeries(ctx context.Context, provider string, days int) ([]DayCount, error) {
	return s.mapsUsageSeriesKinds(ctx, provider, days, nil)
}

func (s *Store) MapsUsageSeriesKind(ctx context.Context, provider string, days int, kind MapsKind) ([]DayCount, error) {
	return s.mapsUsageSeriesKinds(ctx, provider, days, []MapsKind{kind})
}

func (s *Store) MapsUsageSeriesLBS(ctx context.Context, provider string, days int) ([]DayCount, error) {
	return s.mapsUsageSeriesKinds(ctx, provider, days, LBSKinds)
}

func (s *Store) mapsUsageSeriesKinds(ctx context.Context, provider string, days int, kinds []MapsKind) ([]DayCount, error) {
	if days < 1 {
		days = 7
	}
	var rows pgx.Rows
	var err error
	if len(kinds) == 0 {
		rows, err = s.pool.Query(ctx, `
			WITH days AS (
				SELECT generate_series(
					CURRENT_DATE - ($2::int - 1),
					CURRENT_DATE,
					'1 day'::interval
				)::date AS day
			)
			SELECT d.day, COALESCE(SUM(u.count), 0)::int
			FROM days d
			LEFT JOIN maps_usage_daily u
			  ON u.day = d.day AND u.provider = $1
			GROUP BY d.day
			ORDER BY d.day
		`, provider, days)
	} else if len(kinds) == 1 {
		rows, err = s.pool.Query(ctx, `
			WITH days AS (
				SELECT generate_series(
					CURRENT_DATE - ($3::int - 1),
					CURRENT_DATE,
					'1 day'::interval
				)::date AS day
			)
			SELECT d.day, COALESCE(SUM(u.count), 0)::int
			FROM days d
			LEFT JOIN maps_usage_daily u
			  ON u.day = d.day AND u.provider = $1 AND u.kind = $2
			GROUP BY d.day
			ORDER BY d.day
		`, provider, string(kinds[0]), days)
	} else {
		kindStrs := make([]string, len(kinds))
		for i, k := range kinds {
			kindStrs[i] = string(k)
		}
		rows, err = s.pool.Query(ctx, `
			WITH days AS (
				SELECT generate_series(
					CURRENT_DATE - ($3::int - 1),
					CURRENT_DATE,
					'1 day'::interval
				)::date AS day
			)
			SELECT d.day, COALESCE(SUM(u.count), 0)::int
			FROM days d
			LEFT JOIN maps_usage_daily u
			  ON u.day = d.day AND u.provider = $1 AND u.kind = ANY($2::text[])
			GROUP BY d.day
			ORDER BY d.day
		`, provider, kindStrs, days)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayCount
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) InsertCapacitySample(ctx context.Context, voice, video, wsApprox int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO capacity_samples (ts, active_voice, active_video, ws_approx)
		VALUES (now(), $1, $2, $3)
	`, voice, video, wsApprox)
	return err
}

func (s *Store) TrimCapacitySamples(ctx context.Context, keepDays int) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM capacity_samples WHERE ts < now() - ($1::int * interval '1 day')
	`, keepDays)
	return err
}

type ActiveCalls struct {
	Voice int
	Video int
}

func (s *Store) CountActiveCalls(ctx context.Context) (ActiveCalls, error) {
	var a ActiveCalls
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE media_type = 'voice')::int,
			COUNT(*) FILTER (WHERE media_type = 'video')::int
		FROM calls
		WHERE status IN ('ringing', 'active')
	`).Scan(&a.Voice, &a.Video)
	return a, err
}

type CallDayStats struct {
	Day   time.Time `json:"day"`
	Count int       `json:"count"`
	Peak  int       `json:"peak"`
}

func (s *Store) CallSeries(ctx context.Context, days int) ([]CallDayStats, error) {
	if days < 1 {
		days = 7
	}
	rows, err := s.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series(
				CURRENT_DATE - ($1::int - 1),
				CURRENT_DATE,
				'1 day'::interval
			)::date AS day
		),
		counts AS (
			SELECT created_at::date AS day, COUNT(*)::int AS cnt
			FROM calls
			WHERE created_at >= CURRENT_DATE - ($1::int - 1)
			GROUP BY 1
		),
		peaks AS (
			SELECT ts::date AS day, MAX(active_voice + active_video)::int AS peak
			FROM capacity_samples
			WHERE ts >= CURRENT_DATE - ($1::int - 1)
			GROUP BY 1
		)
		SELECT d.day,
			COALESCE(c.cnt, 0),
			COALESCE(p.peak, 0)
		FROM days d
		LEFT JOIN counts c ON c.day = d.day
		LEFT JOIN peaks p ON p.day = d.day
		ORDER BY d.day
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallDayStats
	for rows.Next() {
		var r CallDayStats
		if err := rows.Scan(&r.Day, &r.Count, &r.Peak); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type UserCounts struct {
	Devices       int `json:"devices"`
	Elders        int `json:"elders"`
	Children      int `json:"children"`
	Families      int `json:"families"`
	Active7d      int `json:"active7d"`
}

func (s *Store) UserCounts(ctx context.Context) (UserCounts, error) {
	var u UserCounts
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*)::int FROM devices),
			(SELECT COUNT(*)::int FROM devices WHERE role = 'elder'),
			(SELECT COUNT(*)::int FROM devices WHERE role = 'child'),
			(SELECT COUNT(*)::int FROM families),
			(SELECT COUNT(*)::int FROM devices WHERE last_seen_at >= now() - interval '7 days')
	`).Scan(&u.Devices, &u.Elders, &u.Children, &u.Families, &u.Active7d)
	return u, err
}

type ActiveDeviceDay struct {
	Day   time.Time `json:"day"`
	Count int       `json:"count"`
}

// ActiveDeviceSeries approximates daily active devices from last_seen buckets (coarse).
func (s *Store) ActiveDeviceSeries(ctx context.Context, days int) ([]ActiveDeviceDay, error) {
	if days < 1 {
		days = 7
	}
	rows, err := s.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series(
				CURRENT_DATE - ($1::int - 1),
				CURRENT_DATE,
				'1 day'::interval
			)::date AS day
		)
		SELECT d.day,
			(
				SELECT COUNT(*)::int FROM devices
				WHERE last_seen_at::date = d.day
			)
		FROM days d
		ORDER BY d.day
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActiveDeviceDay
	for rows.Next() {
		var r ActiveDeviceDay
		if err := rows.Scan(&r.Day, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SetMapsQuotaExhausted(ctx context.Context, provider, infocode string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO maps_quota_state (provider, exhausted_at, last_infocode)
		VALUES ($1, now(), $2)
		ON CONFLICT (provider) DO UPDATE
		SET exhausted_at = now(), last_infocode = EXCLUDED.last_infocode
	`, provider, infocode)
	return err
}

func (s *Store) MapsQuotaState(ctx context.Context, provider string) (exhaustedAt *time.Time, infocode string, err error) {
	var t *time.Time
	var code string
	err = s.pool.QueryRow(ctx, `
		SELECT exhausted_at, last_infocode FROM maps_quota_state WHERE provider = $1
	`, provider).Scan(&t, &code)
	if err == pgx.ErrNoRows {
		return nil, "", nil
	}
	return t, code, err
}

func (s *Store) PeakConcurrent24h(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(active_voice + active_video), 0)::int
		FROM capacity_samples
		WHERE ts >= now() - interval '24 hours'
	`).Scan(&n)
	return n, err
}
