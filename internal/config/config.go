package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	JWTSecret           string
	InviteHMACSecret    string
	InviteTTL           time.Duration
	PublicBaseURL       string
	AllowedOrigins      []string
	APNsEnabled         bool
	APNsKeyPath         string
	APNsKeyID           string
	APNsTeamID          string
	APNsBundleID        string
	APNsProduction      bool
	LocationMinInterval time.Duration
	LiveKitURL          string
	LiveKitPublicURL    string
	LiveKitAPIKey       string
	LiveKitAPISecret    string
	AmapWebKey          string
	// PushRelayURL is the official WayMate API origin used by self-hosted servers
	// to deliver APNs/VoIP without holding the App Store team's .p8 key.
	PushRelayURL string
	// PushRelayToken is sent as Bearer auth when calling PushRelayURL (client side).
	PushRelayToken string
	// PushRelayAuthToken authenticates incoming /v1/push-relay/* calls (relay host side).
	PushRelayAuthToken string
}

func Load() Config {
	return Config{
		HTTPAddr:            getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:         getenv("DATABASE_URL", "postgres://waymate:waymate@localhost:5432/waymate?sslmode=disable"),
		JWTSecret:           getenv("JWT_SECRET", "dev-change-me-waymate-jwt-secret-32b"),
		InviteHMACSecret:    getenv("INVITE_HMAC_SECRET", "dev-change-me-waymate-invite-hmac"),
		InviteTTL:           durationEnv("INVITE_TTL", 24*time.Hour),
		PublicBaseURL:       strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		AllowedOrigins:      splitCSV(getenv("ALLOWED_ORIGINS", "*")),
		APNsEnabled:         boolEnv("APNS_ENABLED", false),
		APNsKeyPath:         getenv("APNS_KEY_PATH", ""),
		APNsKeyID:           getenv("APNS_KEY_ID", ""),
		APNsTeamID:          getenv("APNS_TEAM_ID", ""),
		APNsBundleID:        getenv("APNS_BUNDLE_ID", "com.waymate.app"),
		APNsProduction:      boolEnv("APNS_PRODUCTION", false),
		LocationMinInterval: durationEnv("LOCATION_MIN_INTERVAL", 2*time.Second),
		LiveKitURL:          getenv("LIVEKIT_URL", "ws://livekit:7880"),
		LiveKitPublicURL:    forceWSS(getenv("LIVEKIT_PUBLIC_URL", "ws://127.0.0.1:17880")),
		LiveKitAPIKey:       getenv("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret:    getenv("LIVEKIT_API_SECRET", "replace-with-livekit-secret"),
		AmapWebKey:          getenv("AMAP_WEB_KEY", ""),
		PushRelayURL:        strings.TrimRight(getenv("PUSH_RELAY_URL", ""), "/"),
		PushRelayToken:      getenv("PUSH_RELAY_TOKEN", ""),
		PushRelayAuthToken:  getenv("PUSH_RELAY_AUTH_TOKEN", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// forceWSS rewrites ws:// → wss:// so iOS clients never receive cleartext media URLs.
func forceWSS(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "ws://") {
		return "wss://" + strings.TrimPrefix(u, "ws://")
	}
	if strings.HasPrefix(u, "http://") {
		return "wss://" + strings.TrimPrefix(u, "http://")
	}
	if strings.HasPrefix(u, "https://") {
		return "wss://" + strings.TrimPrefix(u, "https://")
	}
	return u
}
