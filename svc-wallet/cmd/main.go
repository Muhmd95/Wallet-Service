package main

import (
	"context"
	walletv1 "github.com/Muhmd95/Contracts/wallet/v1"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// project paths
	"svc-wallet/api/grpcserver"
	"svc-wallet/api/rest"
	"svc-wallet/external/mongodb"
	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
	"svc-wallet/util/tracer"
)

func main() {
	// Initialize logger
	logger.InitLogger("svc-wallet")
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
		logger.Log.Info().Msg("No .ENV file found, relying on os environment") // because when using docker env variables will be injected
	}

	// coneect the port
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8000" // default port
	}

	// get the wallet grpc port
	grpcPort := os.Getenv("GRPC_SERVER_PORT")
	if grpcPort == "" {
		grpcPort = "50051" // default grpc port
	}

	// get mongo uri
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		logger.Log.Fatal().Msg("MONGO_URI environment variable is required but not set")
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

	walletRepo, err := mongodb.NewWalletRepository(context.Background(), database) // passing context because this may introduce delay
	// the rest are only memory connections
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

	// ListenAndServe blocks forever unless it crashes
	//if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	//logger.Log.Fatal().Err(err).Msg("Server crashed")

	//}

	// 1. Run the server in a separate goroutine so it doesn't block the rest of the code
	go func() {
		logger.Log.Info().Str("port", port).Msg("Server is listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("Server crashed")
		}
	}()

	// run the grpc server in a separate goroutine
	// Initialize the gRPC server
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()), // adding the grpc interceptor
		// to extract the trace id from the incoming requests
	)
	myWalletServer := grpcserver.NewWalletServer(service)
	go func() {
		listener, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			logger.Log.Fatal().Err(err).Msg("Failed to listen on gRPC port")
		}

		// Register the gRPC server
		walletv1.RegisterWalletServiceServer(grpcServer, myWalletServer)
		logger.Log.Info().Str("port", grpcPort).Msg("gRPC server is listening")
		if err := grpcServer.Serve(listener); err != nil {
			logger.Log.Fatal().Err(err).Msg("gRPC server crashed")
		}
	}()

	// 2. Set up the signal listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// 3. Block until a shutdown signal is caught
	<-quit
	logger.Log.Info().Msg("Shutting down svc-wallet gracefully...")

	// 4. Wait up to 10 seconds for current requests to finish
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	// stop the grpc server gracefully
	grpcServer.GracefulStop()

	logger.Log.Info().Msg("svc-wallet exited safely")
}
