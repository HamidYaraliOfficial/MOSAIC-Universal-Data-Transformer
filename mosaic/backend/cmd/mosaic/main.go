// Command mosaic is the entrypoint for the MOSAIC Go Data Engine. It is
// built as a standalone binary and launched by the Tauri shell as a sidecar
// process; the desktop UI never talks to anything except this process's
// local HTTP API (see internal/bridge).
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mosaic/internal/bridge"
)

func main() {
	var (
		port       = flag.Int("port", 0, "port to listen on (0 = pick a free port automatically)")
		dataDir    = flag.String("data-dir", defaultDataDir(), "directory for project storage and autosave")
		vaultPass  = flag.String("vault-key", os.Getenv("MOSAIC_VAULT_KEY"), "master key for the Secrets Vault (normally supplied by the OS keychain via the Tauri shell)")
	)
	flag.Parse()

	if *vaultPass == "" {
		*vaultPass = "mosaic-dev-key-change-me" // never used in a real build; the Tauri shell always supplies one
		log.Println("warning: no vault key supplied, using an insecure development default")
	}

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("mosaic: cannot create data dir %q: %v", *dataDir, err)
	}

	srv, err := bridge.New(*dataDir, *vaultPass)
	if err != nil {
		log.Fatalf("mosaic: failed to initialize backend: %v", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatalf("mosaic: failed to bind port: %v", err)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	// The Tauri shell reads this exact line from stdout to learn which
	// port the sidecar bound to (see frontend/src-tauri/src/main.rs).
	fmt.Printf("MOSAIC_ENGINE_PORT=%d\n", actualPort)
	log.Printf("MOSAIC Go Data Engine listening on 127.0.0.1:%d (data dir: %s)", actualPort, *dataDir)

	httpServer := &http.Server{Handler: srv.Handler()}

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("mosaic: server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("MOSAIC engine shutting down")
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mosaic"
	}
	return filepath.Join(home, ".mosaic")
}
