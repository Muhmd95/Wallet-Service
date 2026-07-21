package main

import (
	"context"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	// project paths
	"svc-wallet/api/rest"
	"svc-wallet/external/mongodb"
	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
	"svc-wallet/util/tracer"
)

func main() {
	// Initialize logger
	logger.InitLogger()
	logger.Log.Info().Msg("Starting svc-wallet ...")

	// init the tracer
	tp, err := tracer.InitTracer("svc-wallet")
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize tracer")
	}

	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			logger.Log.Error().Err(err).Msg("Failed to shutdown tracer")
		}
	}()

	// load .ENV file
	if err := godotenv.Load(".ENV"); err != nil {
		logger.Log.Warn().Err(err).Msg("Error loading .ENV file") // because when using docker env variables will be injected
	}

	// coneect the port
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8000" // default port
	}

	// get mongo uri
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		logger.Log.Fatal().Err(err).Msg("MONGO_URI environment variable is required but not set")
		// fatal crashes the app dont need to use os.exit(1)
	}

	// get database name
	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "wallet_db" // default database name
	}

	mongoClient, err := mongodb.ConnectMongoDB(mongoURI)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to connect to MongoDB")
		// We crash the app here because it cannot run without a database
	}

	// ensure to disconnect the database
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Log.Error().Err(err).Msg("Failed to disconnect MongoDB")
		}
	}()

	// init the database
	database := mongoClient.Database(dbName)
	logger.Log.Info().Str("dbName", dbName).Msg("Using database")

	walletRepo, err := mongodb.NewWalletRepository(database)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to create wallet repository")
	}

	// Initialize the wallet service
	service := wallet.NewService(walletRepo)

	// Initialize the REST API handler
	controller := rest.NewWalletController(service)

	// create a server mux
	mux := http.NewServeMux()
	// reguster the routes
	rest.RegisterRoutes(mux, controller)

	// Start the HTTP server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	logger.Log.Info().Str("port", port).Msg("Server is listening")

	// ListenAndServe blocks forever unless it crashes
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Log.Fatal().Err(err).Msg("Server crashed")

	}

}
