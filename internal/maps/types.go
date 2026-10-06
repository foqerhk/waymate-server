package maps

import (
	"time"
)

// Coordinate matches the iOS Coordinate JSON shape.
type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Place struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Address        string  `json:"address"`
	District       string  `json:"district,omitempty"`
	Type           string  `json:"type,omitempty"`
	Latitude       float64 `json:"latitude"`  // WGS-84 for the app
	Longitude      float64 `json:"longitude"` // WGS-84 for the app
	DistanceMeters *int    `json:"distanceMeters,omitempty"`
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
