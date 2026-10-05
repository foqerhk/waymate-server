package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Coordinate matches the iOS Coordinate JSON shape.
type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type TransitLeg struct {
	Vehicle                string   `json:"vehicle"`
	LineName               *string  `json:"lineName"`
	DepartureStop          *string  `json:"departureStop"`
	ArrivalStop            *string  `json:"arrivalStop"`
	PlannedDurationSeconds *float64 `json:"plannedDurationSeconds"`
	RealtimeArrivalText    *string  `json:"realtimeArrivalText"`
	ViaStopsCount          *int     `json:"viaStopsCount"`
}

type RouteStep struct {
	ID             string       `json:"id"`
	Instruction    string       `json:"instruction"`
	Maneuver       string       `json:"maneuver"`
	DistanceMeters float64      `json:"distanceMeters"`
	Polyline       []Coordinate `json:"polyline"`
	Transit        *TransitLeg  `json:"transit"`
}

// RoutePlan matches the iOS RoutePlan JSON shape so the app can decode it directly.
type RoutePlan struct {
	ID                         string       `json:"id"`
	Mode                       string       `json:"mode"`
	Origin                     Coordinate   `json:"origin"`
	Destination                Coordinate   `json:"destination"`
	DestinationName            string       `json:"destinationName"`
	DistanceMeters             float64      `json:"distanceMeters"`
	ExpectedTravelTime         float64      `json:"expectedTravelTime"`
	CostYuan                   *float64     `json:"costYuan"`
	Summary                    string       `json:"summary"`
	Polyline                   []Coordinate `json:"polyline"`
	Steps                      []RouteStep  `json:"steps"`
	CreatedAt                  time.Time    `json:"createdAt"`
	CreatedByName              string       `json:"createdByName"`
	RideHailStatus             *string      `json:"rideHailStatus"`
	RealtimeTransitUnavailable bool         `json:"realtimeTransitUnavailable"`
}

func (c *Client) PlanWalking(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName string) (*RoutePlan, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("amap key not configured")
	}
	oLat, oLng := wgs84ToGcj02(originLat, originLng)
	dLat, dLng := wgs84ToGcj02(destLat, destLng)

	q := url.Values{}
	q.Set("key", c.key)
	q.Set("origin", fmt.Sprintf("%.6f,%.6f", oLng, oLat))
	q.Set("destination", fmt.Sprintf("%.6f,%.6f", dLng, dLat))
	q.Set("output", "JSON")

	body, err := c.get(ctx, "/v3/direction/walking", q)
	if err != nil {
		return nil, err
	}
	var decoded walkingResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Status != "1" {
		return nil, fmt.Errorf("amap walking: %s (%s)", decoded.Info, decoded.Infocode)
	}
	if len(decoded.Route.Paths) == 0 {
		return nil, fmt.Errorf("no walking route")
	}
	path := decoded.Route.Paths[0]

	var steps []RouteStep
	var poly []Coordinate
	for i, s := range path.Steps {
		coords := parsePolylineWGS(stringifyFlex(s.Polyline))
		poly = append(poly, coords...)
		steps = append(steps, RouteStep{
			ID:             fmt.Sprintf("walk-%d-%s", i, uuid.NewString()[:8]),
			Instruction:    firstNonEmpty(stringifyFlex(s.Instruction), stringifyFlex(s.Road), "继续前进"),
			Maneuver:       "continueStraight",
			DistanceMeters: parseFloatFlex(s.Distance),
			Polyline:       coords,
			Transit:        nil,
		})
	}
	steps = append(steps, RouteStep{
		ID:             uuid.NewString(),
		Instruction:    "已到达目的地",
		Maneuver:       "arrive",
		DistanceMeters: 0,
		Polyline:       []Coordinate{{Latitude: destLat, Longitude: destLng}},
		Transit:        nil,
	})
	if len(poly) == 0 {
		poly = []Coordinate{
			{Latitude: originLat, Longitude: originLng},
			{Latitude: destLat, Longitude: destLng},
		}
	}
	dist := parseFloatFlex(path.Distance)
	dur := parseFloatFlex(path.Duration)
	if dur == 0 && dist > 0 {
		dur = dist / 1.2
	}
	return &RoutePlan{
		ID:                         uuid.NewString(),
		Mode:                       "walking",
		Origin:                     Coordinate{Latitude: originLat, Longitude: originLng},
		Destination:                Coordinate{Latitude: destLat, Longitude: destLng},
		DestinationName:            destinationName,
		DistanceMeters:             dist,
		ExpectedTravelTime:         dur,
		Summary:                    destinationName,
		Polyline:                   poly,
		Steps:                      steps,
		CreatedAt:                  time.Now().UTC(),
		CreatedByName:              createdByName,
		RealtimeTransitUnavailable: false,
	}, nil
}

func (c *Client) PlanTransit(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName, city string) (*RoutePlan, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("amap key not configured")
	}
	city = strings.TrimSpace(city)
	if city == "" {
		city = "重庆"
	}
	oLat, oLng := wgs84ToGcj02(originLat, originLng)
	dLat, dLng := wgs84ToGcj02(destLat, destLng)

	q := url.Values{}
	q.Set("key", c.key)
	q.Set("origin", fmt.Sprintf("%.6f,%.6f", oLng, oLat))
	q.Set("destination", fmt.Sprintf("%.6f,%.6f", dLng, dLat))
	q.Set("city", city)
	q.Set("cityd", city)
	q.Set("strategy", "0")
	q.Set("extensions", "all")
	q.Set("output", "JSON")

	body, err := c.get(ctx, "/v3/direction/transit/integrated", q)
	if err != nil {
		return nil, err
	}
	var decoded transitResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Status != "1" {
		return nil, fmt.Errorf("amap transit: %s (%s)", decoded.Info, decoded.Infocode)
	}
	if decoded.Route == nil || len(decoded.Route.Transits) == 0 {
		return nil, fmt.Errorf("no transit route")
	}
	transit := decoded.Route.Transits[0]

	var steps []RouteStep
	var poly []Coordinate
	var summaryParts []string

	for i, seg := range transit.Segments {
		if seg.Walking != nil {
			for wIndex, walk := range seg.Walking.Steps {
				coords := parsePolylineWGS(stringifyFlex(walk.Polyline))
				poly = append(poly, coords...)
				dur := parseFloatFlex(walk.Duration)
				if dur == 0 {
					dur = parseFloatFlex(seg.Walking.Duration)
				}
				leg := &TransitLeg{Vehicle: "walk"}
				if dur > 0 {
					leg.PlannedDurationSeconds = &dur
				}
				steps = append(steps, RouteStep{
					ID:             fmt.Sprintf("transit-walk-%d-%d", i, wIndex),
					Instruction:    firstNonEmpty(stringifyFlex(walk.Instruction), "步行前往车站"),
					Maneuver:       "continueStraight",
					DistanceMeters: parseFloatFlex(walk.Distance),
					Polyline:       coords,
					Transit:        leg,
				})
			}
		}
		if seg.Bus != nil && len(seg.Bus.Buslines) > 0 {
			line := seg.Bus.Buslines[0]
			coords := parsePolylineWGS(stringifyFlex(line.Polyline))
			poly = append(poly, coords...)
			lineName := firstNonEmpty(stringifyFlex(line.Name), "公交")
			summaryParts = append(summaryParts, lineName)
			dep := stopName(line.DepartureStop)
			arr := stopName(line.ArrivalStop)
			instruction := "请乘坐" + lineName
			if dep != "" && arr != "" {
				instruction = fmt.Sprintf("请乘坐%s，从%s上车，到%s下车", lineName, dep, arr)
			}
			vehicle := vehicleKind(stringifyFlex(line.Type))
			leg := &TransitLeg{Vehicle: vehicle, LineName: strPtr(lineName)}
			if dep != "" {
				leg.DepartureStop = &dep
			}
			if arr != "" {
				leg.ArrivalStop = &arr
			}
			if d := parseFloatFlex(line.Duration); d > 0 {
				leg.PlannedDurationSeconds = &d
			}
			if n, err := strconv.Atoi(stringifyFlex(line.ViaNum)); err == nil {
				leg.ViaStopsCount = &n
			}
			steps = append(steps, RouteStep{
				ID:             fmt.Sprintf("transit-bus-%d", i),
				Instruction:    instruction,
				Maneuver:       "continueStraight",
				DistanceMeters: parseFloatFlex(line.Distance),
				Polyline:       coords,
				Transit:        leg,
			})
		}
	}
	steps = append(steps, RouteStep{
		ID:             uuid.NewString(),
		Instruction:    "已到达目的地",
		Maneuver:       "arrive",
		DistanceMeters: 0,
		Polyline:       []Coordinate{{Latitude: destLat, Longitude: destLng}},
		Transit:        nil,
	})
	if len(poly) == 0 {
		poly = []Coordinate{
			{Latitude: originLat, Longitude: originLng},
			{Latitude: destLat, Longitude: destLng},
		}
	}
	summary := strings.Join(summaryParts, " → ")
	if summary == "" {
		summary = "公交地铁"
	}
	dist := parseFloatFlex(transit.Distance)
	if dist == 0 {
		dist = parseFloatFlex(decoded.Route.Distance)
	}
	dur := parseFloatFlex(transit.Duration)
	var cost *float64
	if c := parseFloatFlex(transit.Cost); c > 0 {
		cost = &c
	}
	return &RoutePlan{
		ID:                         uuid.NewString(),
		Mode:                       "transit",
		Origin:                     Coordinate{Latitude: originLat, Longitude: originLng},
		Destination:                Coordinate{Latitude: destLat, Longitude: destLng},
		DestinationName:            destinationName,
		DistanceMeters:             dist,
		ExpectedTravelTime:         dur,
		CostYuan:                   cost,
		Summary:                    summary,
		Polyline:                   poly,
		Steps:                      steps,
		CreatedAt:                  time.Now().UTC(),
		CreatedByName:              createdByName,
		RealtimeTransitUnavailable: true,
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

func parsePolylineWGS(s string) []Coordinate {
	parts := strings.Split(strings.TrimSpace(s), ";")
	out := make([]Coordinate, 0, len(parts))
	for _, pair := range parts {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		xy := strings.Split(pair, ",")
		if len(xy) != 2 {
			continue
		}
		lng, err1 := strconv.ParseFloat(xy[0], 64)
		lat, err2 := strconv.ParseFloat(xy[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		wgsLat, wgsLng := gcj02ToWgs84(lat, lng)
		out = append(out, Coordinate{Latitude: wgsLat, Longitude: wgsLng})
	}
	return out
}

func parseFloatFlex(raw json.RawMessage) float64 {
	s := stringifyFlex(raw)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func strPtr(s string) *string { return &s }

func stopName(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "[]" || string(raw) == "null" {
		return ""
	}
	var obj struct {
		Name json.RawMessage `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return stringifyFlex(obj.Name)
	}
	return stringifyFlex(raw)
}

func vehicleKind(typ string) string {
	t := strings.ToLower(typ)
	switch {
	case strings.Contains(typ, "地铁") || strings.Contains(t, "metro") || strings.Contains(typ, "轨"):
		return "subway"
	case strings.Contains(typ, "火") || strings.Contains(t, "railway"):
		return "railway"
	case strings.Contains(typ, "公交") || strings.Contains(t, "bus"):
		return "bus"
	default:
		if typ == "" {
			return "bus"
		}
		return "other"
	}
}

type walkingResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	Infocode string `json:"infocode"`
	Route    struct {
		Paths []struct {
			Distance json.RawMessage `json:"distance"`
			Duration json.RawMessage `json:"duration"`
			Steps    []struct {
				Instruction json.RawMessage `json:"instruction"`
				Road        json.RawMessage `json:"road"`
				Distance    json.RawMessage `json:"distance"`
				Duration    json.RawMessage `json:"duration"`
				Polyline    json.RawMessage `json:"polyline"`
			} `json:"steps"`
		} `json:"paths"`
	} `json:"route"`
}

type transitResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	Infocode string `json:"infocode"`
	Route    *struct {
		Distance json.RawMessage `json:"distance"`
		Transits []struct {
			Cost     json.RawMessage `json:"cost"`
			Duration json.RawMessage `json:"duration"`
			Distance json.RawMessage `json:"distance"`
			Segments []struct {
				Walking *struct {
					Distance json.RawMessage `json:"distance"`
					Duration json.RawMessage `json:"duration"`
					Steps    []struct {
						Instruction json.RawMessage `json:"instruction"`
						Distance    json.RawMessage `json:"distance"`
						Duration    json.RawMessage `json:"duration"`
						Polyline    json.RawMessage `json:"polyline"`
					} `json:"steps"`
				} `json:"walking"`
				Bus *struct {
					Buslines []struct {
						Name          json.RawMessage `json:"name"`
						Type          json.RawMessage `json:"type"`
						Distance      json.RawMessage `json:"distance"`
						Duration      json.RawMessage `json:"duration"`
						Polyline      json.RawMessage `json:"polyline"`
						ViaNum        json.RawMessage `json:"via_num"`
						DepartureStop json.RawMessage `json:"departure_stop"`
						ArrivalStop   json.RawMessage `json:"arrival_stop"`
					} `json:"buslines"`
				} `json:"bus"`
			} `json:"segments"`
		} `json:"transits"`
	} `json:"route"`
}
