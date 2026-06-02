package handler

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gym-manager-v2/internal/audit"
	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type ClientHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewClientHandler(q *store.Queries, t *template.Template) *ClientHandler {
	return &ClientHandler{queries: q, templates: t}
}

func (h *ClientHandler) withUser(r *http.Request, data map[string]any) map[string]any {
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	return data
}

func (h *ClientHandler) Index(w http.ResponseWriter, r *http.Request) {
	h.List(w, r)
}

func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) {
	clients, err := h.queries.ListClients(r.Context(), store.ListClientsParams{
		Limit:  50,
		Offset: 0,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	count, _ := h.queries.CountClients(r.Context())

	data := map[string]any{
		"Clients":         clients,
		"Count":           count,
		"Title":           "Klienci",
		"ContentTemplate": "client_list",
	}

	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "client_list", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func (h *ClientHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	var clients any
	var count int64
	var err error
	if q == "" {
		clients, err = h.queries.ListClients(r.Context(), store.ListClientsParams{Limit: 50, Offset: 0})
		if err == nil {
			count, _ = h.queries.CountClients(r.Context())
		}
	} else {
		var res []store.Client
		res, err = h.queries.SearchClients(r.Context(), sql.NullString{String: q, Valid: true})
		clients = res
		count = int64(len(res))
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]any{
		"Clients": clients,
		"Count":   count,
		"Query":   q,
	}
	h.templates.ExecuteTemplate(w, "client_table", data)
}

func (h *ClientHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":  "Nowy klient",
		"Client": store.Client{},
		"IsNew":           true,
		"ContentTemplate": "client_form",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "client_form", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	client, err := h.queries.CreateClient(r.Context(), store.CreateClientParams{
		Name:      r.FormValue("name"),
		Surname:   r.FormValue("surname"),
		Email:     TextOrNull(r.FormValue("email")),
		Phone:     TextOrNull(r.FormValue("phone")),
		Comment:   TextOrNull(r.FormValue("comment")),
		AlertNote: r.FormValue("alert_note"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		audit.Log(r.Context(), h.queries, u.UserID, "client.create", map[string]string{"name": r.FormValue("name") + " " + r.FormValue("surname")})
	}
	w.Header().Set("HX-Redirect", fmt.Sprintf("/clients/%d", client.ID))
	w.WriteHeader(http.StatusCreated)
}

func (h *ClientHandler) Show(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	client, err := h.queries.GetClient(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Nie znaleziono klienta", 404)
		return
	}
	memberships, _ := h.queries.ListMembershipsByClient(r.Context(), int64(id))
	activeTypes, _ := h.queries.ListActiveMembershipTypes(r.Context())
	entries, _ := h.queries.ListEntriesByClient(r.Context(), store.ListEntriesByClientParams{
		ClientID: int64(id),
		Limit:    20,
	})
	entryCount, _ := h.queries.CountEntriesByClient(r.Context(), int64(id))
	payments, _ := h.queries.ListPaymentsByClient(r.Context(), int64(id))
	data := map[string]any{
		"Client":          client,
		"Title":           client.Name + " " + client.Surname,
		"Memberships":     memberships,
		"ActiveTypes":     activeTypes,
		"Entries":         entries,
		"EntryCount":      entryCount,
		"Payments":        payments,
		"ContentTemplate": "client_detail",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "client_detail", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func (h *ClientHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	client, err := h.queries.GetClient(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Nie znaleziono klienta", 404)
		return
	}
	data := map[string]any{
		"Client": client,
		"Title":  "Edytuj: " + client.Name + " " + client.Surname,
		"IsNew":           false,
		"ContentTemplate": "client_form",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "client_form", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func (h *ClientHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	r.ParseForm()
	_, err := h.queries.UpdateClient(r.Context(), store.UpdateClientParams{
		ID:        int64(id),
		Name:      r.FormValue("name"),
		Surname:   r.FormValue("surname"),
		Email:     TextOrNull(r.FormValue("email")),
		Phone:     TextOrNull(r.FormValue("phone")),
		Comment:   TextOrNull(r.FormValue("comment")),
		AlertNote: r.FormValue("alert_note"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		audit.Log(r.Context(), h.queries, u.UserID, "client.update", map[string]any{"id": id})
	}
	w.Header().Set("HX-Redirect", fmt.Sprintf("/clients/%d", id))
	w.WriteHeader(http.StatusOK)
}

func (h *ClientHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	err := h.queries.DeleteClient(r.Context(), int64(id))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		audit.Log(r.Context(), h.queries, u.UserID, "client.delete", map[string]any{"id": id})
	}
	w.Header().Set("HX-Redirect", "/clients")
	w.WriteHeader(http.StatusOK)
}

func (h *ClientHandler) ClearAlert(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	err := h.queries.ClearClientAlert(r.Context(), int64(id))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		audit.Log(r.Context(), h.queries, u.UserID, "client.clear_alert", map[string]any{"id": id})
	}
	h.Show(w, r)
}

func TextOrNull(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
