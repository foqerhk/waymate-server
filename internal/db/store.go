package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/foqerhk/waymate-server/internal/models"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrForbidden     = errors.New("forbidden")
	ErrInviteInvalid = errors.New("invite invalid")
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) RegisterDevice(ctx context.Context, installationID string, role models.Role, displayName, platform string) (*models.Device, bool, error) {
	var d models.Device
	err := s.pool.QueryRow(ctx, `
		INSERT INTO devices (installation_id, role, display_name, platform)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (installation_id) DO UPDATE
		  SET display_name = EXCLUDED.display_name,
		      role = EXCLUDED.role,
		      platform = EXCLUDED.platform,
		      last_seen_at = now()
		RETURNING id, installation_id, role, display_name, platform, apns_token, apns_sandbox, created_at, last_seen_at
	`, installationID, role, displayName, platform).Scan(
		&d.ID, &d.InstallationID, &d.Role, &d.DisplayName, &d.Platform, &d.APNsToken, &d.APNsSandbox, &d.CreatedAt, &d.LastSeenAt,
	)
	if err != nil {
		return nil, false, err
	}
	// created = true if brand new: approximate via created_at ~ now
	created := time.Since(d.CreatedAt) < 2*time.Second
	return &d, created, nil
}

func (s *Store) GetDevice(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	var d models.Device
	err := s.pool.QueryRow(ctx, `
		SELECT id, installation_id, role, display_name, platform, apns_token, apns_sandbox, created_at, last_seen_at
		FROM devices WHERE id = $1
	`, id).Scan(&d.ID, &d.InstallationID, &d.Role, &d.DisplayName, &d.Platform, &d.APNsToken, &d.APNsSandbox, &d.CreatedAt, &d.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

func (s *Store) TouchDevice(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE devices SET last_seen_at = now() WHERE id = $1`, id)
	return err
}

func (s *Store) UpdateAPNs(ctx context.Context, id uuid.UUID, token string, sandbox bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE devices SET apns_token = $2, apns_sandbox = $3, last_seen_at = now() WHERE id = $1
	`, id, token, sandbox)
	return err
}

func (s *Store) CreateFamily(ctx context.Context, creatorID uuid.UUID, name, creatorDisplayName string) (*models.Family, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT family_id FROM family_members WHERE device_id = $1`, creatorID).Scan(&existing)
	if err == nil {
		return nil, ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var f models.Family
	err = tx.QueryRow(ctx, `
		INSERT INTO families (name, created_by_device_id) VALUES ($1, $2)
		RETURNING id, name, created_by_device_id, created_at
	`, name, creatorID).Scan(&f.ID, &f.Name, &f.CreatedByDeviceID, &f.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO family_members (family_id, device_id, role, display_name, avatar_symbol, font_scale)
		VALUES ($1, $2, 'child', $3, 'person.crop.circle.fill', 1.0)
	`, f.ID, creatorID, creatorDisplayName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) FamilyIDForDevice(ctx context.Context, deviceID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT family_id FROM family_members WHERE device_id = $1`, deviceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

func (s *Store) SaveInvite(ctx context.Context, familyID, createdBy uuid.UUID, code string) error {
	// Revoke previous active invites so the family has one current permanent code.
	_, _ = s.pool.Exec(ctx, `
		UPDATE invite_codes SET revoked = TRUE
		WHERE family_id = $1 AND revoked = FALSE
	`, familyID)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO invite_codes (family_id, code, created_by, expires_at, max_uses)
		VALUES ($1, $2, $3, '9999-12-31 23:59:59+00', 1000000)
	`, familyID, code, createdBy)
	return err
}

func (s *Store) LatestInvite(ctx context.Context, familyID uuid.UUID) (code string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT code FROM invite_codes
		WHERE family_id = $1 AND revoked = FALSE
		ORDER BY created_at DESC LIMIT 1
	`, familyID).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return code, err
}

func (s *Store) JoinFamily(ctx context.Context, code string, expectedFamily uuid.UUID, elderDeviceID uuid.UUID, displayName, relationshipKey string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var inviteID uuid.UUID
	var familyID uuid.UUID
	var revoked bool
	err = tx.QueryRow(ctx, `
		SELECT id, family_id, revoked
		FROM invite_codes WHERE code = $1 FOR UPDATE
	`, code).Scan(&inviteID, &familyID, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInviteInvalid
	}
	if err != nil {
		return uuid.Nil, err
	}
	// Permanent invite: only revoked codes are rejected (no time expiry).
	if revoked {
		return uuid.Nil, ErrInviteInvalid
	}
	if expectedFamily != uuid.Nil && expectedFamily != familyID {
		return uuid.Nil, ErrInviteInvalid
	}

	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT family_id FROM family_members WHERE device_id = $1`, elderDeviceID).Scan(&existing)
	if err == nil {
		if existing == familyID {
			return familyID, tx.Commit(ctx)
		}
		return uuid.Nil, ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}

	var rel *string
	if relationshipKey != "" {
		rel = &relationshipKey
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO family_members (family_id, device_id, role, display_name, relationship_key, avatar_symbol, font_scale)
		VALUES ($1, $2, 'elder', $3, $4, 'figure.walk', 1.15)
	`, familyID, elderDeviceID, displayName, rel)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE invite_codes SET use_count = use_count + 1 WHERE id = $1`, inviteID)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE devices SET role = 'elder', display_name = $2 WHERE id = $1`, elderDeviceID, displayName)
	if err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return familyID, nil
}

// ClaimElder adds an unpaired elder device into the child's family (family scanned elder QR).
func (s *Store) ClaimElder(ctx context.Context, childDeviceID, elderDeviceID uuid.UUID, displayName string) (uuid.UUID, error) {
	familyID, err := s.FamilyIDForDevice(ctx, childDeviceID)
	if err != nil {
		return uuid.Nil, err
	}

	elder, err := s.GetDevice(ctx, elderDeviceID)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	if elder.Role != models.RoleElder {
		return uuid.Nil, ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT family_id FROM family_members WHERE device_id = $1`, elderDeviceID).Scan(&existing)
	if err == nil {
		if existing == familyID {
			return familyID, tx.Commit(ctx)
		}
		return uuid.Nil, ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}

	name := strings.TrimSpace(displayName)
	if name == "" {
		name = elder.DisplayName
	}
	if name == "" {
		name = "Elder"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO family_members (family_id, device_id, role, display_name, avatar_symbol, font_scale)
		VALUES ($1, $2, 'elder', $3, 'figure.walk', 1.15)
	`, familyID, elderDeviceID, name)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE devices SET role = 'elder', display_name = $2 WHERE id = $1`, elderDeviceID, name)
	if err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return familyID, nil
}

func (s *Store) ListMembers(ctx context.Context, familyID uuid.UUID) ([]models.Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT family_id, device_id, role, display_name, relationship_key, avatar_symbol, font_scale, joined_at
		FROM family_members WHERE family_id = $1 ORDER BY joined_at
	`, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Member
	for rows.Next() {
		var m models.Member
		if err := rows.Scan(&m.FamilyID, &m.DeviceID, &m.Role, &m.DisplayName, &m.RelationshipKey, &m.AvatarSymbol, &m.FontScale, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MemberDisplayName returns the family nickname for a device (onboarding display name).
func (s *Store) MemberDisplayName(ctx context.Context, deviceID uuid.UUID) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `
		SELECT display_name FROM family_members WHERE device_id = $1
	`, deviceID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

// UpdateDisplayName updates both device and family_member nicknames.
func (s *Store) UpdateDisplayName(ctx context.Context, deviceID uuid.UUID, displayName string) error {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return fmt.Errorf("empty display name")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `
		UPDATE devices SET display_name = $2, last_seen_at = now() WHERE id = $1
	`, deviceID, displayName)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx, `
		UPDATE family_members SET display_name = $2 WHERE device_id = $1
	`, deviceID, displayName)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetFamily(ctx context.Context, id uuid.UUID) (*models.Family, error) {
	var f models.Family
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, created_by_device_id, created_at FROM families WHERE id = $1
	`, id).Scan(&f.ID, &f.Name, &f.CreatedByDeviceID, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &f, err
}

func (s *Store) UpsertLocation(ctx context.Context, loc models.Location) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO elder_locations (elder_device_id, latitude, longitude, accuracy, heading, speed, is_background, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT (elder_device_id) DO UPDATE SET
		  latitude = EXCLUDED.latitude,
		  longitude = EXCLUDED.longitude,
		  accuracy = EXCLUDED.accuracy,
		  heading = EXCLUDED.heading,
		  speed = EXCLUDED.speed,
		  is_background = EXCLUDED.is_background,
		  updated_at = now()
	`, loc.ElderDeviceID, loc.Latitude, loc.Longitude, loc.Accuracy, loc.Heading, loc.Speed, loc.IsBackground)
	return err
}

func (s *Store) GetLocation(ctx context.Context, elderID uuid.UUID) (*models.Location, error) {
	var loc models.Location
	err := s.pool.QueryRow(ctx, `
		SELECT elder_device_id, latitude, longitude, accuracy, heading, speed, is_background, updated_at
		FROM elder_locations WHERE elder_device_id = $1
	`, elderID).Scan(&loc.ElderDeviceID, &loc.Latitude, &loc.Longitude, &loc.Accuracy, &loc.Heading, &loc.Speed, &loc.IsBackground, &loc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &loc, err
}

func (s *Store) UpsertTrip(ctx context.Context, elderID, createdBy uuid.UUID, routeJSON, guidanceJSON json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO active_trips (elder_device_id, created_by_device_id, route_json, guidance_json, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (elder_device_id) DO UPDATE SET
		  created_by_device_id = EXCLUDED.created_by_device_id,
		  route_json = EXCLUDED.route_json,
		  guidance_json = COALESCE(EXCLUDED.guidance_json, active_trips.guidance_json),
		  updated_at = now()
	`, elderID, createdBy, routeJSON, guidanceJSON)
	return err
}

func (s *Store) UpdateGuidance(ctx context.Context, elderID uuid.UUID, guidanceJSON json.RawMessage) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE active_trips SET guidance_json = $2, updated_at = now() WHERE elder_device_id = $1
	`, elderID, guidanceJSON)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetTrip(ctx context.Context, elderID uuid.UUID) (*models.ActiveTrip, error) {
	var t models.ActiveTrip
	err := s.pool.QueryRow(ctx, `
		SELECT elder_device_id, created_by_device_id, route_json, guidance_json, updated_at
		FROM active_trips WHERE elder_device_id = $1
	`, elderID).Scan(&t.ElderDeviceID, &t.CreatedByDeviceID, &t.RouteJSON, &t.GuidanceJSON, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// ClearTrip removes the active navigation trip for an elder (arrive or family stop).
func (s *Store) ClearTrip(ctx context.Context, elderID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM active_trips WHERE elder_device_id = $1`, elderID)
	return err
}

func (s *Store) AddFavorite(ctx context.Context, familyID, elderID uuid.UUID, payload json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO favorites (family_id, elder_device_id, payload_json) VALUES ($1,$2,$3)
	`, familyID, elderID, payload)
	return err
}

func (s *Store) UpdateFavorite(ctx context.Context, elderID uuid.UUID, favoriteID string, payload json.RawMessage) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE favorites
		SET payload_json = $3
		WHERE elder_device_id = $1 AND payload_json->>'id' = $2
	`, elderID, favoriteID, payload)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteFavorite(ctx context.Context, elderID uuid.UUID, favoriteID string) error {
	ct, err := s.pool.Exec(ctx, `
		DELETE FROM favorites
		WHERE elder_device_id = $1 AND payload_json->>'id' = $2
	`, elderID, favoriteID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListFavorites(ctx context.Context, elderID uuid.UUID) ([]json.RawMessage, error) {
	rows, err := s.pool.Query(ctx, `SELECT payload_json FROM favorites WHERE elder_device_id = $1 ORDER BY created_at`, elderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

func (s *Store) DeviceLastSeen(ctx context.Context, id uuid.UUID) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx, `SELECT last_seen_at FROM devices WHERE id = $1`, id).Scan(&t)
	return t, err
}

func (s *Store) AssertSameFamily(ctx context.Context, a, b uuid.UUID) error {
	var fa, fb uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT family_id FROM family_members WHERE device_id = $1`, a).Scan(&fa)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	err = s.pool.QueryRow(ctx, `SELECT family_id FROM family_members WHERE device_id = $1`, b).Scan(&fb)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if fa != fb {
		return ErrForbidden
	}
	return nil
}

func (s *Store) FamilyMemberIDs(ctx context.Context, familyID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `SELECT device_id FROM family_members WHERE family_id = $1`, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ElderAPNsTokens(ctx context.Context, elderID uuid.UUID) (token string, sandbox bool, err error) {
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(apns_token,''), apns_sandbox FROM devices WHERE id = $1`, elderID).Scan(&token, &sandbox)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrNotFound
	}
	return token, sandbox, err
}

// RemoveElderFromFamily unbinds an elder device from the family and clears trip/location/favorites.
func (s *Store) RemoveElderFromFamily(ctx context.Context, actorID, elderID uuid.UUID) error {
	if err := s.AssertSameFamily(ctx, actorID, elderID); err != nil {
		return err
	}
	var role models.Role
	err := s.pool.QueryRow(ctx, `SELECT role FROM family_members WHERE device_id = $1`, elderID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role != models.RoleElder {
		return ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, _ = tx.Exec(ctx, `DELETE FROM active_trips WHERE elder_device_id = $1`, elderID)
	_, _ = tx.Exec(ctx, `DELETE FROM elder_locations WHERE elder_device_id = $1`, elderID)
	_, _ = tx.Exec(ctx, `DELETE FROM favorites WHERE elder_device_id = $1`, elderID)
	ct, err := tx.Exec(ctx, `DELETE FROM family_members WHERE device_id = $1 AND role = 'elder'`, elderID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// LeaveFamily lets an elder leave their current family (same cleanup as remove).
func (s *Store) LeaveFamily(ctx context.Context, elderID uuid.UUID) error {
	var role models.Role
	err := s.pool.QueryRow(ctx, `SELECT role FROM family_members WHERE device_id = $1`, elderID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role != models.RoleElder {
		return ErrForbidden
	}
	return s.RemoveElderFromFamily(ctx, elderID, elderID)
}

func PresenceFromLastSeen(lastSeen time.Time) string {
	if time.Since(lastSeen) <= 2*time.Minute {
		return "online"
	}
	return "offline"
}

func (s *Store) BuildSession(ctx context.Context, deviceID uuid.UUID, invite *models.Invite) (*models.SessionSnapshot, error) {
	device, err := s.GetDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	snap := &models.SessionSnapshot{CurrentDevice: *device, Members: []models.Member{}, Elders: []models.ElderSnapshot{}, Favorites: []json.RawMessage{}}
	familyID, err := s.FamilyIDForDevice(ctx, deviceID)
	if errors.Is(err, ErrNotFound) {
		return snap, nil
	}
	if err != nil {
		return nil, err
	}
	family, err := s.GetFamily(ctx, familyID)
	if err != nil {
		return nil, err
	}
	snap.Family = family
	members, err := s.ListMembers(ctx, familyID)
	if err != nil {
		return nil, err
	}
	snap.Members = members
	snap.Invite = invite

	for _, m := range members {
		if m.Role != models.RoleElder {
			continue
		}
		es := models.ElderSnapshot{Member: m, Presence: "offline"}
		if last, err := s.DeviceLastSeen(ctx, m.DeviceID); err == nil {
			es.Presence = PresenceFromLastSeen(last)
		}
		if loc, err := s.GetLocation(ctx, m.DeviceID); err == nil {
			es.Location = loc
		}
		if trip, err := s.GetTrip(ctx, m.DeviceID); err == nil {
			es.Trip = trip
		}
		favs, _ := s.ListFavorites(ctx, m.DeviceID)
		if device.Role == models.RoleChild || device.ID == m.DeviceID {
			snap.Favorites = append(snap.Favorites, favs...)
		}
		snap.Elders = append(snap.Elders, es)
	}
	return snap, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func FmtErr(err error) string {
	return fmt.Sprintf("%v", err)
}
