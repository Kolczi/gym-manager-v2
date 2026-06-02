package server

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/handler"
	"gym-manager-v2/internal/rfid"
	"gym-manager-v2/internal/store"
)

type Server struct {
	Router    *chi.Mux
	queries   *store.Queries
	templates *template.Template
	authSvc   *auth.Service
}

func New(queries *store.Queries, authSvc *auth.Service, templateFS fs.FS, staticFS fs.FS) *Server {
	s := &Server{
		queries: queries,
		authSvc: authSvc,
	}
	s.parseTemplates(templateFS)
	s.setupRouter(staticFS)
	return s
}

func (s *Server) parseTemplates(templateFS fs.FS) {
	var tmplSet *template.Template

	funcMap := template.FuncMap{
		"textVal": func(t interface{ String() string }) string {
			return t.String()
		},
		"formatPrice": func(n float64) string {
			return fmt.Sprintf("%.2f", n)
		},
		"formatDate": func(d string) string {
			if d == "" {
				return "—"
			}
			t, err := time.Parse("2006-01-02", d)
			if err != nil {
				return d
			}
			return t.Format("02.01.2006")
		},
		"today": func() string {
			return time.Now().Format("2006-01-02")
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int64) int64 { return a - b },
		"isExpired": func(d string) bool {
			if d == "" {
				return false
			}
			t, err := time.Parse("2006-01-02", d)
			if err != nil {
				return false
			}
			return t.Before(time.Now().Truncate(24 * time.Hour))
		},
		"callTemplate": func(name string, data any) (template.HTML, error) {
			var buf bytes.Buffer
			if name == "" {
				return "", nil
			}
			err := tmplSet.ExecuteTemplate(&buf, name, data)
			if err != nil {
				fmt.Printf("callTemplate error: %s: %v\n", name, err)
			}
			return template.HTML(buf.String()), err
		},
	}

	tmplSet = template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS,
			"internal/templates/layouts/*.html",
			"internal/templates/partials/*.html",
		),
	)
	s.templates = tmplSet
}

func (s *Server) setupRouter(staticFS fs.FS) {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.Compress(5))

	staticSub, _ := fs.Sub(staticFS, "frontend/dist")
	fileServer := http.FileServer(http.FS(staticSub))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	ah := handler.NewAuthHandler(s.authSvc, s.queries, s.templates)
	r.Get("/login", ah.LoginPage)
	r.Post("/login", ah.Login)

	// Unprotected root — redirect to login (Wails WebView doesn't follow 302 from middleware)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if user has a valid session
		cookie, err := r.Cookie(s.authSvc.CookieName())
		if err == nil && cookie.Value != "" {
			// Let the auth middleware handle it — forward to dashboard
			s.authSvc.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				dh := handler.NewDashboardHandler(s.queries, s.templates)
				dh.Index(w, r)
			})).ServeHTTP(w, r)
			return
		}
		// No session — serve login page directly (not redirect)
		ah.LoginPage(w, r)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.authSvc.Middleware)

		ch := handler.NewClientHandler(s.queries, s.templates)
		r.Post("/logout", ah.Logout)
		r.Get("/clients", ch.List)
		r.Get("/clients/search", ch.Search)
		r.Get("/clients/new", ch.NewForm)
		r.Post("/clients", ch.Create)
		r.Get("/clients/{id}", ch.Show)
		r.Get("/clients/{id}/edit", ch.EditForm)
		r.Put("/clients/{id}", ch.Update)
		r.Delete("/clients/{id}", ch.Delete)
		r.Post("/clients/{id}/clear-alert", ch.ClearAlert)

		mth := handler.NewMembershipTypeHandler(s.queries, s.templates)
		r.Get("/membership-types", mth.List)
		r.Get("/membership-types/new", mth.NewForm)
		r.Post("/membership-types", mth.Create)
		r.Get("/membership-types/{id}/edit", mth.EditForm)
		r.Put("/membership-types/{id}", mth.Update)
		r.Delete("/membership-types/{id}", mth.Delete)

		mh := handler.NewMembershipHandler(s.queries)
		r.Post("/clients/{clientID}/memberships", mh.Create)
		r.Post("/clients/{clientID}/memberships/{id}/deactivate", mh.Deactivate)
		r.Post("/clients/{clientID}/memberships/{id}/freeze", mh.Freeze)
		r.Post("/clients/{clientID}/memberships/{id}/unfreeze", mh.Unfreeze)

		eh := handler.NewEntryHandler(s.queries, s.templates)
		r.Get("/entries", eh.TodayLog)
		r.Post("/clients/{clientID}/entries", eh.Create)

		ph := handler.NewPaymentHandler(s.queries, s.templates)
		r.Get("/payments/overdue", ph.OverdueList)
		r.Post("/payments/{id}/pay", ph.MarkPaid)
		r.Post("/clients/{clientID}/memberships/{membershipID}/payments", ph.CreateForMembership)
	})

	rfidScanner := rfid.NewScanner(s.queries, nil)
	rh := handler.NewRFIDHandler(rfidScanner)
	r.Route("/api/rfid", func(r chi.Router) {
		r.Post("/scan", rh.Scan)
		r.Get("/state", rh.State)
		r.Get("/events", rh.Events)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.authSvc.Middleware)
		r.Post("/api/rfid/assign/{clientID}", rh.PrepareAssign)
		r.Delete("/api/rfid/assign", rh.CancelAssign)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.authSvc.Middleware)
		r.Use(handler.AdminOnly)
		uh := handler.NewUserHandler(s.queries, s.templates)
		r.Get("/users", uh.List)
		r.Get("/users/new", uh.NewForm)
		r.Post("/users", uh.Create)
		r.Get("/users/{id}/edit", uh.EditForm)
		r.Put("/users/{id}", uh.Update)
		r.Delete("/users/{id}", uh.Delete)

		auh := handler.NewAuditHandler(s.queries, s.templates)
		r.Get("/audit", auh.List)

		bh := handler.NewBackupHandler(s.templates)
		r.Get("/backup", bh.List)
		r.Post("/backup", bh.Create)
	})

	s.Router = r
}
