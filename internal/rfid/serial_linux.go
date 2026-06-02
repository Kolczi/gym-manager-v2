//go:build serial

package rfid

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gym-manager-v2/internal/store"

	"go.bug.st/serial"
)

// StartSerialListener opens the serial port and reads RFID tags line by line.
func (s *Scanner) StartSerialListener(port string, baud int) error {
	mode := &serial.Mode{BaudRate: baud}
	p, err := serial.Open(port, mode)
	if err != nil {
		return fmt.Errorf("serial open %s: %w", port, err)
	}

	s.serialPort = p
	log.Printf("RFID serial: listening on %s @ %d baud", port, baud)

	go func() {
		scanner := bufio.NewScanner(p)
		for scanner.Scan() {
			tag := strings.TrimSpace(scanner.Text())
			if tag == "" {
				continue
			}
			log.Printf("RFID serial: raw read: %q", tag)

			s.mu.Lock()
			mode := s.scanMode
			clientID := s.assignClientID
			s.mu.Unlock()

			if mode == 2 && clientID > 0 {
				s.handleAssign(tag, clientID)
			} else {
				_ = s.ProcessTag(context.Background(), tag)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("RFID serial error: %v", err)
		}
	}()

	return nil
}

// handleAssign assigns the scanned tag to the pending client
func (s *Scanner) handleAssign(tag string, clientID int64) {
	ctx := context.Background()

	existing, err := s.queries.GetClientByRFID(ctx, textVal(tag))
	if err == nil && existing.ID != clientID {
		log.Printf("RFID assign: tag %s already used by client %d", tag, existing.ID)
		s.writeSerial("0")
		s.mu.Lock()
		s.scanMode = 1
		s.assignClientID = 0
		s.stopYellowFlash()
		s.mu.Unlock()
		if s.onScan != nil {
			s.onScan(ScanEvent{
				Tag:     tag,
				Status:  "tag_in_use",
				Message: fmt.Sprintf("Tag %s jest już przypisany do %s %s", tag, existing.Name, existing.Surname),
				Time:    time.Now(),
			})
		}
		s.broadcast(ScanEvent{
			Tag:     tag,
			Status:  "tag_in_use",
			Message: fmt.Sprintf("Tag %s jest już przypisany do %s %s", tag, existing.Name, existing.Surname),
			Time:    time.Now(),
		})
		return
	}

	err = s.queries.UpdateClientRFID(ctx, store.UpdateClientRFIDParams{
		ID:      clientID,
		RfidTag: textVal(tag),
	})
	if err != nil {
		log.Printf("RFID assign error: %v", err)
		s.writeSerial("0")
	} else {
		log.Printf("RFID assign: tag %s → client %d", tag, clientID)
		s.writeSerial("1")
	}

	s.mu.Lock()
	s.scanMode = 1
	s.assignClientID = 0
	s.stopYellowFlash()
	s.mu.Unlock()

	status := "assigned"
	msg := fmt.Sprintf("Tag %s przypisany do klienta #%d", tag, clientID)
	if err != nil {
		status = "error"
		msg = "Błąd przypisania: " + err.Error()
	}
	ev := ScanEvent{
		Tag:      tag,
		ClientID: clientID,
		Status:   status,
		Message:  msg,
		Time:     time.Now(),
	}
	if s.onScan != nil {
		s.onScan(ev)
	}
	s.broadcast(ev)
}

func (s *Scanner) writeSerial(cmd string) {
	if s.serialPort != nil {
		s.serialPort.Write([]byte(cmd))
	}
}

func (s *Scanner) stopYellowFlash() {
	if s.flashCancel != nil {
		s.flashCancel()
		s.flashCancel = nil
	}
}

func (s *Scanner) startYellowFlash() {
	s.stopYellowFlash()
	ctx, cancel := context.WithCancel(context.Background())
	s.flashCancel = cancel

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.writeSerial("2")
			}
		}
	}()
}

func (s *Scanner) PrepareAssign(clientID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanMode = 2
	s.assignClientID = clientID
	s.startYellowFlash()
	log.Printf("RFID: assign mode ON for client %d", clientID)
}

func (s *Scanner) CancelAssign() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanMode = 1
	s.assignClientID = 0
	s.stopYellowFlash()
	log.Printf("RFID: assign mode OFF")
}

func (s *Scanner) AssignState() (active bool, clientID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanMode == 2, s.assignClientID
}
