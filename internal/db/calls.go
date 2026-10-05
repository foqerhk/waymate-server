package db

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CallRecord struct {
	ID             uuid.UUID  `json:"id"`
	FamilyID       uuid.UUID  `json:"familyId"`
	CallerDeviceID uuid.UUID  `json:"callerDeviceId"`
	CalleeDeviceID uuid.UUID  `json:"calleeDeviceId"`
	MediaType      string     `json:"mediaType"`
	ChannelName    string     `json:"channelName"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	AnsweredAt     *time.Time `json:"answeredAt,omitempty"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
}

func (s *Store) UpdateVoIPToken(ctx context.Context, deviceID uuid.UUID, token string) error {
	_, err := s.pool.Exec(ctx, `UPDATE devices SET voip_token = $2, last_seen_at = now() WHERE id = $1`, deviceID, token)
	return err
}

func (s *Store) VoIPToken(ctx context.Context, deviceID uuid.UUID) (string, error) {
	var token *string
	err := s.pool.QueryRow(ctx, `SELECT voip_token FROM devices WHERE id = $1`, deviceID).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", nil
	}
	return *token, nil
}

func (s *Store) CreateCall(ctx context.Context, familyID, caller, callee uuid.UUID, mediaType, channel string) (*CallRecord, error) {
	var c CallRecord
	err := s.pool.QueryRow(ctx, `
		INSERT INTO calls (family_id, caller_device_id, callee_device_id, media_type, channel_name, status)
		VALUES ($1,$2,$3,$4,$5,'ringing')
		RETURNING id, family_id, caller_device_id, callee_device_id, media_type, channel_name, status, created_at, answered_at, ended_at
	`, familyID, caller, callee, mediaType, channel).Scan(
		&c.ID, &c.FamilyID, &c.CallerDeviceID, &c.CalleeDeviceID, &c.MediaType, &c.ChannelName, &c.Status, &c.CreatedAt, &c.AnsweredAt, &c.EndedAt,
	)
	return &c, err
}

func (s *Store) GetCall(ctx context.Context, id uuid.UUID) (*CallRecord, error) {
	var c CallRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, family_id, caller_device_id, callee_device_id, media_type, channel_name, status, created_at, answered_at, ended_at
		FROM calls WHERE id = $1
	`, id).Scan(
		&c.ID, &c.FamilyID, &c.CallerDeviceID, &c.CalleeDeviceID, &c.MediaType, &c.ChannelName, &c.Status, &c.CreatedAt, &c.AnsweredAt, &c.EndedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) AnswerCall(ctx context.Context, id uuid.UUID) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE calls SET status = 'active', answered_at = now()
		WHERE id = $1 AND status = 'ringing'
	`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) EndCall(ctx context.Context, id uuid.UUID, status string) error {
	if status == "" {
		status = "ended"
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE calls SET status = $2, ended_at = now()
		WHERE id = $1 AND status IN ('ringing','active')
	`, id, status)
	return err
}
