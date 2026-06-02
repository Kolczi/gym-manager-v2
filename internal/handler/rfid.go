package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gym-manager-v2/internal/rfid"
)

type RFIDHandler struct {
	scanner *rfid.Scanner
}

func NewRFIDHandler(scanner *rfid.Scanner) *RFIDHandler {
	return &RFIDHandler{scanner: scanner}
}

// Scan processes an RFID tag scan via HTTP API.
// POST /api/rfid/scan  body: {"tag": "ABC123"}
func (h *RFIDHandler) Scan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tag string `json:"tag"`
	}

	if r.Header.Get("Content-Type") == "application/json" {
		json.NewDecoder(r.Body).Decode(&req)
	} else {
		r.ParseForm()
		req.Tag = r.FormValue("tag")
	}

	// If in assign mode, handle via AssignTagHTTP
	active, clientID := h.scanner.AssignState()
	if active && clientID > 0 {
		event := h.scanner.AssignTagHTTP(r.Context(), req.Tag, clientID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(event)
		return
	}

	event := h.scanner.ProcessTag(r.Context(), req.Tag)

	w.Header().Set("Content-Type", "application/json")
	if event.Status == "unknown_tag" {
		w.WriteHeader(http.StatusNotFound)
	} else if event.Status == "error" {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(event)
}

// PrepareAssign sets the scanner to assign mode for a given client.
// POST /api/rfid/assign/{clientID}
// Returns HTML fragment for HTMX swap.
func (h *RFIDHandler) PrepareAssign(w http.ResponseWriter, r *http.Request) {
	clientID, err := strconv.Atoi(chi.URLParam(r, "clientID"))
	if err != nil {
		http.Error(w, "invalid client ID", 400)
		return
	}
	h.scanner.PrepareAssign(int64(clientID))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<span class="rfid-waiting">
    <span class="rfid-pulse">📡</span> Przyłóż kartę do czytnika...
</span>
<button hx-delete="/api/rfid/assign" hx-target="#rfid-area" hx-swap="innerHTML"
    style="margin-left:0.5rem; font-size:0.8rem; padding:0.2rem 0.6rem; background:#ef4444; color:#fff; border:none; border-radius:4px; cursor:pointer;">
    ✕ Anuluj
</button>
<style>
.rfid-waiting { color:#fbbf24; font-weight:bold; }
.rfid-pulse { display:inline-block; animation: rfid-blink 1s ease-in-out infinite; }
@keyframes rfid-blink {
    0%%,100%% { opacity:1; transform:scale(1); }
    50%% { opacity:0.3; transform:scale(1.3); }
}
</style>
`)
}

// CancelAssign cancels assign mode.
// DELETE /api/rfid/assign
// Returns HTML fragment — restores the assign button.
func (h *RFIDHandler) CancelAssign(w http.ResponseWriter, r *http.Request) {
	_, clientID := h.scanner.AssignState()
	h.scanner.CancelAssign()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<span style="color:#94a3b8;">anulowano</span>
<button hx-post="/api/rfid/assign/%d" hx-target="#rfid-area" hx-swap="innerHTML"
    class="btn" style="margin-left:0.5rem; font-size:0.8rem; padding:0.2rem 0.6rem; background:#22c55e; color:#fff; border:none; border-radius:4px; cursor:pointer;">
    🏷 Przypisz tag
</button>
`, clientID)
}

// State returns current scanner state (JSON, for API consumers).
// GET /api/rfid/state
func (h *RFIDHandler) State(w http.ResponseWriter, r *http.Request) {
	active, clientID := h.scanner.AssignState()
	mode := "normal"
	if active {
		mode = "assign"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"mode":      mode,
		"client_id": clientID,
	})
}

// Events is an SSE endpoint that streams RFID scan events to the browser.
// GET /api/rfid/events
func (h *RFIDHandler) Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.scanner.Subscribe()
	defer h.scanner.Unsubscribe(ch)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: scan\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
