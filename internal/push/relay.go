package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// RelayPusher forwards APNs/VoIP delivery to the official WayMate push relay.
// Self-hosted deployments use this so they never need the App Store team's .p8 key.
type RelayPusher struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

func NewRelayPusher(baseURL, authToken string) *RelayPusher {
	return &RelayPusher{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		authToken: strings.TrimSpace(authToken),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

type relayRouteBody struct {
	DeviceToken string         `json:"deviceToken"`
	Sandbox     bool           `json:"sandbox"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Custom      map[string]any `json:"custom,omitempty"`
}

type relayVoIPBody struct {
	VoIPToken string         `json:"voipToken"`
	Sandbox   bool           `json:"sandbox"`
	Data      map[string]any `json:"data"`
}

func (p *RelayPusher) NotifyRoute(ctx context.Context, deviceToken string, sandbox bool, title, body string, custom map[string]any) error {
	if strings.TrimSpace(deviceToken) == "" {
		return nil
	}
	return p.post(ctx, "/v1/push-relay/route", relayRouteBody{
		DeviceToken: deviceToken,
		Sandbox:     sandbox,
		Title:       title,
		Body:        body,
		Custom:      custom,
	})
}

func (p *RelayPusher) NotifyVoIP(ctx context.Context, voipToken string, sandbox bool, data map[string]any) error {
	if strings.TrimSpace(voipToken) == "" {
		return fmt.Errorf("empty voip token")
	}
	return p.post(ctx, "/v1/push-relay/voip", relayVoIPBody{
		VoIPToken: voipToken,
		Sandbox:   sandbox,
		Data:      data,
	})
}

func (p *RelayPusher) post(ctx context.Context, path string, payload any) error {
	if p.baseURL == "" || p.authToken == "" {
		return fmt.Errorf("push relay not configured")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.authToken)
	req.Header.Set("User-Agent", "waymate-server-push-relay/1")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push relay request: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 300 {
		return fmt.Errorf("push relay status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	log.Printf("push relay ok path=%s status=%d", path, res.StatusCode)
	return nil
}
