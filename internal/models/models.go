package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleChild Role = "child"
	RoleElder Role = "elder"
)

type Device struct {
	ID             uuid.UUID `json:"id"`
	InstallationID string    `json:"installationId"`
	Role           Role      `json:"role"`
	DisplayName    string    `json:"displayName"`
	Platform       string    `json:"platform"`
	APNsToken      *string   `json:"apnsToken,omitempty"`
	APNsSandbox    bool      `json:"apnsSandbox"`
	CreatedAt      time.Time `json:"createdAt"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
}

type Family struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	CreatedByDeviceID uuid.UUID `json:"createdByDeviceId"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Member struct {
	FamilyID         uuid.UUID `json:"familyId"`
	DeviceID         uuid.UUID `json:"deviceId"`
	Role             Role      `json:"role"`
	DisplayName      string    `json:"displayName"`
	RelationshipKey  *string   `json:"relationshipKey,omitempty"`
	AvatarSymbol     string    `json:"avatarSymbol"`
	FontScale        float64   `json:"fontScale"`
	JoinedAt         time.Time `json:"joinedAt"`
}

type Invite struct {
	Code      string    `json:"code"`
	FamilyID  uuid.UUID `json:"familyId"`
	ExpiresAt time.Time `json:"expiresAt"`
	QRPayload string    `json:"qrPayload"`
	DeepLink  string    `json:"deepLink"`
}

type Location struct {
	ElderDeviceID uuid.UUID `json:"elderDeviceId"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Accuracy      float64   `json:"accuracy"`
	Heading       *float64  `json:"heading,omitempty"`
	Speed         *float64  `json:"speed,omitempty"`
	IsBackground  bool      `json:"isBackground"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ActiveTrip struct {
	ElderDeviceID     uuid.UUID       `json:"elderDeviceId"`
	CreatedByDeviceID uuid.UUID       `json:"createdByDeviceId"`
	RouteJSON         json.RawMessage `json:"route"`
	GuidanceJSON      json.RawMessage `json:"guidance,omitempty"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

type Favorite struct {
	ID            uuid.UUID       `json:"id"`
	FamilyID      uuid.UUID       `json:"familyId"`
	ElderDeviceID uuid.UUID       `json:"elderDeviceId"`
	PayloadJSON   json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"createdAt"`
}

// SessionSnapshot is the full family state for one device.
type SessionSnapshot struct {
	CurrentDevice Device           `json:"currentDevice"`
	Family        *Family          `json:"family,omitempty"`
	Members       []Member         `json:"members"`
	Elders        []ElderSnapshot  `json:"elders"`
	Favorites     []json.RawMessage `json:"favorites"`
	Invite        *Invite          `json:"invite,omitempty"`
}

type ElderSnapshot struct {
	Member   Member          `json:"member"`
	Presence string          `json:"presence"` // online|offline
	Location *Location       `json:"location,omitempty"`
	Trip     *ActiveTrip     `json:"trip,omitempty"`
}

type WSEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}
