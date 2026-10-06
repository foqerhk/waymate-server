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
	// MapsProvider selects the place/route backend: "amap" (CN default) or "google" (intl).
	MapsProvider string
	// AmapWebKey legacy single key; prefer AmapWebKeys for multi-key.
	AmapWebKey  string
	AmapWebKeys string
	// Personal-dev defaults: search 5000/mo/key, LBS (walk+transit) 150000/mo/key.
	AmapSearchMonthlyQuota int
	AmapLBSMonthlyQuota    int
	GoogleMapsAPIKey       string
	GoogleMapsAPIKeys      string
	GoogleSearchMonthlyQuota int
	GoogleLBSMonthlyQuota    int
	CapacityMaxVoiceCalls  int
	CapacityMaxVideoCalls  int
	CapacityVideoWeight    int
	MapsCallsPerFamilyDay  int
	PeakCallFractionPct    int // e.g. 2 => 2%
	PushRelayURL           string
	PushRelayToken         string
	PushRelayAuthToken     string
	// Store links shown on the website; empty => button greyed out.
	DownloadAppStoreURL   string
	DownloadGooglePlayURL string
	DownloadYYBURL        string
	DownloadHuaweiURL     string
}

func Load() Config {
	return Config{
		HTTPAddr:                 getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:              getenv("DATABASE_URL", "postgres://waymate:waymate@localhost:5432/waymate?sslmode=disable"),
		JWTSecret:                getenv("JWT_SECRET", "dev-change-me-waymate-jwt-secret-32b"),
		InviteHMACSecret:         getenv("INVITE_HMAC_SECRET", "dev-change-me-waymate-invite-hmac"),
		InviteTTL:                durationEnv("INVITE_TTL", 24*time.Hour),
		PublicBaseURL:            strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		AllowedOrigins:           splitCSV(getenv("ALLOWED_ORIGINS", "*")),
		APNsEnabled:              boolEnv("APNS_ENABLED", false),
		APNsKeyPath:              getenv("APNS_KEY_PATH", ""),
		APNsKeyID:                getenv("APNS_KEY_ID", ""),
		APNsTeamID:               getenv("APNS_TEAM_ID", ""),
		APNsBundleID:             getenv("APNS_BUNDLE_ID", "com.waymate.app"),
		APNsProduction:           boolEnv("APNS_PRODUCTION", false),
		LocationMinInterval:      durationEnv("LOCATION_MIN_INTERVAL", 2*time.Second),
		LiveKitURL:               getenv("LIVEKIT_URL", "ws://livekit:7880"),
		LiveKitPublicURL:         forceWSS(getenv("LIVEKIT_PUBLIC_URL", "ws://127.0.0.1:17880")),
		LiveKitAPIKey:            getenv("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret:         getenv("LIVEKIT_API_SECRET", "replace-with-livekit-secret"),
		MapsProvider:             getenv("MAPS_PROVIDER", "amap"),
		AmapWebKey:               getenv("AMAP_WEB_KEY", ""),
		AmapWebKeys:              getenv("AMAP_WEB_KEYS", ""),
		AmapSearchMonthlyQuota:   intEnv("AMAP_SEARCH_MONTHLY_QUOTA", 5000),
		AmapLBSMonthlyQuota:      intEnv("AMAP_LBS_MONTHLY_QUOTA", 150000),
		GoogleMapsAPIKey:         getenv("GOOGLE_MAPS_API_KEY", ""),
		GoogleMapsAPIKeys:        getenv("GOOGLE_MAPS_API_KEYS", ""),
		GoogleSearchMonthlyQuota: intEnv("GOOGLE_SEARCH_MONTHLY_QUOTA", 0),
		GoogleLBSMonthlyQuota:    intEnv("GOOGLE_LBS_MONTHLY_QUOTA", 0),
		CapacityMaxVoiceCalls:    intEnv("CAPACITY_MAX_VOICE_CALLS", 30),
		CapacityMaxVideoCalls:    intEnv("CAPACITY_MAX_VIDEO_CALLS", 8),
		CapacityVideoWeight:      intEnv("CAPACITY_VIDEO_WEIGHT", 3),
		MapsCallsPerFamilyDay:    intEnv("MAPS_CALLS_PER_FAMILY_DAY", 20),
		PeakCallFractionPct:      intEnv("PEAK_CALL_FRACTION_PCT", 2),
		PushRelayURL:             strings.TrimRight(getenv("PUSH_RELAY_URL", ""), "/"),
		PushRelayToken:           getenv("PUSH_RELAY_TOKEN", ""),
		PushRelayAuthToken:       getenv("PUSH_RELAY_AUTH_TOKEN", ""),
		DownloadAppStoreURL:      strings.TrimSpace(getenv("DOWNLOAD_APPSTORE_URL", "")),
		DownloadGooglePlayURL:    strings.TrimSpace(getenv("DOWNLOAD_GOOGLE_PLAY_URL", "")),
		DownloadYYBURL:           strings.TrimSpace(getenv("DOWNLOAD_YYB_URL", "")),
		DownloadHuaweiURL:        strings.TrimSpace(getenv("DOWNLOAD_HUAWEI_URL", "")),
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

func intEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
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
