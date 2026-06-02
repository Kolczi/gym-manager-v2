package rfid

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"gym-manager-v2/internal/store"
)

// Scanner handles RFID tag processing — both from serial port and HTTP API.
type Scanner struct {
	queries        *store.Queries
	onScan         func(event ScanEvent) // callback for UI/SSE notifications
	mu             sync.Mutex
	scanMode       int   // 1=normal, 2=assign
	assignClientID int64 // client to assign next tag to
	serialPort     io.ReadWriteCloser
	flashCancel    context.CancelFunc

	// SSE subscribers
	ssemu   sync.Mutex
	sseSubs map[chan ScanEvent]struct{}
}

// ScanEvent represents a processed RFID scan result.
type ScanEvent struct {
	Tag        string    `json:"tag"`
	ClientID   int64     `json:"client_id,omitempty"`
	ClientName string    `json:"client_name,omitempty"`
	EntryID    int64     `json:"entry_id,omitempty"`
	Status     string    `json:"status"` // "ok", "unknown_tag", "no_active_membership", "error"
	Message    string    `json:"message"`
	AlertNote  string    `json:"alert_note,omitempty"`
	Time       time.Time `json:"time"`
}

func NewScanner(q *store.Queries, onScan func(ScanEvent)) *Scanner {
	return &Scanner{queries: q, onScan: onScan, scanMode: 1, sseSubs: make(map[chan ScanEvent]struct{})}
}

// Subscribe returns a channel that receives scan events. Call Unsubscribe to clean up.
func (s *Scanner) Subscribe() chan ScanEvent {
	ch := make(chan ScanEvent, 8)
	s.ssemu.Lock()
	s.sseSubs[ch] = struct{}{}
	s.ssemu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (s *Scanner) Unsubscribe(ch chan ScanEvent) {
	s.ssemu.Lock()
	delete(s.sseSubs, ch)
	s.ssemu.Unlock()
	close(ch)
}

// broadcast sends event to all SSE subscribers.
func (s *Scanner) broadcast(ev ScanEvent) {
	s.ssemu.Lock()
	defer s.ssemu.Unlock()
	for ch := range s.sseSubs {
		select {
		case ch <- ev:
		default: // drop if subscriber is slow
		}
	}
}

func textVal(str string) sql.NullString {
	return sql.NullString{String: str, Valid: str != ""}
}

// ProcessTag looks up the RFID tag, verifies membership, and creates an entry.
func (s *Scanner) ProcessTag(ctx context.Context, tag string) ScanEvent {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ScanEvent{Tag: tag, Status: "error", Message: "Pusty tag", Time: time.Now()}
	}

	// Lookup client
	client, err := s.queries.GetClientByRFID(ctx, sql.NullString{String: tag, Valid: true})
	if err != nil {
		ev := ScanEvent{Tag: tag, Status: "unknown_tag", Message: "Nieznany tag RFID", Time: time.Now()}
		log.Printf("RFID scan: unknown tag %s", tag)
		if s.onScan != nil {
			s.onScan(ev)
		}
		s.broadcast(ev)
		return ev
	}

	// Check active membership
	active, _ := s.queries.CountActiveMemberships(ctx)
	// Actually check for this specific client
	memberships, _ := s.queries.ListMembershipsByClient(ctx, client.ID)
	hasActive := false
	isFrozen := false
	for _, m := range memberships {
		if m.IsActive.Int64 == 1 && m.IsActive.Valid {
			endsAt, err := time.Parse("2006-01-02", m.EndsAt)
			if err == nil && endsAt.After(time.Now().Add(-24*time.Hour)) {
				if m.FrozenAt.Valid {
					isFrozen = true
				} else {
					hasActive = true
				}
			}
		}
	}

	status := "ok"
	msg := fmt.Sprintf("Wejście: %s %s", client.Name, client.Surname)
	if !hasActive {
		if isFrozen {
			status = "membership_frozen"
			msg = fmt.Sprintf("%s %s — KARNET ZAWIESZONY!", client.Name, client.Surname)
		} else {
			status = "no_active_membership"
			msg = fmt.Sprintf("%s %s — BRAK AKTYWNEGO KARNETU!", client.Name, client.Surname)
		}
	}
	_ = active // suppress unused

	// Only create entry for clients with active membership + debounce 10min
	var entryID int64
	if hasActive {
		lastEntry, err := s.queries.GetLastEntryByClient(ctx, client.ID)
		needNew := err != nil
		if !needNew && lastEntry.CreatedAt.Valid {
			createdAt, parseErr := time.Parse("2006-01-02 15:04:05", lastEntry.CreatedAt.String)
			if parseErr != nil {
				createdAt, parseErr = time.Parse("2006-01-02T15:04:05Z", lastEntry.CreatedAt.String)
			}
			needNew = parseErr != nil || time.Since(createdAt) > 10*time.Minute
		} else if !needNew {
			needNew = true
		}
		if needNew {
			entry, err := s.queries.CreateEntry(ctx, store.CreateEntryParams{
				ClientID:   client.ID,
				RecordedBy: sql.NullInt64{},
				Method:     sql.NullString{String: "rfid", Valid: true},
			})
			if err != nil {
				ev := ScanEvent{Tag: tag, Status: "error", Message: "Błąd zapisu: " + err.Error(), Time: time.Now()}
				if s.onScan != nil {
					s.onScan(ev)
				}
				return ev
			}
			entryID = entry.ID
		} else {
			entryID = lastEntry.ID // reuse — duplicate scan
		}
	}

	ev := ScanEvent{
		Tag:        tag,
		ClientID:   client.ID,
		ClientName: client.Name + " " + client.Surname,
		EntryID:    entryID,
		Status:     status,
		Message:    msg,
		AlertNote:  client.AlertNote,
		Time:       time.Now(),
	}

	log.Printf("RFID scan: %s → %s (%s)", tag, ev.ClientName, ev.Status)
	if s.onScan != nil {
		s.onScan(ev)
	}
	s.broadcast(ev)
	return ev
}

// AssignTagHTTP assigns a tag to a client via HTTP API (non-serial flow).
func (s *Scanner) AssignTagHTTP(ctx context.Context, tag string, clientID int64) ScanEvent {
	tag = strings.TrimSpace(tag)

	// Check if tag already in use by another client
	existing, err := s.queries.GetClientByRFID(ctx, sql.NullString{String: tag, Valid: true})
	if err == nil && existing.ID != clientID {
		s.CancelAssign()
		ev := ScanEvent{
			Tag:     tag,
			Status:  "tag_in_use",
			Message: fmt.Sprintf("Tag %s jest już przypisany do %s %s", tag, existing.Name, existing.Surname),
			Time:    time.Now(),
		}
		s.broadcast(ev)
		return ev
	}

	// Assign
	err = s.queries.UpdateClientRFID(ctx, store.UpdateClientRFIDParams{
		ID:      clientID,
		RfidTag: sql.NullString{String: tag, Valid: true},
	})
	s.CancelAssign()

	if err != nil {
		return ScanEvent{Tag: tag, ClientID: clientID, Status: "error", Message: "Błąd: " + err.Error(), Time: time.Now()}
	}

	log.Printf("RFID assign (HTTP): tag %s → client %d", tag, clientID)
	ev := ScanEvent{
		Tag:      tag,
		ClientID: clientID,
		Status:   "assigned",
		Message:  fmt.Sprintf("Tag %s przypisany", tag),
		Time:     time.Now(),
	}
	s.broadcast(ev)
	return ev
}
