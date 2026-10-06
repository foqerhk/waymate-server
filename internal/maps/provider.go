package maps

import (
	"context"
	"fmt"
	"strings"
)

// Provider is the shared place-search + route planner used by the API.
// Deploy-time switch: MAPS_PROVIDER=amap|google
type Provider interface {
	Enabled() bool
	Name() string
	SearchPlaces(ctx context.Context, keywords string, lat, lng float64) ([]Place, error)
	PlanWalking(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName string) (*RoutePlan, error)
	PlanTransit(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName, city string) (*RoutePlan, error)
}

func NormalizeProvider(raw string) string {
	p := strings.ToLower(strings.TrimSpace(raw))
	switch p {
	case "google", "gmaps", "google_maps":
		return "google"
	case "amap", "gaode", "":
		return "amap"
	default:
		return p
	}
}

// ErrNotConfigured indicates the selected MAPS_PROVIDER has no usable API key.
var ErrNotConfigured = fmt.Errorf("maps_not_configured")
