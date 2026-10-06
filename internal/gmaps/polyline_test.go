package gmaps

import (
	"testing"

	"github.com/waymate/backend/internal/maps"
)

var _ maps.Provider = (*Client)(nil)

func TestDecodePolyline(t *testing.T) {
	// Encoded polyline for (38.5, -120.2), (40.7, -120.95), (43.252, -126.453)
	// from Google's polyline algorithm docs.
	coords := decodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@")
	if len(coords) != 3 {
		t.Fatalf("len=%d want 3", len(coords))
	}
	assertNear(t, coords[0].Latitude, 38.5)
	assertNear(t, coords[0].Longitude, -120.2)
	assertNear(t, coords[1].Latitude, 40.7)
	assertNear(t, coords[1].Longitude, -120.95)
	assertNear(t, coords[2].Latitude, 43.252)
	assertNear(t, coords[2].Longitude, -126.453)
}

func assertNear(t *testing.T, got, want float64) {
	t.Helper()
	if diff := got - want; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("got %v want %v", got, want)
	}
}
