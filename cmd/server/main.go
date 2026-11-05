package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	issuesv1 "jira-contracts/gen/go/issues/v1"
	"jira-issues-go/internal/config"
	grpchandler "jira-issues-go/internal/grpc"
	"jira-issues-go/internal/kafka"
	"jira-issues-go/internal/repository/postgres"
)

func main() {
	cfg := config.LoadConfig()
	log.Printf("[MS1] Starting jira-issues-go service on port :%s", cfg.GRPCPort)

	// 1. Initialize PostgreSQL Connection Pool
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[MS1] Failed to open database handle: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(ctxPing); err != nil {
		log.Printf("[MS1] Warning: Database ping failed (%v). Continuing startup...", err)
	} else {
		log.Println("[MS1] Connected to PostgreSQL (issues_db) successfully.")
	}
	cancelPing()

	// 2. Run Database Migrations if directory exists
	if _, err := os.Stat(cfg.MigrationsDir); err == nil {
		log.Printf("[MS1] Running migrations from %s...", cfg.MigrationsDir)
		if err := postgres.RunMigrations(db, cfg.MigrationsDir); err != nil {
			log.Printf("[MS1] Warning: Migration execution encountered: %v", err)
		} else {
			log.Println("[MS1] Database migrations applied successfully.")
		}
	}

	// 3. Initialize Repository & Kafka Producer
	repo := postgres.NewIssueRepository(db)
	producer := kafka.NewEventProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer producer.Close()

	// 4. Initialize gRPC Server
	grpcHandler := grpchandler.NewServer(repo, producer)
	grpcServer := grpc.NewServer()
	issuesv1.RegisterIssueServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	// 5. Bind TCP Listener
	addr := fmt.Sprintf(":%s", cfg.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[MS1] Failed to listen on %s: %v", addr, err)
	}

	// 6. Run Server in Background
	go func() {
		log.Printf("[MS1] gRPC server listening at %s", addr)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("[MS1] gRPC serve error: %v", err)
		}
	}()

	// 7. Graceful Shutdown on OS Signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("[MS1] Received signal %v. Initiating graceful shutdown...", sig)

	shutdownDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		log.Println("[MS1] gRPC server stopped cleanly.")
	case <-time.After(10 * time.Second):
		log.Println("[MS1] Graceful shutdown timeout exceeded. Forcing stop.")
		grpcServer.Stop()
	}

	log.Println("[MS1] Shutdown complete. Goodbye.")
}
