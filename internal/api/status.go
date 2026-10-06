package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/waymate/backend/internal/db"
	"github.com/waymate/backend/internal/maps"
)

type statusCache struct {
	mu      sync.Mutex
	at      time.Time
	rangeN  int
	payload any
}

func (s *Server) handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	rangeDays := 7
	if v := strings.TrimSpace(r.URL.Query().Get("range")); v != "" {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(v), "d")); err == nil && n > 0 && n <= 90 {
			rangeDays = n
		}
	}

	s.statusCache.mu.Lock()
	if s.statusCache.payload != nil &&
		s.statusCache.rangeN == rangeDays &&
		time.Since(s.statusCache.at) < 20*time.Second {
		payload := s.statusCache.payload
		s.statusCache.mu.Unlock()
		writeJSON(w, http.StatusOK, payload)
		return
	}
	s.statusCache.mu.Unlock()

	ctx := r.Context()
	users, err := s.store.UserCounts(ctx)
	if err != nil {
		log.Printf("status users: %v", err)
		writeErr(w, http.StatusInternalServerError, "status_unavailable")
		return
	}
	active, err := s.store.CountActiveCalls(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "status_unavailable")
		return
	}
	provider := maps.NormalizeProvider(s.cfg.MapsProvider)
	usage, _ := s.store.MapsUsageAggregate(ctx, provider)
	exhaustedAt, infocode, _ := s.store.MapsQuotaState(ctx, provider)
	peak24, _ := s.store.PeakConcurrent24h(ctx)
	callSeries, _ := s.store.CallSeries(ctx, rangeDays)
	searchSeries, _ := s.store.MapsUsageSeriesKind(ctx, provider, rangeDays, db.MapsKindSearch)
	walkSeries, _ := s.store.MapsUsageSeriesKind(ctx, provider, rangeDays, db.MapsKindWalk)
	transitSeries, _ := s.store.MapsUsageSeriesKind(ctx, provider, rangeDays, db.MapsKindTransit)
	lbsSeries, _ := s.store.MapsUsageSeriesLBS(ctx, provider, rangeDays)
	activeSeries, _ := s.store.ActiveDeviceSeries(ctx, rangeDays)

	keyCount := 0
	searchQuotaPerKey := 0
	lbsQuotaPerKey := 0
	if m, ok := s.maps.(*maps.Metered); ok {
		keyCount = m.KeyCount()
		searchQuotaPerKey = m.SearchQuotaPerKey
		lbsQuotaPerKey = m.LBSQuotaPerKey
	} else if s.maps != nil && s.maps.Enabled() {
		keyCount = 1
		if provider == "google" {
			searchQuotaPerKey = s.cfg.GoogleSearchMonthlyQuota
			lbsQuotaPerKey = s.cfg.GoogleLBSMonthlyQuota
		} else {
			searchQuotaPerKey = s.cfg.AmapSearchMonthlyQuota
			lbsQuotaPerKey = s.cfg.AmapLBSMonthlyQuota
		}
	}

	searchQuotaTotal := keyCount * searchQuotaPerKey
	lbsQuotaTotal := keyCount * lbsQuotaPerKey

	remainDays := func(used, quota, last3 int) any {
		left := quota - used
		if left < 0 {
			left = 0
		}
		if last3 <= 0 {
			if left <= 0 {
				return 0
			}
			return nil // unknown / idle
		}
		avg := float64(last3) / 3.0
		if avg <= 0 {
			return nil
		}
		days := float64(left) / avg
		// round to 1 decimal for UI
		return float64(int(days*10+0.5)) / 10
	}

	videoW := s.cfg.CapacityVideoWeight
	if videoW < 1 {
		videoW = 3
	}
	loadUnits := active.Voice + active.Video*videoW
	maxVoice := s.cfg.CapacityMaxVoiceCalls
	maxVideo := s.cfg.CapacityMaxVideoCalls
	maxUnits := maxVoice
	if maxVideo*videoW > maxUnits {
		maxUnits = maxVideo * videoW
	}

	mapsPerFamily := s.cfg.MapsCallsPerFamilyDay
	if mapsPerFamily < 1 {
		mapsPerFamily = 20
	}
	peakPct := s.cfg.PeakCallFractionPct
	if peakPct < 1 {
		peakPct = 2
	}

	// Personal Amap: keyword search (5k/key/mo) is usually the tighter product limit.
	mapsBudget := searchQuotaTotal
	familiesByMaps := 0
	if mapsBudget > 0 {
		familiesByMaps = (mapsBudget / 30) / mapsPerFamily
		if familiesByMaps < 1 {
			familiesByMaps = 1
		}
	}
	familiesByCalls := 0
	if peakPct > 0 && maxVoice > 0 {
		familiesByCalls = (maxVoice * 100) / peakPct
	}
	recommended := familiesByMaps
	bottleneck := "maps"
	if familiesByMaps == 0 || (familiesByCalls > 0 && familiesByCalls < familiesByMaps) {
		recommended = familiesByCalls
		bottleneck = "calls"
	}
	if familiesByMaps == 0 && familiesByCalls == 0 {
		bottleneck = "unknown"
	}

	region := "cn"
	if provider == "google" {
		region = "net"
	}

	payload := map[string]any{
		"ok":        true,
		"region":    region,
		"generated": time.Now().UTC(),
		"health": map[string]any{
			"api":     true,
			"db":      true,
			"livekit": s.cfg.LiveKitPublicURL != "",
		},
		"users": users,
		"calls": map[string]any{
			"activeVoice":       active.Voice,
			"activeVideo":       active.Video,
			"peakConcurrent24h": peak24,
			"wsOnline":          s.hub.OnlineCount(),
		},
		"capacity": map[string]any{
			"maxVoiceCalls":       maxVoice,
			"maxVideoCalls":       maxVideo,
			"videoWeight":         videoW,
			"loadUnits":           loadUnits,
			"maxLoadUnits":        maxUnits,
			"recommendedFamilies": recommended,
			"bottleneck":          bottleneck,
			"familiesByMaps":      familiesByMaps,
			"familiesByCalls":     familiesByCalls,
		},
		"maps": map[string]any{
			"provider":          provider,
			"keyCount":          keyCount,
			"quotaSource":       "configured+self_metered",
			"searchUsedMonth":   usage.SearchMonth,
			"searchUsedToday":   usage.SearchToday,
			"searchQuotaMonth":  searchQuotaTotal,
			"searchQuotaPerKey": searchQuotaPerKey,
			"walkUsedMonth":     usage.WalkMonth,
			"walkUsedToday":     usage.WalkToday,
			"transitUsedMonth":  usage.TransitMonth,
			"transitUsedToday":  usage.TransitToday,
			"lbsUsedMonth":      usage.LBSMonth,
			"lbsUsedToday":      usage.LBSToday,
			"lbsQuotaMonth":     lbsQuotaTotal,
			"lbsQuotaPerKey":    lbsQuotaPerKey,
			"searchLast3d":      usage.SearchLast3d,
			"walkLast3d":        usage.WalkLast3d,
			"transitLast3d":     usage.TransitLast3d,
			"lbsLast3d":         usage.LBSLast3d,
			"searchDaysLeft":    remainDays(usage.SearchMonth, searchQuotaTotal, usage.SearchLast3d),
			"lbsDaysLeft":       remainDays(usage.LBSMonth, lbsQuotaTotal, usage.LBSLast3d),
			"quotaExhaustedAt":  exhaustedAt,
			"lastQuotaInfocode": infocode,
		},
		"series": map[string]any{
			"rangeDays":      rangeDays,
			"calls":          callSeries,
			"mapsCalls":      lbsSeries, // backward-compat: LBS pool total
			"mapsSearch":     searchSeries,
			"mapsWalk":       walkSeries,
			"mapsTransit":    transitSeries,
			"mapsLBS":        lbsSeries,
			"activeDevices":  activeSeries,
		},
		"docs": map[string]string{
			"capacity": "https://github.com/foqerhk/waymate-server/blob/main/docs/CAPACITY.md",
		},
	}

	s.statusCache.mu.Lock()
	s.statusCache.at = time.Now()
	s.statusCache.rangeN = rangeDays
	s.statusCache.payload = payload
	s.statusCache.mu.Unlock()

	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) startCapacitySampler() {
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for range t.C {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			active, err := s.store.CountActiveCalls(ctx)
			if err == nil {
				_ = s.store.InsertCapacitySample(ctx, active.Voice, active.Video, s.hub.OnlineCount())
				_ = s.store.TrimCapacitySamples(ctx, 90)
			} else {
				log.Printf("capacity sample: %v", err)
			}
			cancel()
		}
	}()
}
