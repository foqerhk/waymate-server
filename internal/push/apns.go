package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"github.com/waymate/backend/internal/config"
)

// Pusher sends wake-up notifications when a route is pushed to an elder,
// and VoIP pushes so CallKit can ring when the elder app is not running.
type Pusher interface {
	NotifyRoute(ctx context.Context, deviceToken string, sandbox bool, title, body string, custom map[string]any) error
	NotifyVoIP(ctx context.Context, voipToken string, sandbox bool, data map[string]any) error
}

type LogPusher struct{}

func (LogPusher) NotifyRoute(_ context.Context, deviceToken string, sandbox bool, title, body string, custom map[string]any) error {
	if deviceToken == "" {
		return nil
	}
	log.Printf("apns stub sandbox=%v token=%s… title=%q body=%q custom=%v", sandbox, trim(deviceToken, 8), title, body, custom)
	return nil
}

func (LogPusher) NotifyVoIP(_ context.Context, voipToken string, sandbox bool, data map[string]any) error {
	if voipToken == "" {
		return nil
	}
	log.Printf("voip push stub sandbox=%v token=%s… payload=%v", sandbox, trim(voipToken, 8), data)
	return nil
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// APNsPusher talks to Apple Push Notification service with a .p8 auth key.
type APNsPusher struct {
	keyID    string
	teamID   string
	bundleID string
	token    *token.Token
	dev      *apns2.Client
	prod     *apns2.Client
}

func NewFromEnv(cfg config.Config) Pusher {
	// 1) Local APNs — official cloud or a fully custom-signed iOS binary.
	if cfg.APNsEnabled {
		if cfg.APNsKeyPath == "" || cfg.APNsKeyID == "" || cfg.APNsTeamID == "" {
			log.Printf("apns: enabled but missing KEY_PATH/KEY_ID/TEAM_ID — checking push relay")
		} else if authKey, err := token.AuthKeyFromFile(cfg.APNsKeyPath); err != nil {
			log.Printf("apns: cannot load key %s: %v — checking push relay", cfg.APNsKeyPath, err)
		} else {
			tok := &token.Token{
				AuthKey: authKey,
				KeyID:   cfg.APNsKeyID,
				TeamID:  cfg.APNsTeamID,
			}
			p := &APNsPusher{
				keyID:    cfg.APNsKeyID,
				teamID:   cfg.APNsTeamID,
				bundleID: cfg.APNsBundleID,
				token:    tok,
				dev:      apns2.NewTokenClient(tok).Development(),
				prod:     apns2.NewTokenClient(tok).Production(),
			}
			log.Printf("apns: enabled key=%s team=%s bundle=%s production_default=%v",
				cfg.APNsKeyID, cfg.APNsTeamID, cfg.APNsBundleID, cfg.APNsProduction)
			return p
		}
	}

	// 2) Official push relay — self-hosted data plane, App Store push credentials stay central.
	if cfg.PushRelayURL != "" && cfg.PushRelayToken != "" {
		log.Printf("apns: using official push relay at %s", cfg.PushRelayURL)
		return NewRelayPusher(cfg.PushRelayURL, cfg.PushRelayToken)
	}

	log.Printf("apns: disabled — VoIP/route wake uses stub (set APNS_* or PUSH_RELAY_*)")
	return LogPusher{}
}

func (p *APNsPusher) client(sandbox bool) *apns2.Client {
	if sandbox {
		return p.dev
	}
	return p.prod
}

func (p *APNsPusher) NotifyRoute(ctx context.Context, deviceToken string, sandbox bool, title, body string, custom map[string]any) error {
	if strings.TrimSpace(deviceToken) == "" {
		return nil
	}
	// Prefer body-only alert so iOS shows the localized CFBundleDisplayName (带路)
	// instead of a hardcoded English title like "WayMate".
	pl := payload.NewPayload().
		AlertBody(body).
		Sound("default").
		ContentAvailable().
		Custom("waymate", "route")
	if strings.TrimSpace(title) != "" {
		pl = pl.AlertTitle(title)
	}
	for k, v := range custom {
		pl.Custom(k, v)
	}
	n := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       p.bundleID,
		PushType:    apns2.PushTypeAlert,
		Priority:    apns2.PriorityHigh,
		Expiration:  time.Now().Add(24 * time.Hour),
		Payload:     pl,
	}
	res, err := p.client(sandbox).PushWithContext(ctx, n)
	if err != nil {
		return err
	}
	if !res.Sent() {
		return fmt.Errorf("apns route reject status=%d reason=%s", res.StatusCode, res.Reason)
	}
	return nil
}

func (p *APNsPusher) NotifyVoIP(ctx context.Context, voipToken string, sandbox bool, data map[string]any) error {
	voipToken = strings.TrimSpace(voipToken)
	if voipToken == "" {
		return fmt.Errorf("empty voip token")
	}
	pl := payload.NewPayload().ContentAvailable()
	for k, v := range data {
		pl.Custom(k, v)
	}
	// Keep a compact JSON for logs / debugging without dumping livekit tokens.
	meta, _ := json.Marshal(map[string]any{
		"callId":    data["callId"],
		"mediaType": data["mediaType"],
		"sandbox":   sandbox,
	})
	n := &apns2.Notification{
		DeviceToken: voipToken,
		Topic:       p.bundleID + ".voip",
		PushType:    apns2.PushTypeVOIP,
		Priority:    apns2.PriorityHigh,
		Expiration:  time.Now().Add(60 * time.Second),
		Payload:     pl,
	}
	res, err := p.client(sandbox).PushWithContext(ctx, n)
	if err != nil {
		return err
	}
	if !res.Sent() {
		log.Printf("apns voip reject token=%s… status=%d reason=%s meta=%s",
			trim(voipToken, 8), res.StatusCode, res.Reason, meta)
		return fmt.Errorf("apns voip reject status=%d reason=%s", res.StatusCode, res.Reason)
	}
	log.Printf("apns voip sent token=%s… meta=%s", trim(voipToken, 8), meta)
	return nil
}

// Ensure the key file is readable early (helps catch mount/path mistakes at boot).
func WarmKeyPath(path string) {
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		log.Printf("apns: key path check failed: %v", err)
	}
}
