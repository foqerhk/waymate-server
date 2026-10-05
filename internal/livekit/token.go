package livekit

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	URL       string // public URL for iOS clients
	APIKey    string
	APISecret string
}

func (c Config) Enabled() bool {
	return c.URL != "" && c.APIKey != "" && c.APISecret != ""
}

// RoomToken mints a LiveKit-compatible access token (HS256 JWT).
func (c Config) RoomToken(identity, room string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":  c.APIKey,
		"sub":  identity,
		"name": identity,
		"nbf":  now.Add(-10 * time.Second).Unix(),
		"exp":  now.Add(ttl).Unix(),
		"video": map[string]any{
			"roomJoin":       true,
			"room":           room,
			"canPublish":     true,
			"canSubscribe":   true,
			"canPublishData": true,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.APISecret))
}
