package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrUserNotFound    = errors.New("user not found")
	ErrTaskNotFound    = errors.New("task not found")
	ErrBidNotFound     = errors.New("bid not found")
	ErrPaymentNotFound = errors.New("payment not found")
	ErrUserBlocked = errors.New("user is blocked")
	ErrTaskInvalidStatus    = errors.New("invalid task status transition")
	ErrTaskAlreadyCompleted = errors.New("task is already completed")
	ErrTaskCancelled        = errors.New("task is cancelled")
	ErrTaskNotPublished     = errors.New("task is not published")
	ErrTaskNotInProgress    = errors.New("task is not in progress")
	ErrBidAlreadyExists   = errors.New("bid already exists")
	ErrBidAlreadyAccepted = errors.New("task already has an accepted bid")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrTaskAlreadyPaid     = errors.New("task already confirmed")
)
