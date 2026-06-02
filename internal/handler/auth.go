package handler

import (
	"context"
	"html/template"
	"net/http"

	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type AuthHandler struct {
	authSvc   *auth.Service
	queries   *store.Queries
	templates *template.Template
}

func NewAuthHandler(authSvc *auth.Service, q *store.Queries, t *template.Template) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, queries: q, templates: t}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "Logowanie",
		"Error": "",
	}
	h.templates.ExecuteTemplate(w, "login", data)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	login := r.FormValue("login")
	password := r.FormValue("password")

	session, token, err := h.authSvc.Authenticate(r.Context(), login, password)
	if err != nil {
		data := map[string]any{
			"Title": "Logowanie",
			"Error": "Nieprawidłowy login lub hasło",
			"Login": login,
		}
		w.WriteHeader(http.StatusUnauthorized)
		h.templates.ExecuteTemplate(w, "login", data)
		return
	}

	h.authSvc.SetCookie(w, token)

	// Render clients page directly — WebKitGTK doesn't persist cookies
	// before JS redirect fires, so we skip the redirect entirely.
	ctx := context.WithValue(r.Context(), auth.UserKey, session)
	clients, err := h.queries.ListClients(ctx, store.ListClientsParams{Limit: 50, Offset: 0})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	count, _ := h.queries.CountClients(ctx)

	data := map[string]any{
		"Clients":         clients,
		"Count":           count,
		"Title":           "Klienci",
		"ContentTemplate": "client_list",
		"User":            session,
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.authSvc.CookieName())
	if err == nil {
		h.authSvc.Logout(cookie.Value)
	}
	h.authSvc.ClearCookie(w)
	h.LoginPage(w, r)
}
