package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/foqerhk/waymate-server/internal/amap"
	"github.com/foqerhk/waymate-server/internal/auth"
	"github.com/foqerhk/waymate-server/internal/config"
	"github.com/foqerhk/waymate-server/internal/db"
	lk "github.com/foqerhk/waymate-server/internal/livekit"
	"github.com/foqerhk/waymate-server/internal/models"
	"github.com/foqerhk/waymate-server/internal/pair"
	"github.com/foqerhk/waymate-server/internal/push"
	"github.com/foqerhk/waymate-server/internal/ws"
)

type Server struct {
	cfg     config.Config
	store   *db.Store
	tokens  *auth.TokenService
	invites *pair.Signer
	hub     *ws.Hub
	push    push.Pusher
	amap    *amap.Client
	upgrader websocket.Upgrader
}

func New(cfg config.Config, store *db.Store, hub *ws.Hub, pusher push.Pusher) *Server {
	return &Server{
		cfg:     cfg,
		store:   store,
		tokens:  auth.NewTokenService(cfg.JWTSecret),
		invites: pair.NewSigner(cfg.InviteHMACSecret, cfg.PublicBaseURL, cfg.InviteTTL),
		hub:     hub,
		push:    pusher,
		amap:    amap.New(cfg.AmapWebKey),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/devices/register", s.handleRegister)
	mux.HandleFunc("GET /v1/session", s.auth(s.handleSession))
	mux.HandleFunc("POST /v1/families", s.auth(s.handleCreateFamily))
	mux.HandleFunc("POST /v1/invites/refresh", s.auth(s.handleRefreshInvite))
	mux.HandleFunc("POST /v1/families/join", s.auth(s.handleJoin))
	mux.HandleFunc("POST /v1/pairing/elder-offer", s.auth(s.handleElderOffer))
	mux.HandleFunc("POST /v1/pairing/claim-elder", s.auth(s.handleClaimElder))
	mux.HandleFunc("POST /v1/devices/apns", s.auth(s.handleAPNs))
	mux.HandleFunc("POST /v1/elders/{elderId}/location", s.auth(s.handleLocation))
	mux.HandleFunc("POST /v1/elders/{elderId}/routes", s.auth(s.handleSendRoute))
	mux.HandleFunc("PUT /v1/elders/{elderId}/guidance", s.auth(s.handleGuidance))
	mux.HandleFunc("DELETE /v1/elders/{elderId}/trip", s.auth(s.handleStopTrip))
	mux.HandleFunc("POST /v1/elders/{elderId}/stop", s.auth(s.handleStopTrip))
	mux.HandleFunc("POST /v1/elders/{elderId}/favorites", s.auth(s.handleAddFavorite))
	mux.HandleFunc("PUT /v1/elders/{elderId}/favorites/{favoriteId}", s.auth(s.handleUpdateFavorite))
	mux.HandleFunc("DELETE /v1/elders/{elderId}/favorites/{favoriteId}", s.auth(s.handleDeleteFavorite))
	mux.HandleFunc("GET /v1/places/search", s.auth(s.handlePlaceSearch))
	mux.HandleFunc("POST /v1/routes/plan", s.auth(s.handlePlanRoute))
	mux.HandleFunc("DELETE /v1/elders/{elderId}", s.auth(s.handleRemoveElder))
	mux.HandleFunc("POST /v1/me/leave-family", s.auth(s.handleLeaveFamily))
	mux.HandleFunc("PATCH /v1/me/display-name", s.auth(s.handleUpdateDisplayName))
	mux.HandleFunc("POST /v1/me/display-name", s.auth(s.handleUpdateDisplayName))
	mux.HandleFunc("POST /v1/devices/voip", s.auth(s.handleVoIPToken))
	mux.HandleFunc("POST /v1/calls", s.auth(s.handleStartCall))
	mux.HandleFunc("POST /v1/calls/{callId}/answer", s.auth(s.handleAnswerCall))
	mux.HandleFunc("POST /v1/calls/{callId}/end", s.auth(s.handleEndCall))
	mux.HandleFunc("GET /v1/calls/{callId}", s.auth(s.handleGetCall))
	mux.HandleFunc("POST /v1/push-relay/route", s.handlePushRelayRoute)
	mux.HandleFunc("POST /v1/push-relay/voip", s.handlePushRelayVoIP)
	mux.HandleFunc("GET /v1/ws", s.handleWS)
	return s.cors(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "db unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePlaceSearch(w http.ResponseWriter, r *http.Request, _ uuid.UUID, _ models.Role) {
	if s.amap == nil || !s.amap.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "amap_not_configured")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeErr(w, http.StatusBadRequest, "q required")
		return
	}
	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("latitude"), 64)
	lng, err2 := strconv.ParseFloat(r.URL.Query().Get("longitude"), 64)
	if err1 != nil || err2 != nil {
		writeErr(w, http.StatusBadRequest, "latitude and longitude required")
		return
	}
	places, err := s.amap.SearchPlaces(r.Context(), q, lat, lng)
	if err != nil {
		log.Printf("amap place search: %v", err)
		writeErr(w, http.StatusBadGateway, "place_search_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"places": places})
}

type planRouteReq struct {
	Mode                 string  `json:"mode"`
	OriginLatitude       float64 `json:"originLatitude"`
	OriginLongitude      float64 `json:"originLongitude"`
	DestinationLatitude  float64 `json:"destinationLatitude"`
	DestinationLongitude float64 `json:"destinationLongitude"`
	DestinationName      string  `json:"destinationName"`
	CreatedByName        string  `json:"createdByName"`
	City                 string  `json:"city"`
}

func (s *Server) handlePlanRoute(w http.ResponseWriter, r *http.Request, _ uuid.UUID, _ models.Role) {
	if s.amap == nil || !s.amap.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "amap_not_configured")
		return
	}
	var req planRouteReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	mode := strings.TrimSpace(req.Mode)
	name := strings.TrimSpace(req.DestinationName)
	if name == "" {
		name = "目的地"
	}
	by := strings.TrimSpace(req.CreatedByName)
	if by == "" {
		by = "Family"
	}
	var (
		plan *amap.RoutePlan
		err  error
	)
	switch mode {
	case "walking":
		plan, err = s.amap.PlanWalking(
			r.Context(),
			req.OriginLatitude, req.OriginLongitude,
			req.DestinationLatitude, req.DestinationLongitude,
			name, by,
		)
	case "transit":
		plan, err = s.amap.PlanTransit(
			r.Context(),
			req.OriginLatitude, req.OriginLongitude,
			req.DestinationLatitude, req.DestinationLongitude,
			name, by, req.City,
		)
	default:
		writeErr(w, http.StatusBadRequest, "mode must be walking or transit")
		return
	}
	if err != nil {
		log.Printf("amap route plan (%s): %v", mode, err)
		writeErr(w, http.StatusBadGateway, "route_plan_failed")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

type registerReq struct {
	InstallationID string      `json:"installationId"`
	Role           models.Role `json:"role"`
	DisplayName    string      `json:"displayName"`
	Platform       string      `json:"platform"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.InstallationID = strings.TrimSpace(req.InstallationID)
	if req.InstallationID == "" || (req.Role != models.RoleChild && req.Role != models.RoleElder) {
		writeErr(w, http.StatusBadRequest, "installationId and role required")
		return
	}
	if req.Platform == "" {
		req.Platform = "ios"
	}
	if req.DisplayName == "" {
		if req.Role == models.RoleElder {
			req.DisplayName = "Elder"
		} else {
			req.DisplayName = "Family"
		}
	}
	device, _, err := s.store.RegisterDevice(r.Context(), req.InstallationID, req.Role, req.DisplayName, req.Platform)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, exp, err := s.tokens.Issue(device.ID, device.Role)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token issue failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device":      device,
		"accessToken": token,
		"expiresAt":   exp,
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	_ = role
	invite, _ := s.currentInvite(r.Context(), deviceID)
	snap, err := s.store.BuildSession(r.Context(), deviceID, invite)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

type createFamilyReq struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateFamily(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family role can create a family")
		return
	}
	var req createFamilyReq
	_ = decode(r, &req)
	if req.Name == "" {
		req.Name = "My Family"
	}
	device, err := s.store.GetDevice(r.Context(), deviceID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "device not found")
		return
	}
	family, err := s.store.CreateFamily(r.Context(), deviceID, req.Name, device.DisplayName)
	if errors.Is(err, db.ErrConflict) {
		writeErr(w, http.StatusConflict, "already in a family")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	invite, err := s.issueInvite(r.Context(), family.ID, deviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.SetFamily(deviceID, family.ID)
	snap, err := s.store.BuildSession(r.Context(), deviceID, invite)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleRefreshInvite(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family role can refresh invite")
		return
	}
	familyID, err := s.store.FamilyIDForDevice(r.Context(), deviceID)
	if errors.Is(err, db.ErrNotFound) {
		writeErr(w, http.StatusBadRequest, "create a family first")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	invite, err := s.issueInvite(r.Context(), familyID, deviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, invite)
}

type joinReq struct {
	Payload          string `json:"payload"`
	Code             string `json:"code"`
	DisplayName      string `json:"displayName"`
	RelationshipKey  string `json:"relationshipKey"`
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleElder {
		writeErr(w, http.StatusForbidden, "only elder role can join via QR")
		return
	}
	var req joinReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	raw := strings.TrimSpace(req.Payload)
	if raw == "" {
		raw = strings.TrimSpace(req.Code)
	}
	code, famHint, err := s.invites.VerifyPayload(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		name = "Elder"
	}
	familyID, err := s.store.JoinFamily(r.Context(), code, famHint, deviceID, name, req.RelationshipKey)
	if errors.Is(err, db.ErrInviteInvalid) {
		writeErr(w, http.StatusBadRequest, "invalid invite")
		return
	}
	if errors.Is(err, db.ErrConflict) {
		writeErr(w, http.StatusConflict, "already in another family")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.SetFamily(deviceID, familyID)
	s.broadcastSession(r.Context(), familyID, uuid.Nil)
	snap, err := s.store.BuildSession(r.Context(), deviceID, nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleElderOffer(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleElder {
		writeErr(w, http.StatusForbidden, "only elder can create a bind offer")
		return
	}
	qr, deep := s.invites.BuildElderOffer(deviceID)
	writeJSON(w, http.StatusOK, models.Invite{
		Code:      "ELDER",
		FamilyID:  uuid.Nil,
		ExpiresAt: pair.FarFuture(),
		QRPayload: qr,
		DeepLink:  deep,
	})
}

type claimElderReq struct {
	Payload string `json:"payload"`
}

func (s *Server) handleClaimElder(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can claim an elder QR")
		return
	}
	var req claimElderReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	elderID, err := s.invites.VerifyElderOffer(req.Payload)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	elder, err := s.store.GetDevice(r.Context(), elderID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "elder device not found")
		return
	}
	familyID, err := s.store.ClaimElder(r.Context(), deviceID, elderID, elder.DisplayName)
	if errors.Is(err, db.ErrConflict) {
		writeErr(w, http.StatusConflict, "already in another family")
		return
	}
	if errors.Is(err, db.ErrForbidden) {
		writeErr(w, http.StatusForbidden, "not an elder offer")
		return
	}
	if errors.Is(err, db.ErrNotFound) {
		writeErr(w, http.StatusBadRequest, "create a family first")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.SetFamily(elderID, familyID)
	s.broadcastSession(r.Context(), familyID, uuid.Nil)
	invite, _ := s.currentInvite(r.Context(), deviceID)
	snap, err := s.store.BuildSession(r.Context(), deviceID, invite)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

type apnsReq struct {
	Token   string `json:"token"`
	Sandbox *bool  `json:"sandbox"`
}

func (s *Server) handleAPNs(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, _ models.Role) {
	var req apnsReq
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.Token) == "" {
		writeErr(w, http.StatusBadRequest, "token required")
		return
	}
	sandbox := true
	if req.Sandbox != nil {
		sandbox = *req.Sandbox
	}
	if err := s.store.UpdateAPNs(r.Context(), deviceID, req.Token, sandbox); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type locationReq struct {
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	Accuracy     float64  `json:"accuracy"`
	Heading      *float64 `json:"heading"`
	Speed        *float64 `json:"speed"`
	IsBackground bool     `json:"isBackground"`
}

func (s *Server) handleLocation(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	if role != models.RoleElder || deviceID != elderID {
		writeErr(w, http.StatusForbidden, "elders can only update their own location")
		return
	}
	var req locationReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	_ = s.store.TouchDevice(r.Context(), deviceID)
	loc := models.Location{
		ElderDeviceID: elderID,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		Accuracy:      req.Accuracy,
		Heading:       req.Heading,
		Speed:         req.Speed,
		IsBackground:  req.IsBackground,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := s.store.UpsertLocation(r.Context(), loc); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	familyID, err := s.store.FamilyIDForDevice(r.Context(), deviceID)
	if err == nil {
		raw, _ := json.Marshal(loc)
		s.hub.BroadcastFamily(familyID, models.WSEnvelope{Type: "location", Data: raw}, uuid.Nil)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSendRoute(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can send routes")
		return
	}
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
		writeErr(w, http.StatusForbidden, "not in same family")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		writeErr(w, http.StatusBadRequest, "route body required")
		return
	}
	guidanceSeed, _ := json.Marshal(map[string]any{
		"phase": "waitingToStart",
		"route": json.RawMessage(body),
	})
	if err := s.store.UpsertTrip(r.Context(), elderID, deviceID, json.RawMessage(body), guidanceSeed); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	familyID, _ := s.store.FamilyIDForDevice(r.Context(), deviceID)
	raw, _ := json.Marshal(map[string]any{
		"elderDeviceId": elderID,
		"route":         json.RawMessage(body),
	})
	// Always persist first. Realtime WS is best-effort — offline elders still get the
	// trip from DB (+ APNs wake) the next time they open the app.
	online := s.hub.IsOnline(elderID)
	if familyID != uuid.Nil {
		s.hub.BroadcastFamily(familyID, models.WSEnvelope{Type: "route", Data: raw}, uuid.Nil)
	}
	if online {
		s.hub.SendTo(elderID, models.WSEnvelope{Type: "route", Data: raw})
	}

	pushSent := false
	token, sandbox, _ := s.store.ElderAPNsTokens(r.Context(), elderID)
	if strings.TrimSpace(token) != "" {
		if err := s.push.NotifyRoute(r.Context(), token, sandbox, "WayMate", "家人给你安排了一条新路线"); err != nil {
			log.Printf("route push failed elder=%s: %v", elderID, err)
		} else {
			pushSent = true
		}
	}

	delivery := "queued"
	if online {
		delivery = "realtime"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"delivery":    delivery,
		"elderOnline": online,
		"pushSent":    pushSent,
	})
}

func (s *Server) handleGuidance(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	if role == models.RoleElder && deviceID != elderID {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	if role == models.RoleChild {
		if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		writeErr(w, http.StatusBadRequest, "guidance required")
		return
	}
	if err := s.store.UpdateGuidance(r.Context(), elderID, json.RawMessage(body)); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "no active trip")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	familyID, _ := s.store.FamilyIDForDevice(r.Context(), elderID)
	s.hub.BroadcastFamily(familyID, models.WSEnvelope{Type: "guidance", Data: json.RawMessage(body)}, uuid.Nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStopTrip(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	switch role {
	case models.RoleElder:
		if deviceID != elderID {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
	case models.RoleChild:
		if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
	default:
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := s.store.ClearTrip(r.Context(), elderID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	familyID, _ := s.store.FamilyIDForDevice(r.Context(), elderID)
	raw, _ := json.Marshal(map[string]any{
		"elderDeviceId": elderID.String(),
		"stoppedBy":     deviceID.String(),
		"reason":        map[bool]string{true: "family", false: "self"}[role == models.RoleChild],
	})
	if familyID != uuid.Nil {
		s.hub.BroadcastFamily(familyID, models.WSEnvelope{Type: "route_stopped", Data: raw}, uuid.Nil)
		s.broadcastSession(r.Context(), familyID, uuid.Nil)
	} else {
		s.hub.SendTo(elderID, models.WSEnvelope{Type: "route_stopped", Data: raw})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAddFavorite(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can add favorites")
		return
	}
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	familyID, err := s.store.FamilyIDForDevice(r.Context(), deviceID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no family")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil || len(body) == 0 {
		writeErr(w, http.StatusBadRequest, "payload required")
		return
	}
	if err := s.store.AddFavorite(r.Context(), familyID, elderID, json.RawMessage(body)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUpdateFavorite(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can update favorites")
		return
	}
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	favoriteID := strings.TrimSpace(r.PathValue("favoriteId"))
	if favoriteID == "" {
		writeErr(w, http.StatusBadRequest, "bad favoriteId")
		return
	}
	if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil || len(body) == 0 {
		writeErr(w, http.StatusBadRequest, "payload required")
		return
	}
	if err := s.store.UpdateFavorite(r.Context(), elderID, favoriteID, json.RawMessage(body)); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteFavorite(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can delete favorites")
		return
	}
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	favoriteID := strings.TrimSpace(r.PathValue("favoriteId"))
	if favoriteID == "" {
		writeErr(w, http.StatusBadRequest, "bad favoriteId")
		return
	}
	if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := s.store.DeleteFavorite(r.Context(), elderID, favoriteID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		authz := r.Header.Get("Authorization")
		token = strings.TrimPrefix(authz, "Bearer ")
	}
	claims, err := s.tokens.Parse(token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	familyID, _ := s.store.FamilyIDForDevice(r.Context(), claims.DeviceID)
	client := &ws.Client{
		DeviceID: claims.DeviceID,
		FamilyID: familyID,
		Conn:     conn,
		Send:     make(chan []byte, 32),
	}
	s.hub.Register(client)
	_ = s.store.TouchDevice(r.Context(), claims.DeviceID)

	go s.writePump(client)
	s.readPump(client)
}

func (s *Server) writePump(c *ws.Client) {
	defer func() {
		_ = c.Conn.Close()
	}()
	for msg := range c.Send {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (s *Server) readPump(c *ws.Client) {
	defer func() {
		s.hub.Unregister(c.DeviceID)
		_ = c.Conn.Close()
	}()
	c.Conn.SetReadLimit(1 << 20)
	_ = c.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_ = s.store.TouchDevice(context.Background(), c.DeviceID)
		return nil
	})
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		var env models.WSEnvelope
		if json.Unmarshal(message, &env) != nil {
			continue
		}
		if env.Type == "ping" {
			pong, _ := json.Marshal(models.WSEnvelope{Type: "pong"})
			select {
			case c.Send <- pong:
			default:
			}
		}
	}
}

type voipReq struct {
	Token string `json:"token"`
}

func (s *Server) handleVoIPToken(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, _ models.Role) {
	var req voipReq
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.Token) == "" {
		writeErr(w, http.StatusBadRequest, "token required")
		return
	}
	if err := s.store.UpdateVoIPToken(r.Context(), deviceID, req.Token); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type startCallReq struct {
	ElderDeviceID string `json:"elderDeviceId"`
	MediaType     string `json:"mediaType"` // voice | video
}

func (s *Server) handleStartCall(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can start calls")
		return
	}
	var req startCallReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.MediaType != "voice" && req.MediaType != "video" {
		writeErr(w, http.StatusBadRequest, "mediaType must be voice or video")
		return
	}
	elderID, err := uuid.Parse(req.ElderDeviceID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderDeviceId")
		return
	}
	if err := s.store.AssertSameFamily(r.Context(), deviceID, elderID); err != nil {
		writeErr(w, http.StatusForbidden, "not in same family")
		return
	}
	// Elder must be reachable via live WebSocket OR a registered VoIP token
	// (PushKit wake when the app is suspended / not running).
	voip, _ := s.store.VoIPToken(r.Context(), elderID)
	online := s.hub.IsOnline(elderID)
	if !online && strings.TrimSpace(voip) == "" {
		writeErr(w, http.StatusConflict, "elder_offline")
		return
	}
	familyID, err := s.store.FamilyIDForDevice(r.Context(), deviceID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no family")
		return
	}
	channel := "wm_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	call, err := s.store.CreateCall(r.Context(), familyID, deviceID, elderID, req.MediaType, channel)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	lkCfg := lk.Config{
		URL:       s.cfg.LiveKitPublicURL,
		APIKey:    s.cfg.LiveKitAPIKey,
		APISecret: s.cfg.LiveKitAPISecret,
	}
	if !lkCfg.Enabled() {
		writeErr(w, http.StatusServiceUnavailable, "livekit not configured")
		return
	}
	callerToken, err := lkCfg.RoomToken(deviceID.String(), channel, 2*time.Hour)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token mint failed")
		return
	}
	elderToken, err := lkCfg.RoomToken(elderID.String(), channel, 2*time.Hour)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token mint failed")
		return
	}

	callerName := "家人"
	if memberName, err := s.store.MemberDisplayName(r.Context(), deviceID); err == nil && strings.TrimSpace(memberName) != "" {
		callerName = strings.TrimSpace(memberName)
	} else if caller, err := s.store.GetDevice(r.Context(), deviceID); err == nil && caller != nil && strings.TrimSpace(caller.DisplayName) != "" {
		callerName = strings.TrimSpace(caller.DisplayName)
	}

	elderPayload := map[string]any{
		"callId":         call.ID.String(),
		"mediaType":      call.MediaType,
		"channelName":    call.ChannelName,
		"callerDeviceId": call.CallerDeviceID.String(),
		"calleeDeviceId": call.CalleeDeviceID.String(),
		"livekitUrl":     lkCfg.URL,
		"livekitToken":   elderToken,
		"autoAnswer":     true,
		"callerName":     callerName,
	}
	callerPayload := map[string]any{
		"callId":         call.ID.String(),
		"mediaType":      call.MediaType,
		"channelName":    call.ChannelName,
		"callerDeviceId": call.CallerDeviceID.String(),
		"calleeDeviceId": call.CalleeDeviceID.String(),
		"livekitUrl":     lkCfg.URL,
		"livekitToken":   callerToken,
		"autoAnswer":     false,
	}
	elderRaw, _ := json.Marshal(elderPayload)
	callerRaw, _ := json.Marshal(callerPayload)
	if online {
		s.hub.SendTo(elderID, models.WSEnvelope{Type: "incoming_call", Data: elderRaw})
	}
	s.hub.SendTo(deviceID, models.WSEnvelope{Type: "call_started", Data: callerRaw})

	if strings.TrimSpace(voip) != "" {
		sandbox := !s.cfg.APNsProduction
		if !online {
			// Offline wake must succeed before we tell the caller it's ringing.
			if err := s.push.NotifyVoIP(r.Context(), voip, sandbox, elderPayload); err != nil {
				log.Printf("voip push failed elder=%s online=%v: %v", elderID, online, err)
				writeErr(w, http.StatusConflict, "voip_push_failed")
				return
			}
		} else {
			// Elder already on WS — don't block the caller's UI on APNs RTT.
			go func() {
				if err := s.push.NotifyVoIP(context.Background(), voip, sandbox, elderPayload); err != nil {
					log.Printf("voip push (online) failed elder=%s: %v", elderID, err)
				}
			}()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"call":         call,
		"livekitUrl":   lkCfg.URL,
		"livekitToken": callerToken,
	})
}

func (s *Server) handleAnswerCall(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, _ models.Role) {
	callID, err := uuid.Parse(r.PathValue("callId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad callId")
		return
	}
	call, err := s.store.GetCall(r.Context(), callID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "call not found")
		return
	}
	if call.CalleeDeviceID != deviceID && call.CallerDeviceID != deviceID {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	_ = s.store.AnswerCall(r.Context(), callID)
	raw, _ := json.Marshal(map[string]any{"callId": callID.String(), "status": "active"})
	s.hub.SendTo(call.CallerDeviceID, models.WSEnvelope{Type: "call_answered", Data: raw})
	s.hub.SendTo(call.CalleeDeviceID, models.WSEnvelope{Type: "call_answered", Data: raw})
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (s *Server) handleEndCall(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, _ models.Role) {
	callID, err := uuid.Parse(r.PathValue("callId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad callId")
		return
	}
	call, err := s.store.GetCall(r.Context(), callID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "call not found")
		return
	}
	if call.CalleeDeviceID != deviceID && call.CallerDeviceID != deviceID {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	_ = s.store.EndCall(r.Context(), callID, "ended")
	raw, _ := json.Marshal(map[string]any{"callId": callID.String(), "status": "ended"})
	s.hub.SendTo(call.CallerDeviceID, models.WSEnvelope{Type: "call_ended", Data: raw})
	s.hub.SendTo(call.CalleeDeviceID, models.WSEnvelope{Type: "call_ended", Data: raw})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ended"})
}

func (s *Server) handleGetCall(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, _ models.Role) {
	callID, err := uuid.Parse(r.PathValue("callId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad callId")
		return
	}
	call, err := s.store.GetCall(r.Context(), callID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "call not found")
		return
	}
	if call.CalleeDeviceID != deviceID && call.CallerDeviceID != deviceID {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	lkCfg := lk.Config{
		URL:       s.cfg.LiveKitPublicURL,
		APIKey:    s.cfg.LiveKitAPIKey,
		APISecret: s.cfg.LiveKitAPISecret,
	}
	token, _ := lkCfg.RoomToken(deviceID.String(), call.ChannelName, 2*time.Hour)
	writeJSON(w, http.StatusOK, map[string]any{
		"call":         call,
		"livekitUrl":   lkCfg.URL,
		"livekitToken": token,
	})
}

func (s *Server) handleRemoveElder(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleChild {
		writeErr(w, http.StatusForbidden, "only family can remove an elder")
		return
	}
	elderID, err := uuid.Parse(r.PathValue("elderId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad elderId")
		return
	}
	familyID, _ := s.store.FamilyIDForDevice(r.Context(), deviceID)
	if err := s.store.RemoveElderFromFamily(r.Context(), deviceID, elderID); err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrForbidden) {
			writeErr(w, http.StatusForbidden, "cannot remove")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.hub.SendTo(elderID, models.WSEnvelope{Type: "unpaired", Data: json.RawMessage(`{}`)})
	s.broadcastSession(r.Context(), familyID, uuid.Nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLeaveFamily(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, role models.Role) {
	if role != models.RoleElder {
		writeErr(w, http.StatusForbidden, "only elder can leave")
		return
	}
	familyID, _ := s.store.FamilyIDForDevice(r.Context(), deviceID)
	if err := s.store.LeaveFamily(r.Context(), deviceID); err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrForbidden) {
			writeErr(w, http.StatusBadRequest, "not in a family")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if familyID != uuid.Nil {
		s.broadcastSession(r.Context(), familyID, deviceID)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUpdateDisplayName(w http.ResponseWriter, r *http.Request, deviceID uuid.UUID, _ models.Role) {
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	runeCount := 0
	for range name {
		runeCount++
	}
	if name == "" || runeCount > 40 {
		writeErr(w, http.StatusBadRequest, "display name required (1–40 characters)")
		return
	}
	if err := s.store.UpdateDisplayName(r.Context(), deviceID, name); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "device not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	invite, _ := s.currentInvite(r.Context(), deviceID)
	snap, err := s.store.BuildSession(r.Context(), deviceID, invite)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if familyID, err := s.store.FamilyIDForDevice(r.Context(), deviceID); err == nil {
		// Push refreshed session to every other bound device so lists / CallKit names update.
		s.broadcastSession(r.Context(), familyID, deviceID)
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) issueInvite(ctx context.Context, familyID, createdBy uuid.UUID) (*models.Invite, error) {
	code, err := pair.RandomCode(6)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveInvite(ctx, familyID, createdBy, code); err != nil {
		return nil, err
	}
	qr, deep := s.invites.BuildPayload(familyID, code)
	return &models.Invite{
		Code:      code,
		FamilyID:  familyID,
		ExpiresAt: pair.FarFuture(),
		QRPayload: qr,
		DeepLink:  deep,
	}, nil
}

func (s *Server) currentInvite(ctx context.Context, deviceID uuid.UUID) (*models.Invite, error) {
	familyID, err := s.store.FamilyIDForDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	code, err := s.store.LatestInvite(ctx, familyID)
	if errors.Is(err, db.ErrNotFound) {
		return s.issueInvite(ctx, familyID, deviceID)
	}
	if err != nil {
		return nil, err
	}
	qr, deep := s.invites.BuildPayload(familyID, code)
	return &models.Invite{Code: code, FamilyID: familyID, ExpiresAt: pair.FarFuture(), QRPayload: qr, DeepLink: deep}, nil
}

func (s *Server) broadcastSession(ctx context.Context, familyID uuid.UUID, except uuid.UUID) {
	ids, err := s.store.FamilyMemberIDs(ctx, familyID)
	if err != nil {
		return
	}
	for _, id := range ids {
		if id == except {
			continue
		}
		snap, err := s.store.BuildSession(ctx, id, nil)
		if err != nil {
			continue
		}
		raw, _ := json.Marshal(snap)
		s.hub.SendTo(id, models.WSEnvelope{Type: "session", Data: raw})
	}
}

type ctxKey int

const deviceCtxKey ctxKey = 1

type deviceCtx struct {
	ID   uuid.UUID
	Role models.Role
}

func (s *Server) auth(next func(http.ResponseWriter, *http.Request, uuid.UUID, models.Role)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		claims, err := s.tokens.Parse(strings.TrimPrefix(authz, "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		_ = s.store.TouchDevice(r.Context(), claims.DeviceID)
		next(w, r, claims.DeviceID, claims.Role)
	}
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
