package api

import (
	"net/http"
	"strings"

	"yoldosh/internal/pricing"
)

// Uzum anonim Bearer token ~12 soatda eskiradi. Admin uni qayta deploy qilmasdan yangilay oladi (xotirada; konteyner
// qayta ishga tushsa UZUM_BEARER env dan qayta yuklanadi). Token: brauzerda uzum.uz > DevTools > graphql.uzum.uz > Authorization.

type uzumTokenReq struct {
	Bearer string `json:"bearer"`
}

func (s *Server) adminSetUzumToken(w http.ResponseWriter, r *http.Request) {
	var req uzumTokenReq
	if !decodeJSON(w, r, &req) {
		return
	}
	tok := strings.TrimSpace(req.Bearer)
	if n := len(strings.TrimPrefix(tok, "Bearer ")); n < 40 || n > 4000 {
		writeValidation(w, r, fieldErrors{"bearer": "Uzum Bearer token (brauzer DevTools dan)"})
		return
	}
	pricing.SetUzum(tok, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"ready": pricing.UzumReady()})
}

func (s *Server) adminUzumStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ready": pricing.UzumReady()})
}
