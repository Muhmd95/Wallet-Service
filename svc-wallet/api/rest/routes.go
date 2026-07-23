package rest

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
	_ "svc-wallet/docs"
)

// @title           Wallet Service API
// @version         1.0
// @description     This microservice handles wallets.
// @host            localhost:8000
// @BasePath        /v1
func RegisterRoutes(mux *http.ServeMux, controller *WalletController) {

	walletHander := otelhttp.NewHandler(http.HandlerFunc(controller.WalletHandler), "WalletHandler")

	// 1. (refactored) Create a new wallet or get an existing wallet by phone number
	mux.Handle("/v1/wallet", walletHander)

	balanceHandler := otelhttp.NewHandler(http.HandlerFunc(controller.ModifyWalletBalance), "ModifyWalletBalance")
	// 2. Modify the balance of an existing wallet (deposit/withdraw)
	mux.Handle("/v1/wallet/balance", balanceHandler)

	// 3. Swagger UI handler mounted directly to your mux
	mux.HandleFunc("/v1/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/v1/swagger/doc.json"),
	))

}
