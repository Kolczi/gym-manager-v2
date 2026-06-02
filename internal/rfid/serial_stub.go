//go:build !serial

package rfid

// StartSerialListener is a no-op when built without the "serial" tag.
func (s *Scanner) StartSerialListener(port string, baud int) error {
	return nil
}

// PrepareAssign switches to assign mode (no-op without serial).
func (s *Scanner) PrepareAssign(clientID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanMode = 2
	s.assignClientID = clientID
}

// CancelAssign switches back to normal mode (no-op without serial).
func (s *Scanner) CancelAssign() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanMode = 1
	s.assignClientID = 0
}

// AssignState returns current assign mode state.
func (s *Scanner) AssignState() (active bool, clientID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanMode == 2, s.assignClientID
}
