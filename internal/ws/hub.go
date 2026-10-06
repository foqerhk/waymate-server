package ws

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/waymate/backend/internal/models"
)

type Client struct {
	DeviceID uuid.UUID
	FamilyID uuid.UUID
	Conn     *websocket.Conn
	Send     chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]*Client
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]*Client)}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.clients[c.DeviceID]; ok {
		close(old.Send)
		_ = old.Conn.Close()
	}
	h.clients[c.DeviceID] = c
}

func (h *Hub) Unregister(deviceID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[deviceID]; ok {
		delete(h.clients, deviceID)
		close(c.Send)
	}
}

func (h *Hub) SetFamily(deviceID, familyID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[deviceID]; ok {
		c.FamilyID = familyID
	}
}

func (h *Hub) BroadcastFamily(familyID uuid.UUID, envelope models.WSEnvelope, except uuid.UUID) {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, c := range h.clients {
		if c.FamilyID != familyID || id == except {
			continue
		}
		select {
		case c.Send <- raw:
		default:
		}
	}
}

func (h *Hub) SendTo(deviceID uuid.UUID, envelope models.WSEnvelope) bool {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[deviceID]
	if !ok {
		return false
	}
	select {
	case c.Send <- raw:
		return true
	default:
		return false
	}
}

func (h *Hub) IsOnline(deviceID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[deviceID]
	return ok
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
