package gmaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/waymate/backend/internal/maps"
)

// Client talks to Google Maps Platform (Places Text Search + Directions).
// Coordinates are WGS-84 end-to-end (no GCJ conversion).
type Client struct {
	key  string
	http *http.Client
	base string
}

func New(key string) *Client {
	return &Client{
		key:  strings.TrimSpace(key),
		http: &http.Client{Timeout: 12 * time.Second},
		base: "https://maps.googleapis.com",
	}
}

func (c *Client) Enabled() bool { return c != nil && c.key != "" }
func (c *Client) Name() string  { return "google" }

func (c *Client) SearchPlaces(ctx context.Context, keywords string, lat, lng float64) ([]maps.Place, error) {
	if !c.Enabled() {
		return nil, maps.ErrNotConfigured
	}
	keywords = strings.TrimSpace(keywords)
	if keywords == "" {
		return nil, fmt.Errorf("keywords required")
	}

	q := url.Values{}
	q.Set("query", keywords)
	q.Set("location", fmt.Sprintf("%.6f,%.6f", lat, lng))
	q.Set("radius", "12000")
	q.Set("key", c.key)
	q.Set("language", "en")

	body, err := c.get(ctx, "/maps/api/place/textsearch/json", q)
	if err != nil {
		return nil, err
	}
	var decoded placeTextResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Status != "OK" && decoded.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("google place search: %s (%s)", decoded.Status, decoded.ErrorMessage)
	}

	out := make([]maps.Place, 0, len(decoded.Results))
	for _, r := range decoded.Results {
		plat := r.Geometry.Location.Lat
		plng := r.Geometry.Location.Lng
		place := maps.Place{
			ID:        firstNonEmpty(r.PlaceID, fmt.Sprintf("%.5f,%.5f", plat, plng)),
			Name:      r.Name,
			Address:   firstNonEmpty(r.FormattedAddress, r.Vicinity),
			Type:      firstType(r.Types),
			Latitude:  plat,
			Longitude: plng,
		}
		if lat != 0 || lng != 0 {
			meters := int(haversineMeters(lat, lng, plat, plng) + 0.5)
			place.DistanceMeters = &meters
		}
		out = append(out, place)
	}
	return out, nil
}

func (c *Client) PlanWalking(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName string) (*maps.RoutePlan, error) {
	return c.plan(ctx, "walking", originLat, originLng, destLat, destLng, destinationName, createdByName)
}

func (c *Client) PlanTransit(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName, _ string) (*maps.RoutePlan, error) {
	return c.plan(ctx, "transit", originLat, originLng, destLat, destLng, destinationName, createdByName)
}

func (c *Client) plan(ctx context.Context, mode string, originLat, originLng, destLat, destLng float64, destinationName, createdByName string) (*maps.RoutePlan, error) {
	if !c.Enabled() {
		return nil, maps.ErrNotConfigured
	}

	q := url.Values{}
	q.Set("origin", fmt.Sprintf("%.6f,%.6f", originLat, originLng))
	q.Set("destination", fmt.Sprintf("%.6f,%.6f", destLat, destLng))
	q.Set("mode", mode)
	q.Set("key", c.key)
	q.Set("language", "en")
	q.Set("units", "metric")
	if mode == "transit" {
		q.Set("transit_mode", "bus|subway|train|tram|rail")
	}

	body, err := c.get(ctx, "/maps/api/directions/json", q)
	if err != nil {
		return nil, err
	}
	var decoded directionsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Status != "OK" {
		return nil, fmt.Errorf("google directions (%s): %s (%s)", mode, decoded.Status, decoded.ErrorMessage)
	}
	if len(decoded.Routes) == 0 || len(decoded.Routes[0].Legs) == 0 {
		return nil, fmt.Errorf("no %s route", mode)
	}
	route := decoded.Routes[0]
	leg := route.Legs[0]

	var steps []maps.RouteStep
	var poly []maps.Coordinate
	var summaryParts []string

	for i, s := range leg.Steps {
		coords := decodePolyline(s.Polyline.Points)
		if len(coords) == 0 {
			coords = []maps.Coordinate{
				{Latitude: s.StartLocation.Lat, Longitude: s.StartLocation.Lng},
				{Latitude: s.EndLocation.Lat, Longitude: s.EndLocation.Lng},
			}
		}
		poly = append(poly, coords...)

		instruction := stripHTML(s.HTMLInstructions)
		if instruction == "" {
			instruction = "Continue"
		}
		step := maps.RouteStep{
			ID:             fmt.Sprintf("%s-%d-%s", mode, i, uuid.NewString()[:8]),
			Instruction:    instruction,
			Maneuver:       mapManeuver(s.Maneuver),
			DistanceMeters: float64(s.Distance.Value),
			Polyline:       coords,
		}

		travel := strings.ToUpper(s.TravelMode)
		if travel == "TRANSIT" && s.TransitDetails != nil {
			td := s.TransitDetails
			lineName := firstNonEmpty(td.Line.ShortName, td.Line.Name, "Transit")
			summaryParts = append(summaryParts, lineName)
			dep := td.DepartureStop.Name
			arr := td.ArrivalStop.Name
			vehicle := vehicleKind(td.Line.Vehicle.Type, td.Line.Vehicle.Name)
			legTransit := &maps.TransitLeg{
				Vehicle:  vehicle,
				LineName: strPtr(lineName),
			}
			if dep != "" {
				legTransit.DepartureStop = &dep
			}
			if arr != "" {
				legTransit.ArrivalStop = &arr
			}
			if td.NumStops > 0 {
				n := td.NumStops
				legTransit.ViaStopsCount = &n
			}
			if s.Duration.Value > 0 {
				d := float64(s.Duration.Value)
				legTransit.PlannedDurationSeconds = &d
			}
			if dep != "" && arr != "" {
				step.Instruction = fmt.Sprintf("Take %s from %s to %s", lineName, dep, arr)
			} else {
				step.Instruction = "Take " + lineName
			}
			step.Transit = legTransit
		} else if mode == "transit" && travel == "WALKING" {
			d := float64(s.Duration.Value)
			legTransit := &maps.TransitLeg{Vehicle: "walk"}
			if d > 0 {
				legTransit.PlannedDurationSeconds = &d
			}
			step.Transit = legTransit
		}
		steps = append(steps, step)
	}

	steps = append(steps, maps.RouteStep{
		ID:             uuid.NewString(),
		Instruction:    "You have arrived",
		Maneuver:       "arrive",
		DistanceMeters: 0,
		Polyline:       []maps.Coordinate{{Latitude: destLat, Longitude: destLng}},
	})

	if ov := decodePolyline(route.OverviewPolyline.Points); len(ov) > 0 {
		poly = ov
	}
	if len(poly) == 0 {
		poly = []maps.Coordinate{
			{Latitude: originLat, Longitude: originLng},
			{Latitude: destLat, Longitude: destLng},
		}
	}

	summary := destinationName
	if mode == "transit" {
		if joined := strings.Join(summaryParts, " → "); joined != "" {
			summary = joined
		} else {
			summary = "Transit"
		}
	}

	var cost *float64
	if route.Fare != nil && route.Fare.Value > 0 {
		// Google fare is in local currency; expose as-is for display (app label is historical).
		v := route.Fare.Value
		cost = &v
	}

	return &maps.RoutePlan{
		ID:                         uuid.NewString(),
		Mode:                       mode,
		Origin:                     maps.Coordinate{Latitude: originLat, Longitude: originLng},
		Destination:                maps.Coordinate{Latitude: destLat, Longitude: destLng},
		DestinationName:            destinationName,
		DistanceMeters:             float64(leg.Distance.Value),
		ExpectedTravelTime:         float64(leg.Duration.Value),
		CostYuan:                   cost,
		Summary:                    summary,
		Polyline:                   poly,
		Steps:                      steps,
		CreatedAt:                  time.Now().UTC(),
		CreatedByName:              createdByName,
		RealtimeTransitUnavailable: mode == "transit",
	}, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.base + path + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(io.LimitReader(res.Body, 2<<20))
}

func decodePolyline(encoded string) []maps.Coordinate {
	if encoded == "" {
		return nil
	}
	var coords []maps.Coordinate
	index, lat, lng := 0, 0, 0
	for index < len(encoded) {
		var result, shift int
		for {
			if index >= len(encoded) {
				return coords
			}
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		dlat := result >> 1
		if result&1 != 0 {
			dlat = ^dlat
		}
		lat += dlat

		result, shift = 0, 0
		for {
			if index >= len(encoded) {
				return coords
			}
			b := int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		dlng := result >> 1
		if result&1 != 0 {
			dlng = ^dlng
		}
		lng += dlng
		coords = append(coords, maps.Coordinate{
			Latitude:  float64(lat) / 1e5,
			Longitude: float64(lng) / 1e5,
		})
	}
	return coords
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<div>", " ")
	s = strings.ReplaceAll(s, "</div>", " ")
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func mapManeuver(m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	switch {
	case m == "" || strings.Contains(m, "straight"):
		return "continueStraight"
	case strings.Contains(m, "left"):
		return "turnLeft"
	case strings.Contains(m, "right"):
		return "turnRight"
	case strings.Contains(m, "uturn") || strings.Contains(m, "u-turn"):
		return "uTurn"
	default:
		return "continueStraight"
	}
}

func vehicleKind(typ, name string) string {
	t := strings.ToLower(typ + " " + name)
	switch {
	case strings.Contains(t, "subway") || strings.Contains(t, "metro") || strings.Contains(t, "heavy_rail"):
		return "subway"
	case strings.Contains(t, "rail") || strings.Contains(t, "train"):
		return "railway"
	case strings.Contains(t, "tram") || strings.Contains(t, "bus"):
		return "bus"
	case strings.Contains(t, "ferry"):
		return "other"
	default:
		if typ == "" {
			return "bus"
		}
		return "other"
	}
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371000.0
	toRad := math.Pi / 180
	dLat := (lat2 - lat1) * toRad
	dLng := (lng2 - lng1) * toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstType(types []string) string {
	if len(types) == 0 {
		return ""
	}
	return types[0]
}

func strPtr(s string) *string { return &s }

type placeTextResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Results      []struct {
		PlaceID          string   `json:"place_id"`
		Name             string   `json:"name"`
		FormattedAddress string   `json:"formatted_address"`
		Vicinity         string   `json:"vicinity"`
		Types            []string `json:"types"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

type directionsResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Routes       []struct {
		OverviewPolyline struct {
			Points string `json:"points"`
		} `json:"overview_polyline"`
		Fare *struct {
			Currency string  `json:"currency"`
			Value    float64 `json:"value"`
			Text     string  `json:"text"`
		} `json:"fare"`
		Legs []struct {
			Distance struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"distance"`
			Duration struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"duration"`
			Steps []struct {
				HTMLInstructions string `json:"html_instructions"`
				TravelMode       string `json:"travel_mode"`
				Maneuver         string `json:"maneuver"`
				Distance         struct {
					Value int `json:"value"`
				} `json:"distance"`
				Duration struct {
					Value int `json:"value"`
				} `json:"duration"`
				StartLocation struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"start_location"`
				EndLocation struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"end_location"`
				Polyline struct {
					Points string `json:"points"`
				} `json:"polyline"`
				TransitDetails *struct {
					NumStops      int `json:"num_stops"`
					DepartureStop struct {
						Name string `json:"name"`
					} `json:"departure_stop"`
					ArrivalStop struct {
						Name string `json:"name"`
					} `json:"arrival_stop"`
					Line struct {
						Name      string `json:"name"`
						ShortName string `json:"short_name"`
						Vehicle   struct {
							Name string `json:"name"`
							Type string `json:"type"`
						} `json:"vehicle"`
					} `json:"line"`
				} `json:"transit_details"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
}
