package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "tictactoe/api/gen/tictactoe"
	"tictactoe/internal/server"
	"tictactoe/internal/store"
	"tictactoe/internal/swagger"
)

func main() {
	// Parse command line flags
	grpcPort := flag.Int("grpc-port", 50051, "The gRPC server port")
	httpPort := flag.Int("http-port", 8080, "The HTTP/REST server port")
	shards := flag.Int("shards", 64, "Number of shards for data stores (higher = better concurrency)")
	flag.Parse()

	// Create stores
	gameStore := store.NewGameStore(*shards)
	statsStore := store.NewStatsStore(*shards)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register our service
	ticTacToeServer := server.NewTicTacToeServer(gameStore, statsStore)
	pb.RegisterTicTacToeServiceServer(grpcServer, ticTacToeServer)

	// Register reflection service for tools like grpcurl
	reflection.Register(grpcServer)

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", *grpcPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
	}

	go func() {
		log.Printf("gRPC server listening on %s", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Create gRPC-Gateway mux
	ctx := context.Background()
	gwMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	err = pb.RegisterTicTacToeServiceHandlerFromEndpoint(ctx, gwMux, grpcAddr, opts)
	if err != nil {
		log.Fatalf("Failed to register gateway: %v", err)
	}

	// Create HTTP mux for serving Swagger UI and API
	httpMux := http.NewServeMux()

	// Serve Swagger JSON
	httpMux.HandleFunc("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, "api/swagger/tictactoe.swagger.json")
	})

	// Serve Swagger UI
	swaggerFS, err := fs.Sub(swagger.SwaggerUI, "swagger-ui")
	if err != nil {
		log.Fatalf("Failed to load Swagger UI: %v", err)
	}
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFS))))

	// Redirect /swagger to /swagger/
	httpMux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})

	// Health check endpoint
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// CORS middleware wrapper
	corsHandler := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			h.ServeHTTP(w, r)
		})
	}

	// Route API requests to gRPC-Gateway, others to httpMux
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			gwMux.ServeHTTP(w, r)
		} else {
			httpMux.ServeHTTP(w, r)
		}
	})

	// Start HTTP server
	httpAddr := fmt.Sprintf(":%d", *httpPort)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: corsHandler(mainHandler),
	}

	go func() {
		log.Printf("HTTP/REST server listening on %s", httpAddr)
		log.Printf("Swagger UI available at http://localhost%s/swagger/", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down servers...")
	httpServer.Shutdown(ctx)
	grpcServer.GracefulStop()
	log.Println("Servers stopped")
}
