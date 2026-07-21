package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	// project paths
	"svc-wallet/api/rest"
	"svc-wallet/external/mongodb"
	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
)

func main() {
	// Initialize logger
	logger.InitLogger()
	slog.Info("Starting svc-wallet ...")

	// load .ENV file
	if err := godotenv.Load(".ENV"); err != nil {
		slog.Warn("Error loading .ENV file", "error", err.Error()) // because when using docker env variables will be injected
	}

	// coneect the port
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8000" // default port
	}

	// get mongo uri
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		slog.Error("MONGO_URI environment variable is required but not set")
		os.Exit(1) // We crash the app here because it cannot run without a database
	}

	// get database name
	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "wallet_db" // default database name
	}

	mongoClient, err := mongodb.ConnectMongoDB(mongoURI)
	if err != nil {
		slog.Error("Failed to connect to MongoDB", "error", err.Error())
		os.Exit(1) // We crash the app here because it cannot run without a database
	}

	// ensure to disconnect the database
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			slog.Error("Failed to disconnect MongoDB", "error", err.Error())
		}
	}()

	// init the database
	database := mongoClient.Database(dbName)
	slog.Info("Using database", "dbName", dbName)

	walletRepo, err := mongodb.NewWalletRepository(database)
	if err != nil {
		slog.Error("Failed to create wallet repository", "error", err.Error())
		os.Exit(1)
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

	slog.Info("Server is listening", "port", port)

	// ListenAndServe blocks forever unless it crashes
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server crashed", "error", err.Error())
		os.Exit(1)
	}

}
