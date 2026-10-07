package marketing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// HandleTrackingRedirect handles the GET /r/{token} short link.
func (h *Handler) HandleTrackingRedirect(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Redirect(w, r, h.publicBaseURL, http.StatusFound)
		return
	}

	targetURL, err := h.service.ResolveTrackingLink(r.Context(), token, h.publicBaseURL)
	if err != nil {
		// If expired, not found, or invalid, redirect to home page without attribution
		http.Redirect(w, r, h.publicBaseURL, http.StatusFound)
		return
	}

	http.Redirect(w, r, targetURL, http.StatusFound)
}
