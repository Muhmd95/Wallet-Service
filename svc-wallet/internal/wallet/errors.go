package wallet

import (
	"errors" // Added for custom domain errors
)

// --- Domain Errors ---
// The service layer will check for these exact errors without knowing about
// MongoDB to completely separate service from db
var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrDuplicatePhone      = errors.New("phone number is already registered")
	ErrInvalidPhoneNumber  = errors.New("invalid phone number format")
	ErrInsufficientBalance = errors.New("insufficient balance for the requested operation")
)
