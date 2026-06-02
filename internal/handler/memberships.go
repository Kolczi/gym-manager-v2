package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"gym-manager-v2/internal/store"
)

type MembershipHandler struct {
	queries *store.Queries
}

func NewMembershipHandler(q *store.Queries) *MembershipHandler {
	return &MembershipHandler{queries: q}
}

func (h *MembershipHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	clientID, _ := strconv.Atoi(chi.URLParam(r, "clientID"))
	typeID, _ := strconv.Atoi(r.FormValue("type_id"))
	startsAt, err := time.Parse("2006-01-02", r.FormValue("starts_at"))
	if err != nil {
		http.Error(w, "Nieprawidłowa data rozpoczęcia", 400)
		return
	}

	// Fetch membership type to compute end date
	mt, err := h.queries.GetMembershipType(r.Context(), int64(typeID))
	if err != nil {
		http.Error(w, "Nie znaleziono typu karnetu", 404)
		return
	}

	var endsAt time.Time
	switch mt.DurationUnit {
	case "day":
		endsAt = startsAt.AddDate(0, 0, int(mt.DurationValue))
	case "month":
		endsAt = startsAt.AddDate(0, int(mt.DurationValue), 0)
	case "year":
		endsAt = startsAt.AddDate(int(mt.DurationValue), 0, 0)
	}

	// Check for overlapping active memberships
	hasOverlap, err := h.queries.HasOverlappingMembership(r.Context(), store.HasOverlappingMembershipParams{
		ClientID:    int64(clientID),
		NewStartsAt: startsAt.Format("2006-01-02"),
		NewEndsAt:   endsAt.Format("2006-01-02"),
	})
	if err == nil && hasOverlap {
		http.Error(w, "Klient ma już aktywny karnet w tym okresie. Dezaktywuj obecny lub ustaw datę rozpoczęcia na przyszłość.", 409)
		return
	}

	_, err = h.queries.CreateMembership(r.Context(), store.CreateMembershipParams{
		ClientID: int64(clientID),
		TypeID:   int64(typeID),
		StartsAt: startsAt.Format("2006-01-02"),
		EndsAt:   endsAt.Format("2006-01-02"),
		IsActive: sql.NullInt64{Int64: 1, Valid: true},
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/clients/"+strconv.Itoa(clientID))
	w.WriteHeader(http.StatusCreated)
}

func (h *MembershipHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	clientID := chi.URLParam(r, "clientID")

	err := h.queries.DeactivateMembership(r.Context(), int64(id))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/clients/"+clientID)
	w.WriteHeader(http.StatusOK)
}

func (h *MembershipHandler) Freeze(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	clientID := chi.URLParam(r, "clientID")

	freezeFrom := r.FormValue("freeze_from")
	freezeUntil := r.FormValue("freeze_until")
	if freezeFrom == "" || freezeUntil == "" {
		http.Error(w, "Podaj datę od i do zawieszenia", 400)
		return
	}

	from, err := time.Parse("2006-01-02", freezeFrom)
	if err != nil {
		http.Error(w, "Nieprawidłowa data od", 400)
		return
	}
	until, err := time.Parse("2006-01-02", freezeUntil)
	if err != nil {
		http.Error(w, "Nieprawidłowa data do", 400)
		return
	}
	if !until.After(from) {
		http.Error(w, "Data do musi być późniejsza niż data od", 400)
		return
	}

	// Get membership + type to check freeze limits
	m, err := h.queries.GetMembership(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Nie znaleziono karnetu", 404)
		return
	}

	if m.FrozenAt.Valid {
		http.Error(w, "Karnet jest już zawieszony", 409)
		return
	}

	if m.IsActive.Int64 != 1 {
		http.Error(w, "Nie można zawiesić nieaktywnego karnetu", 400)
		return
	}

	if m.MaxFreezeDays <= 0 {
		http.Error(w, "Ten typ karnetu nie pozwala na zawieszenie", 403)
		return
	}

	freezeDays := int64(until.Sub(from).Hours() / 24)
	if m.TotalFrozenDays+freezeDays > m.MaxFreezeDays {
		http.Error(w, fmt.Sprintf("Przekroczono limit zawieszenia (%d + %d > %d dni)", m.TotalFrozenDays, freezeDays, m.MaxFreezeDays), 403)
		return
	}

	err = h.queries.FreezeMembership(r.Context(), store.FreezeMembershipParams{
		ID:          int64(id),
		FreezeFrom:  sql.NullString{String: freezeFrom, Valid: true},
		FreezeUntil: sql.NullString{String: freezeUntil, Valid: true},
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/clients/"+clientID)
	w.WriteHeader(http.StatusOK)
}

func (h *MembershipHandler) Unfreeze(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	clientID := chi.URLParam(r, "clientID")

	m, err := h.queries.GetMembership(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Nie znaleziono karnetu", 404)
		return
	}

	if !m.FrozenAt.Valid {
		http.Error(w, "Karnet nie jest zawieszony", 400)
		return
	}

	// Calculate frozen days from the stored date string
	frozenAt, _ := time.Parse("2006-01-02", m.FrozenAt.String)
	frozenDays := int64(time.Since(frozenAt).Hours() / 24)
	newTotal := m.TotalFrozenDays + frozenDays
	if newTotal > m.MaxFreezeDays {
		// Cap: unfreeze anyway but warn — the SQL handles the math
	}

	err = h.queries.UnfreezeMembership(r.Context(), int64(id))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("HX-Redirect", "/clients/"+clientID)
	w.WriteHeader(http.StatusOK)
}
