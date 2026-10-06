package amap

import (
	"encoding/json"
	"testing"

	"github.com/waymate/backend/internal/maps"
)

var _ maps.Provider = (*Client)(nil)

func TestPlacePOIDistanceFlexible(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"string", `{"distance":"123"}`, "123"},
		{"empty_array", `{"distance":[]}`, ""},
		{"null", `{"distance":null}`, ""},
		{"empty_string", `{"distance":""}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p placePOI
			if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
				t.Fatal(err)
			}
			got := stringifyFlex(p.Distance)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
