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

	// 1. Create wallet handler
	createWalletHandler := otelhttp.NewHandler(http.HandlerFunc(controller.CreateWallet), "CreateWallet")
	mux.Handle("/v1/wallet", createWalletHandler)

	// 1.1 get wallet by phone number as a path parameter
	getWalletHandler := otelhttp.NewHandler(http.HandlerFunc(controller.GetWallet), "GetWallet")
	mux.Handle("/v1/wallet/{phone_number}", getWalletHandler)

	// 2. Modify the balance of an existing wallet (deposit/withdraw)
	balanceHandler := otelhttp.NewHandler(http.HandlerFunc(controller.ModifyWalletBalance), "ModifyWalletBalance")
	mux.Handle("/v1/wallet/balance", balanceHandler)

	// 3. Swagger UI handler mounted directly to your mux
	mux.HandleFunc("/v1/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/v1/swagger/doc.json"),
	))

}
