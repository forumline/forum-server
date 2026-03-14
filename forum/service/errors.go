package service

// ValidationError indicates the caller provided invalid input (HTTP 400).
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError indicates a resource was not found (HTTP 404).
type NotFoundError struct {
	Msg string
}

func (e *NotFoundError) Error() string { return e.Msg }

// ForbiddenError indicates the user lacks permission (HTTP 403).
type ForbiddenError struct {
	Msg string
}

func (e *ForbiddenError) Error() string { return e.Msg }

// ConflictError indicates a conflict with the current state (HTTP 409).
type ConflictError struct {
	Msg string
}

func (e *ConflictError) Error() string { return e.Msg }
