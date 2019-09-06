package models

import "fmt"

// Notification is a way to communicate between callers
type Notification struct {
	LetterID uint64
	Success  bool
	Error    error
}

// ToString allows you to quickly log the ErrorMessage struct as a string.
func (not *Notification) ToString() string {
	if not.Success {
		return fmt.Sprintf("[LetterID: %d] - Successful.\r\n", not.LetterID)
	}

	return fmt.Sprintf("[LetterID: %d] - Failed. Reason: %s\r\n", not.LetterID, fmt.Sprint(not.Error))
}
