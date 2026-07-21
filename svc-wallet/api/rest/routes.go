package rest

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, controller *WalletController) {

	walletHander := otelhttp.NewHandler(http.HandlerFunc(controller.WalletHandler), "WalletHandler")

	// 1. (refactored) Create a new wallet or get an existing wallet by phone number
	mux.Handle("/v1/wallets/", walletHander)

	balanceHandler := otelhttp.NewHandler(http.HandlerFunc(controller.ModifyWalletBalance), "ModifyWalletBalance")
	// 2. Modify the balance of an existing wallet (deposit/withdraw)
	mux.Handle("/v1/wallets/balance", balanceHandler)
}
