package handler

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gym-manager-v2/internal/audit"
	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/store"
)

type PaymentHandler struct {
	queries   *store.Queries
	templates *template.Template
}

func NewPaymentHandler(q *store.Queries, t *template.Template) *PaymentHandler {
	return &PaymentHandler{queries: q, templates: t}
}

// OverdueList shows all overdue (unpaid, past due_date) payments
func (h *PaymentHandler) OverdueList(w http.ResponseWriter, r *http.Request) {
	payments, _ := h.queries.ListOverduePayments(r.Context())
	count, _ := h.queries.CountOverduePayments(r.Context())
	data := map[string]any{
		"Payments":        payments,
		"Count":           count,
		"Title":           "Zaległe płatności",
		"ContentTemplate": "payments_overdue",
	}
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "payments_overdue", data)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		data["User"] = u
	}
	h.templates.ExecuteTemplate(w, "layout", data)
}

// MarkPaid marks a payment as paid
func (h *PaymentHandler) MarkPaid(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	err := h.queries.MarkPaymentPaid(r.Context(), int64(id))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if u := auth.UserFromContext(r.Context()); u != nil {
		audit.Log(r.Context(), h.queries, u.UserID, "payment.paid", map[string]any{"payment_id": id})
	}

	// Redirect back to referrer or overdue list
	ref := r.Header.Get("HX-Current-URL")
	if ref == "" {
		ref = "/payments/overdue"
	}
	w.Header().Set("HX-Redirect", ref)
	w.WriteHeader(http.StatusOK)
}

// CreateForMembership creates a payment record for a membership
func (h *PaymentHandler) CreateForMembership(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	clientID := chi.URLParam(r, "clientID")
	membershipID, _ := strconv.Atoi(chi.URLParam(r, "membershipID"))

	amountStr := r.FormValue("amount")
	dueDate := r.FormValue("due_date")

	amountVal, _ := strconv.ParseFloat(amountStr, 64)

	_, err := h.queries.CreatePayment(r.Context(), store.CreatePaymentParams{
		MembershipID: int64(membershipID),
		DueDate:      dueDate,
		Amount:       amountVal,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/clients/"+clientID)
	w.WriteHeader(http.StatusCreated)
}
