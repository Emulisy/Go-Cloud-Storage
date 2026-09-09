// Command server runs the single-process file storage HTTP service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Emulisy/Go-Cloud-Storage/internal/download"
	"github.com/Emulisy/Go-Cloud-Storage/internal/httpapi"
	"github.com/Emulisy/Go-Cloud-Storage/internal/storage/local"
	"github.com/Emulisy/Go-Cloud-Storage/internal/storage/memory"
	"github.com/Emulisy/Go-Cloud-Storage/internal/upload"
)

// Runtime settings are fixed here until configuration loading is introduced.
const (
	serverAddress     = ":8080"
	blobDirectory     = "data/blobs"
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

// run owns dependency construction and waits for serving to fail or shutdown to
// finish. Returning errors keeps deferred cleanup outside main's exit path.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metadataStore := memory.NewMetadataStore(nil)
	blobStore, err := local.NewBlobStore(blobDirectory)
	if err != nil {
		return fmt.Errorf("create blob store: %w", err)
	}

	uploadService := upload.NewService(blobStore, metadataStore)
	downloadService := download.NewService(blobStore, metadataStore)
	handler := httpapi.NewHandler(metadataStore, uploadService, downloadService)

	server := &http.Server{
		Addr:              serverAddress,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	// Buffer the result so the serving goroutine can exit while shutdown is running.
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("server listening on %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		// Restore signal handling so a second interrupt can force an exit.
		stop()
	}

	log.Print("shutting down server")
	// The signal context is already canceled; draining needs its own deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		closeErr := server.Close()
		return fmt.Errorf("shut down HTTP server: %w", errors.Join(err, closeErr))
	}

	if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
