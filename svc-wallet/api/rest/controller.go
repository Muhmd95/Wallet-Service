package rest

import (
	"svc-wallet/internal/wallet"
)

// WalletController handles HTTP requests related to wallet operations.
type WalletController struct {
	service *wallet.Service
}

// NewWalletController creates a new instance of WalletController with the provided wallet service.
func NewWalletController(service *wallet.Service) *WalletController {
	return &WalletController{service: service}
}
