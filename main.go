package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)

type closeFunc func() error

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

	logger, closeFunc, err := initializeLogger()
	defer func() {
		err := closeFunc()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to close log file: %v\n", err)
		}
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		return 1
	}

	st, err := store.New(dataDir, logger)

	if err != nil {
		logger.Printf("failed to create store: %v\n", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, logger)
	var serverErr error
	go func() {
		serverErr = s.start()
	}()
	logger.Printf("Linko is running on http://localhost:%d", httpPort)
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Printf("failed to shutdown server: %v\n", err)
		return 1
	}
	if serverErr != nil {
		logger.Printf("server error: %v\n", serverErr)
		return 1
	}
	return 0
}

func initializeLogger() (*log.Logger, closeFunc, error) {
	logFile, exists := os.LookupEnv("LINKO_LOG_FILE")
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	bufferedFile := bufio.NewWriterSize(file, 8192)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}
	var logger *log.Logger
	if exists {
		multiWriter := io.MultiWriter(os.Stderr, bufferedFile)
		logger = log.New(multiWriter, "", log.LstdFlags)
		closeFunc := func() error {
			err := bufferedFile.Flush()
			if err != nil {
				return err
			}
			err = file.Close()
			if err != nil {
				return err
			}
			return nil
		}
		return logger, closeFunc, nil
	} else {
		logger = log.New(os.Stderr, "", log.LstdFlags)
		closeFunc := func() error {
			return nil
		}
		return logger, closeFunc, nil
	}
}
