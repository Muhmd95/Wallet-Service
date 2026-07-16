package wallet

import (
	"time"
)

// i will define the DTOs for every request/response here for the wallet service

type CreateWalletRequest struct {
	// since they are 3 fields only i will validate myself in the service layer and not use a validation library
	OwnerName    string  `json:"owner_name"`
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
	OwnerName    string  `json:"owner_name"`
	Balance      int64   `json:"balance"`
	CurrencyCode string  `json:"currency_code"`
	FamilyID     *string `json:"family_id,omitempty"`
}

type TransactionRequest struct {
	WalletID string `json:"wallet_id"`
	Amount   int64  `json:"amount"`
}

type TransactionResponse struct {
	WalletID string `json:"wallet_id"`
	Balance  int64  `json:"balance"`
	// to annotate the response with a message to indicate if the transaction was successful or not
	Message string `json:"message"`

	//time stamps
	UpdatedAt time.Time `json:"updated_at"`
}
