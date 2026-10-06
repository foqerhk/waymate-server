package maps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/waymate/backend/internal/db"
)

// UsageStore is the subset of db.Store used for per-key metering.
type UsageStore interface {
	IncrMapsUsage(ctx context.Context, provider, keyFP string, kind db.MapsKind, n int) error
	MapsUsageMonthByKey(ctx context.Context, provider, keyFP string, kind db.MapsKind) (int, error)
	SetMapsQuotaExhausted(ctx context.Context, provider, infocode string) error
}

// Metered wraps single-key providers and rotates across keys with monthly quotas.
type Metered struct {
	ProviderName     string
	Keys               []string
	SearchQuotaPerKey  int
	LBSQuotaPerKey     int
	Store              UsageStore
	NewClient          func(key string) Provider
	mu                 sync.Mutex
	rr                 int
}

func KeyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8])
}

func (m *Metered) Enabled() bool {
	return m != nil && len(m.Keys) > 0
}

func (m *Metered) Name() string {
	if m == nil {
		return ""
	}
	return m.ProviderName
}

func (m *Metered) KeyCount() int {
	if m == nil {
		return 0
	}
	return len(m.Keys)
}

func (m *Metered) SearchPlaces(ctx context.Context, keywords string, lat, lng float64) ([]Place, error) {
	return runMetered(m, ctx, db.MapsKindSearch, func(p Provider) ([]Place, error) {
		return p.SearchPlaces(ctx, keywords, lat, lng)
	})
}

func (m *Metered) PlanWalking(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName string) (*RoutePlan, error) {
	return runMetered(m, ctx, db.MapsKindWalk, func(p Provider) (*RoutePlan, error) {
		return p.PlanWalking(ctx, originLat, originLng, destLat, destLng, destinationName, createdByName)
	})
}

func (m *Metered) PlanTransit(ctx context.Context, originLat, originLng, destLat, destLng float64, destinationName, createdByName, city string) (*RoutePlan, error) {
	return runMetered(m, ctx, db.MapsKindTransit, func(p Provider) (*RoutePlan, error) {
		return p.PlanTransit(ctx, originLat, originLng, destLat, destLng, destinationName, createdByName, city)
	})
}

func runMetered[T any](m *Metered, ctx context.Context, kind db.MapsKind, fn func(Provider) (T, error)) (T, error) {
	var zero T
	if !m.Enabled() {
		return zero, ErrNotConfigured
	}
	quota := m.SearchQuotaPerKey
	if kind == db.MapsKindLBS || kind == db.MapsKindWalk || kind == db.MapsKindTransit {
		quota = m.LBSQuotaPerKey
	}
	order := m.keyOrder()
	var lastErr error
	for _, key := range order {
		fp := KeyFingerprint(key)
		used, err := m.Store.MapsUsageMonthByKey(ctx, m.ProviderName, fp, kind)
		if err != nil {
			return zero, err
		}
		if used >= quota {
			continue
		}
		client := m.NewClient(key)
		if client == nil || !client.Enabled() {
			continue
		}
		out, err := fn(client)
		if err != nil {
			lastErr = err
			if code, ok := QuotaInfocode(err); ok {
				_ = m.Store.SetMapsQuotaExhausted(ctx, m.ProviderName, code)
				continue
			}
			return zero, err
		}
		_ = m.Store.IncrMapsUsage(ctx, m.ProviderName, fp, kind, 1)
		return out, nil
	}
	if lastErr != nil {
		return zero, lastErr
	}
	return zero, fmt.Errorf("maps monthly quota exhausted for all keys (%s/%s)", m.ProviderName, kind)
}

func (m *Metered) keyOrder() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.Keys)
	if n == 0 {
		return nil
	}
	start := m.rr % n
	m.rr++
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, m.Keys[(start+i)%n])
	}
	return out
}

// QuotaInfocode extracts Amap-style quota error codes from error text when present.
func QuotaInfocode(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	msg := err.Error()
	for _, code := range []string{"10003", "10044", "10045", "OVER_QUERY_LIMIT"} {
		if strings.Contains(msg, code) {
			return code, true
		}
	}
	if errors.Is(err, ErrNotConfigured) {
		return "", false
	}
	return "", false
}

// ParseKeys merges comma-separated multi-key env with a legacy single key.
func ParseKeys(multiCSV, single string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	for _, p := range strings.Split(multiCSV, ",") {
		add(p)
	}
	add(single)
	return out
}
