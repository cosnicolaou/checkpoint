package checkpointstate

import (
	"context"
)

// Manager represents a checkpoint manager.
type Manager interface {
	// SessionID creates a unique, stable ID for the session from the supplied
	// inputs, which are expected to be unique to each session.
	SessionID(inputs ...string) string
	// Use will use or create the session for the requested ID. Reset
	// must be set to true when the current step state is not be reset and true
	// when it is.
	Use(ctx context.Context, ID string, reset bool) (Session, error)
}

// Session represents a checkpoint session which is a series of steps that
// may be independently tested for completion.
type Session interface {
	// Step implicitly 'ticks' an existing 'in-progress' step as done and then
	// initiates the next step by first checking to see if it has already
	// been completed, in which case it returns true. If it has not been
	// completed it will mark it as the next 'in-progress' step.
	Step(ctx context.Context, step string) (bool, error)
	// Delete deletes all of the state associated with the session.
	Delete(ctx context.Context) error
}