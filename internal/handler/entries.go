package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gym-manager-v2/internal/audit"
	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type EntryHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewEntryHandler(q *store.Queries, t *template.Template) *EntryHandler {
	return &EntryHandler{queries: q, templates: t}
}

// TodayLog shows all entries for today
func (h *EntryHandler) TodayLog(w http.ResponseWriter, r *http.Request) {
	entries, _ := h.queries.ListEntriesToday(r.Context())
	count, _ := h.queries.CountEntriesToday(r.Context())
	data := map[string]any{
		"Entries":         entries,
		"Count":           count,
		"Title":           "Wejścia dziś",
		"ContentTemplate": "entries_today",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "entries_today", data)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

// Create registers a manual entry for a client
func (h *EntryHandler) Create(w http.ResponseWriter, r *http.Request) {
	clientID, _ := strconv.Atoi(chi.URLParam(r, "clientID"))
	user := auth.UserFromContext(r.Context())

	var recordedBy sql.NullInt64
	if user != nil {
		recordedBy = sql.NullInt64{Int64: user.UserID, Valid: true}
	}

	_, err := h.queries.CreateEntry(r.Context(), store.CreateEntryParams{
		ClientID:   int64(clientID),
		RecordedBy: recordedBy,
		Method:     sql.NullString{String: "manual", Valid: true},
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/clients/"+strconv.Itoa(clientID))
	if user != nil {
		audit.Log(r.Context(), h.queries, user.UserID, "entry.create", map[string]any{"client_id": clientID, "method": "manual"})
	}
	w.WriteHeader(http.StatusCreated)
}
