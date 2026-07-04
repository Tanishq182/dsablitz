package ratings

import "errors"

var (
	ErrNotFound       = errors.New("rating record not found")
	ErrInvalidOutcome = errors.New("invalid match outcome")
)
