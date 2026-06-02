package handler

import (
	"html/template"
	"net/http"

	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type DashboardHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewDashboardHandler(q *store.Queries, t *template.Template) *DashboardHandler {
	return &DashboardHandler{queries: q, templates: t}
}

func (h *DashboardHandler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	activeClients, _ := h.queries.CountActiveClients(ctx)
	activeMemberships, _ := h.queries.CountActiveMemberships(ctx)
	entriesToday, _ := h.queries.CountEntriesToday(ctx)
	overduePayments, _ := h.queries.CountOverduePayments(ctx)
	expiringCount, _ := h.queries.CountExpiringMemberships(ctx)
	expiring, _ := h.queries.ListExpiringMemberships(ctx)
	recentEntries, _ := h.queries.ListRecentEntries(ctx)

	data := map[string]any{
		"Title":              "Dashboard",
		"ContentTemplate":    "dashboard",
		"ActiveClients":      activeClients,
		"ActiveMemberships":  activeMemberships,
		"EntriesToday":       entriesToday,
		"OverduePayments":    overduePayments,
		"ExpiringCount":      expiringCount,
		"ExpiringMemberships": expiring,
		"RecentEntries":      recentEntries,
	}

	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "dashboard", data)
		return
	}
	if u := auth.UserFromContext(ctx); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}
