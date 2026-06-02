package handler

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type MembershipTypeHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewMembershipTypeHandler(q *store.Queries, t *template.Template) *MembershipTypeHandler {
	return &MembershipTypeHandler{queries: q, templates: t}
}

func (h *MembershipTypeHandler) withUser(r *http.Request, data map[string]any) map[string]any {
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	return data
}

func (h *MembershipTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	types, err := h.queries.ListMembershipTypes(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	count, _ := h.queries.CountMembershipTypes(r.Context())

	data := map[string]any{
		"Types": types,
		"Count": count,
		"Title":           "Karnety",
		"ContentTemplate": "mt_list",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "mt_list", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func (h *MembershipTypeHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "Nowy karnet",
		"MT":    store.MembershipType{IsActive: sql.NullInt64{Int64: 1, Valid: true}},
		"IsNew":           true,
		"ContentTemplate": "mt_form",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "mt_form", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func boolToNullInt64(val bool) sql.NullInt64 {
	if val {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func (h *MembershipTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	durVal, _ := strconv.Atoi(r.FormValue("duration_value"))
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	maxFreeze, _ := strconv.Atoi(r.FormValue("max_freeze_days"))

	_, err := h.queries.CreateMembershipType(r.Context(), store.CreateMembershipTypeParams{
		Name:          r.FormValue("name"),
		Description:   TextOrNull(r.FormValue("description")),
		DurationValue: int64(durVal),
		DurationUnit:  r.FormValue("duration_unit"),
		IsContract:    boolToNullInt64(r.FormValue("is_contract") == "on"),
		Price:         price,
		IsActive:      boolToNullInt64(r.FormValue("is_active") == "on"),
		MaxFreezeDays: int64(maxFreeze),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("HX-Redirect", "/membership-types")
	w.WriteHeader(http.StatusCreated)
}

func (h *MembershipTypeHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mt, err := h.queries.GetMembershipType(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Nie znaleziono karnetu", 404)
		return
	}
	data := map[string]any{
		"Title": "Edytuj: " + mt.Name,
		"MT":    mt,
		"IsNew":           false,
		"ContentTemplate": "mt_form",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "mt_form", data)
		return
	}
	h.templates.ExecuteTemplate(w, "layout", h.withUser(r, data))
}

func (h *MembershipTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	r.ParseForm()
	durVal, _ := strconv.Atoi(r.FormValue("duration_value"))
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	maxFreeze, _ := strconv.Atoi(r.FormValue("max_freeze_days"))

	_, err := h.queries.UpdateMembershipType(r.Context(), store.UpdateMembershipTypeParams{
		ID:            int64(id),
		Name:          r.FormValue("name"),
		Description:   TextOrNull(r.FormValue("description")),
		DurationValue: int64(durVal),
		DurationUnit:  r.FormValue("duration_unit"),
		IsContract:    boolToNullInt64(r.FormValue("is_contract") == "on"),
		Price:         price,
		IsActive:      boolToNullInt64(r.FormValue("is_active") == "on"),
		MaxFreezeDays: int64(maxFreeze),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("HX-Redirect", "/membership-types")
	w.WriteHeader(http.StatusOK)
}

func (h *MembershipTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	err := h.queries.DeleteMembershipType(r.Context(), int64(id))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("HX-Redirect", "/membership-types")
	w.WriteHeader(http.StatusOK)
}
