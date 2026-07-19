package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	fmt.Println("Linko is shutting down")
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {
	file, err := os.OpenFile("linko.access.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return 1
	}
	accessLogger := log.New(file, "INFO: ", log.LstdFlags)
	standardLogger := log.New(os.Stderr, "DEBUG: ", log.LstdFlags)

	st, err := store.New(dataDir, standardLogger)

	if err != nil {
		standardLogger.Printf("failed to create store: %v\n", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, accessLogger)
	var serverErr error
	go func() {
		serverErr = s.start()
	}()
	standardLogger.Printf("Linko is running on http://localhost:%d", httpPort)
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.shutdown(shutdownCtx); err != nil {
		standardLogger.Printf("failed to shutdown server: %v\n", err)
		return 1
	}
	if serverErr != nil {
		standardLogger.Printf("server error: %v\n", serverErr)
		return 1
	}
	return 0
}
