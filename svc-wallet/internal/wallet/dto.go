package wallet

import (
	"fmt"
	"strings"
	"time"
)

// i will define the DTOs for every request/response here for the wallet service

type CreateWalletRequest struct {
	// since they are 3 fields only i will not use a validation library
	OwnerName    string  `json:"owner_name"`
	PhoneNumber  string  `json:"phone_number"`
	CurrencyCode string  `json:"currency_code"`
	FamilyID     *string `json:"family_id,omitempty"` // a pointer because it can be null if the wallet is not part of a family
}

type CreateWalletResponse struct {
	WalletID string `json:"wallet_id"` // note that this is a string and not an ObjectID because
	// the client will not know how to handle an ObjectID so i will handle it by .HEX()
	Balance int64 `json:"balance"`

	//time stamps
	CreatedAt time.Time `json:"created_at"`
}

// this is the response for the get wallet endpoint the walletid
// will be passed in the url and the rest of the fields will be returned in the response
type GetWalletResponse struct {
	WalletID     string  `json:"wallet_id"`
	PhoneNumber  string  `json:"phone_number"`
	OwnerName    string  `json:"owner_name"`
	Balance      int64   `json:"balance"`
	CurrencyCode string  `json:"currency_code"`
	FamilyID     *string `json:"family_id,omitempty"`
}

type TransactionRequest struct {
	PhoneNumber string `json:"phone_number"` // request are made by phone number and not
	// wallet id because the user will not know the wallet id
	Amount int64 `json:"amount"`
}

type TransactionResponse struct {
	PhoneNumber string `json:"phone_number"`
	Balance     int64  `json:"balance"`
	// to annotate the response with a message to indicate if the transaction was successful or not
	Message string `json:"message"`

	//time stamps
	UpdatedAt time.Time `json:"updated_at"`
}

// validation function for the wallet
func (r *CreateWalletRequest) Validate() error {
	// 1. Trim spaces to prevent bypassing validation with empty spaces (e.g., "   ")
	owner := strings.TrimSpace(r.OwnerName)
	if len(owner) < 3 {
		return fmt.Errorf("owner name is required and must be at least 3 valid characters")
	}

	currency := strings.TrimSpace(r.CurrencyCode)
	if len(currency) != 3 {
		return fmt.Errorf("currency code is required and must be exactly 3 characters")
	}

	phone := strings.TrimSpace(r.PhoneNumber)
	if !strings.HasPrefix(phone, "+20") || len(phone) != 13 {
		return fmt.Errorf("phone number must start with +20 and contain exactly 13 characters")
	}

	// 2. Verify the payload is numeric (prevents +20ABCDEFGHIJ)
	for _, ch := range phone[3:] {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("phone number must contain only numeric digits after the country code")
		}
	}

	return nil
}
