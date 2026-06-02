package handler

import (
	"html/template"
	"net/http"
	"strconv"

	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type AuditHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewAuditHandler(q *store.Queries, t *template.Template) *AuditHandler {
	return &AuditHandler{queries: q, templates: t}
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := int64(50)
	offset := int64(page-1) * limit
	logs, _ := h.queries.ListAuditLogs(r.Context(), store.ListAuditLogsParams{
		Limit:  limit,
		Offset: offset,
	})
	count, _ := h.queries.CountAuditLogs(r.Context())
	data := map[string]any{
		"Logs":            logs,
		"Count":           count,
		"Page":            page,
		"Title":           "Audit log",
		"ContentTemplate": "audit_list",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "audit_list", data)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}
