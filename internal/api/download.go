package api

import (
	"net/http"
	"strings"
)

func storeLink(u string) any {
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "market://") {
		return u
	}
	return nil
}

func (s *Server) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{
		"stores": map[string]any{
			"appstore":   storeLink(s.cfg.DownloadAppStoreURL),
			"googleplay": storeLink(s.cfg.DownloadGooglePlayURL),
			"yyb":        storeLink(s.cfg.DownloadYYBURL),
			"huawei":     storeLink(s.cfg.DownloadHuaweiURL),
		},
	})
}
