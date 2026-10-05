package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Official push relay: self-hosted WayMate servers forward APNs/VoIP here so the
// App Store team's .p8 never leaves the central deployment.

type pushRelayRouteReq struct {
	DeviceToken string `json:"deviceToken"`
	Sandbox     bool   `json:"sandbox"`
	Title       string `json:"title"`
	Body        string `json:"body"`
}

type pushRelayVoIPReq struct {
	VoIPToken string         `json:"voipToken"`
	Sandbox   bool           `json:"sandbox"`
	Data      map[string]any `json:"data"`
}

func (s *Server) handlePushRelayRoute(w http.ResponseWriter, r *http.Request) {
	if !s.authorizePushRelay(w, r) {
		return
	}
	var req pushRelayRouteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	req.DeviceToken = strings.TrimSpace(req.DeviceToken)
	if req.DeviceToken == "" {
		writeErr(w, http.StatusBadRequest, "missing_device_token")
		return
	}
	if !relayRateAllow("route:"+req.DeviceToken) {
		writeErr(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)
	if title == "" {
		title = "WayMate"
	}
	if body == "" {
		body = "New update"
	}
	if err := s.push.NotifyRoute(r.Context(), req.DeviceToken, req.Sandbox, title, body); err != nil {
		log.Printf("push-relay route failed: %v", err)
		writeErr(w, http.StatusBadGateway, "apns_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePushRelayVoIP(w http.ResponseWriter, r *http.Request) {
	if !s.authorizePushRelay(w, r) {
		return
	}
	var req pushRelayVoIPReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	req.VoIPToken = strings.TrimSpace(req.VoIPToken)
	if req.VoIPToken == "" {
		writeErr(w, http.StatusBadRequest, "missing_voip_token")
		return
	}
	if !relayRateAllow("voip:"+req.VoIPToken) {
		writeErr(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}
	if err := s.push.NotifyVoIP(r.Context(), req.VoIPToken, req.Sandbox, req.Data); err != nil {
		log.Printf("push-relay voip failed: %v", err)
		writeErr(w, http.StatusBadGateway, "apns_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) authorizePushRelay(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(s.cfg.PushRelayAuthToken)
	if expected == "" {
		writeErr(w, http.StatusServiceUnavailable, "push_relay_disabled")
		return false
	}
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) || strings.TrimSpace(strings.TrimPrefix(authz, prefix)) != expected {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

// Simple per-token rate limit for the public community relay token.
var (
	relayMu     sync.Mutex
	relayHits   = map[string][]time.Time{}
	relayWindow = time.Minute
	relayMax    = 20
)

func relayRateAllow(key string) bool {
	now := time.Now()
	relayMu.Lock()
	defer relayMu.Unlock()
	cut := now.Add(-relayWindow)
	kept := relayHits[key][:0]
	for _, t := range relayHits[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= relayMax {
		relayHits[key] = kept
		return false
	}
	relayHits[key] = append(kept, now)
	return true
}
