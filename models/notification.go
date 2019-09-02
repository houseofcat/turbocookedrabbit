package models

// Notification is a way to communicate between callers
type Notification struct {
	LetterID uint64
	Success  bool
	Error    error
}
