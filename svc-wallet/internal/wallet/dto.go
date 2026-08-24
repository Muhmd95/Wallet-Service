package wallet

import (
	"fmt"
	"strconv"
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
	NationalID   string  `json:"national_id"`
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
	WalletID     string    `json:"wallet_id"`
	PhoneNumber  string    `json:"phone_number"`
	OwnerName    string    `json:"owner_name"`
	Balance      int64     `json:"balance"`
	CurrencyCode string    `json:"currency_code"`
	FamilyID     *string   `json:"family_id,omitempty"`
	NationalID   string    `json:"national_id"`
	BirthDate    time.Time `json:"birth_date"`
}

type UpdateWalletBalanceRequest struct {
	PhoneNumber string `json:"phone_number"`
	Amount      int64  `json:"amount"` // this can be positive or negative depending on the operation
	// in the future i will add the currency code
}

type UpdateWalletBalanceResponse struct {
	WalletID string `json:"wallet_id"`
	Balance  int64  `json:"balance"`
	// currency code will be added in the future
	UpdatedAt time.Time `json:"updated_at"`
}

// validation function for the wallet
func (r *CreateWalletRequest) Validate() error {
	// 1. Trim spaces to prevent bypassing validation with empty spaces (e.g., "   ")
	r.OwnerName = strings.TrimSpace(r.OwnerName)
	if len(r.OwnerName) < 3 {
		return fmt.Errorf("owner name is required and must be at least 3 valid characters")
	}

	r.CurrencyCode = strings.TrimSpace(r.CurrencyCode)
	if len(r.CurrencyCode) != 3 {
		return fmt.Errorf("currency code is required and must be exactly 3 characters")
	}

	// verify the phone number is valid and starts with +20 and is 13 characters long
	r.PhoneNumber = strings.TrimSpace(r.PhoneNumber)

	if err := ValidatePhoneNumber(&r.PhoneNumber); err != nil {
		return err
	}

	if err := ValidateNationalID(r.NationalID); err != nil {
		return err
	}

	return nil
}

func ValidatePhoneNumber(phoneNumber *string) error {
	*phoneNumber = strings.TrimSpace(*phoneNumber)
	if !(strings.HasPrefix(*phoneNumber, "010") || strings.HasPrefix(*phoneNumber, "011") || strings.HasPrefix(*phoneNumber, "012") || strings.HasPrefix(*phoneNumber, "015")) || len(*phoneNumber) != 11 {
		return ErrInvalidPhoneNumber // return the domain error for invalid phone number format
	}

	// 2. Verify the payload is numeric (prevents +20ABCDEFGHIJ)
	for _, ch := range *phoneNumber {
		if ch < '0' || ch > '9' {
			return ErrInvalidPhoneNumber // return the domain error for invalid phone number format
		}
	}
	return nil
}

func ValidateNationalID(nationalID string) error {
	if len(nationalID) != 14 {
		return ErrInvalidNationalID
	}

	// 2. Verify the payload is numeric
	for _, ch := range nationalID {
		if ch < '0' || ch > '9' {
			return ErrInvalidNationalID
		}
	}

	if nationalID[0] != '2' && nationalID[0] != '3' {
		return ErrInvalidNationalID
	}
	month, err := strconv.Atoi(nationalID[3:5])
	if err != nil {
		return err
	}
	if month <= 0 || month > 12 {
		return ErrInvalidNationalID
	}

	day, err := strconv.Atoi(nationalID[5:7])
	if err != nil {
		return err
	}
	if day <= 0 || day > 31 {
		return ErrInvalidNationalID
	}

	return nil
}
