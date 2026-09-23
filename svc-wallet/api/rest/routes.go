package rest

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "svc-wallet/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Wallet Service API
// @version         1.0
// @description     This microservice handles wallets.
// @host            localhost:8000
// @BasePath        /v1
func RegisterRoutes(mux *http.ServeMux, controller *WalletController) {

	// 1. Create wallet handler
	createWalletHandler := otelhttp.NewHandler(http.HandlerFunc(controller.CreateWallet), "CreateWallet")
	mux.Handle("POST /v1/wallet", createWalletHandler)

	// 2. get wallet by phone number as a path parameter
	getWalletHandlerPN := otelhttp.NewHandler(http.HandlerFunc(controller.GetWalletByPhoneNumber), "GetWalletByPhoneNumber")
	mux.Handle("GET /v1/wallet/phone/{phone_number}", getWalletHandlerPN)

	// 3. get wallet by id
	getWalletHandlerID := otelhttp.NewHandler(http.HandlerFunc(controller.GetWalletByID), "GetWalletByID")
	mux.Handle("GET /v1/wallet/{wallet_id}", getWalletHandlerID)

	// 4. get wallet balance by wallet id
	getWalletBalanceHandler := otelhttp.NewHandler(http.HandlerFunc(controller.GetBalanceByWalletID), "GetWalletBalance")
	mux.Handle("GET /v1/wallet/balance/{wallet_id}", getWalletBalanceHandler)

	// 5. Swagger UI handler mounted directly to your mux
	mux.HandleFunc("/v1/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/v1/swagger/doc.json"),
	))

	// 6. Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())
}
