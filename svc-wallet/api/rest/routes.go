package rest

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, controller *WalletController) {

	// 1. Create a new wallet
	mux.HandleFunc("POST /v1/wallets", RequestIDMiddleware(controller.CreateWallet))

	// 2. Retrieve an existing wallet by phone number
	mux.HandleFunc("GET /v1/wallets", RequestIDMiddleware(controller.GetWallet))

	// 3. Modify the balance of an existing wallet (deposit/withdraw)
	mux.HandleFunc("PATCH /v1/wallets/balance", RequestIDMiddleware(controller.ModifyWalletBalance))
}
