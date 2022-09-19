package tcr

import "errors"

var (
	// ErrConnectionClosed is returned when the construction of a new ChannelHost fails.
	// you can check for this error with errors.Is
	ErrConnectionClosed = errors.New("connection is already closed")

	// ErrPoolClosed is returned when a connection pool shutdown has been triggered
	ErrConnectionPoolClosed = errors.New("connection pool closed")

	// ErrServiceShutdown is returned by service methods that cannot be called, asthe service has been shut down.
	ErrServiceShutdown = errors.New("service shutdown triggered")
)
