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

	"github.com/waymate/backend/internal/maps"
)

type Client struct {
	key  string
	http *http.Client
	base string
}

func New(key string) *Client {
	return &Client{
		key:  strings.TrimSpace(key),
		http: &http.Client{Timeout: 8 * time.Second},
		base: "https://restapi.amap.com",
	}
}

func (c *Client) Enabled() bool { return c != nil && c.key != "" }
func (c *Client) Name() string  { return "amap" }

// SearchPlaces looks up POIs near (lat,lng). Coordinates from the client are treated as
// WGS-84 (CoreLocation) and converted to GCJ-02 for Amap; results are converted back.
func (c *Client) SearchPlaces(ctx context.Context, keywords string, lat, lng float64) ([]maps.Place, error) {
	if !c.Enabled() {
		return nil, maps.ErrNotConfigured
	}
	keywords = strings.TrimSpace(keywords)
	if keywords == "" {
		return nil, fmt.Errorf("keywords required")
	}
	gcjLat, gcjLng := wgs84ToGcj02(lat, lng)

	q := url.Values{}
	q.Set("key", c.key)
	q.Set("keywords", keywords)
	q.Set("location", fmt.Sprintf("%.6f,%.6f", gcjLng, gcjLat))
	q.Set("sortrule", "distance")
	q.Set("offset", "20")
	q.Set("page", "1")
	q.Set("extensions", "base")
	q.Set("output", "JSON")

	u := c.base + "/v3/place/text?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var decoded placeTextResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Status != "1" {
		return nil, fmt.Errorf("amap place search: %s (%s)", decoded.Info, decoded.Infocode)
	}

	out := make([]maps.Place, 0, len(decoded.Pois))
	for _, p := range decoded.Pois {
		plat, plng, ok := parseLocation(p.Location)
		if !ok {
			continue
		}
		wgsLat, wgsLng := gcj02ToWgs84(plat, plng)
		place := maps.Place{
			ID:        firstNonEmpty(p.ID, fmt.Sprintf("%.5f,%.5f", plat, plng)),
			Name:      p.Name,
			Address:   stringifyFlex(p.Address),
			District:  strings.TrimSpace(p.Pname + p.Cityname + p.Adname),
			Type:      p.Type,
			Latitude:  wgsLat,
			Longitude: wgsLng,
		}
		if d := stringifyFlex(p.Distance); d != "" {
			if meters, err := strconv.Atoi(d); err == nil {
				place.DistanceMeters = &meters
			}
		}
		if place.Address == "" {
			place.Address = place.District
		}
		out = append(out, place)
	}
	return out, nil
}

type placeTextResponse struct {
	Status   string     `json:"status"`
	Info     string     `json:"info"`
	Infocode string     `json:"infocode"`
	Pois     []placePOI `json:"pois"`
}

type placePOI struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Address  json.RawMessage `json:"address"`
	Location string          `json:"location"`
	Pname    string          `json:"pname"`
	Cityname string          `json:"cityname"`
	Adname   string          `json:"adname"`
	// Amap may return "" or [] when distance is absent.
	Distance json.RawMessage `json:"distance"`
}

func parseLocation(s string) (lat, lng float64, ok bool) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	lng, err1 := strconv.ParseFloat(parts[0], 64)
	lat, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return lat, lng, true
}

func stringifyFlex(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "[]" || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}
	return strings.Trim(string(raw), `"`)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
