package policy

// SetUnrestricted turns every approval and refusal off or on. While it is
// on, Evaluate allows every action, Blocked hides nothing, and password
// and one-time-code fields accept input on a phone and on a virtual phone.
// The rules file is left as it is.
func (e *Engine) SetUnrestricted(on bool) {
	e.mu.Lock()
	e.unrestricted = on
	e.mu.Unlock()
}

// Unrestricted reports whether SetUnrestricted turned the gates off.
func (e *Engine) Unrestricted() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.unrestricted
}
