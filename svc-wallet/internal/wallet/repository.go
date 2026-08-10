package wallet

// i will create the interface and implementation of the repository layer
// in the same file so it will be easier to read and more compact

import (
	"context" // this is used to pass the context from the controller
	//  to the service layer and then to the repository layer so if the
	// http request is cancelled or times out the context will be cancelled
	//  and the repository layer will stop the database operation
)

type Repository interface {
	// this is the interface for the repository layer so that the service layer can use it
	// without knowing the implementation details of the repository layer
	CreateWallet(ctx context.Context, wallet *Wallet) error

	GetWalletByPhoneNumber(ctx context.Context, phoneNumber string) (*Wallet, error)

	UpdateWalletBalance(ctx context.Context, phoneNumber string, amount int64, refID string) (*Wallet, error)

	GetWalletByID(ctx context.Context, walletID string) (*Wallet, error)
}
