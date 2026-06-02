package handler

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"gym-manager-v2/internal/audit"
	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type UserHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewUserHandler(q *store.Queries, t *template.Template) *UserHandler {
	return &UserHandler{queries: q, templates: t}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, _ := h.queries.ListUsers(r.Context())
	count, _ := h.queries.CountUsers(r.Context())
	data := map[string]any{
		"Users":           users,
		"Count":           count,
		"Title":           "Użytkownicy",
		"ContentTemplate": "user_list",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "user_list", data)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

func (h *UserHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":           "Nowy użytkownik",
		"ContentTemplate": "user_form",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "user_form", data)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	hash, _ := bcrypt.GenerateFromPassword([]byte(r.FormValue("password")), bcrypt.DefaultCost)
	_, err := h.queries.CreateUser(r.Context(), store.CreateUserParams{
		Login:        r.FormValue("login"),
		Name:         r.FormValue("name"),
		Surname:      r.FormValue("surname"),
		Role:         r.FormValue("role"),
		PasswordHash: string(hash),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	audit.Log(r.Context(), h.queries, auth.UserFromContext(r.Context()).UserID, "user.create", map[string]string{"login": r.FormValue("login")})
	w.Header().Set("HX-Redirect", "/users")
	w.WriteHeader(http.StatusCreated)
}

func (h *UserHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	user, err := h.queries.GetUser(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Nie znaleziono", 404)
		return
	}
	data := map[string]any{
		"EditUser":        user,
		"Title":           "Edycja: " + user.Name,
		"ContentTemplate": "user_form",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "user_form", data)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	_, err := h.queries.UpdateUser(r.Context(), store.UpdateUserParams{
		ID:      int64(id),
		Name:    r.FormValue("name"),
		Surname: r.FormValue("surname"),
		Role:    r.FormValue("role"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Update password if provided
	if pw := r.FormValue("password"); pw != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		h.queries.UpdateUserPassword(r.Context(), store.UpdateUserPasswordParams{
			ID:           int64(id),
			PasswordHash: string(hash),
		})
	}
	w.Header().Set("HX-Redirect", "/users")
	w.WriteHeader(http.StatusOK)
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	h.queries.DeleteUser(r.Context(), int64(id))
	w.Header().Set("HX-Redirect", "/users")
	w.WriteHeader(http.StatusOK)
}

// AdminOnly middleware — only admin role can access
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil || u.Role != "admin" {
			http.Error(w, "Brak uprawnień", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
