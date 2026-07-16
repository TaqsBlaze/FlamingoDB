package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/TaqsBlaze/FlamingoDB/internal/network"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/catalog"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/disk"
	"github.com/TaqsBlaze/FlamingoDB/internal/storage/pager"
	"github.com/TaqsBlaze/FlamingoDB/pkg/config"
	"github.com/TaqsBlaze/FlamingoDB/pkg/logger"
)

func main() {
	tcpAddr := flag.String("tcp", ":4080", "TCP address to listen on")
	httpAddr := flag.String("http", ":8080", "HTTP address to listen on")
	username := flag.String("user", "admin", "Database auth username")
	password := flag.String("pass", "admin", "Database auth password")
	dataDir := flag.String("dir", "./data", "Database data directory")
	dbName := flag.String("db", "flamingo", "Database name")
	mcpMode := flag.Bool("mcp", false, "Run in stdio Model Context Protocol (MCP) server mode")
	policyName := flag.String("policy", "", "Policy name to enforce for MCP server queries")
	flag.Parse()

	log := logger.New(logger.LevelInfo)
	log.Info("Starting FlamingoDB server...")

	// 1. Initialize data directory
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Error("Failed to create data directory: %v", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(*dataDir, *dbName+".db")
	cfg := config.Default()
	cfg.DataDirectory = *dataDir

	// 2. Initialize storage engine components
	dm, err := disk.NewDiskManager(dbPath, cfg.PageSize)
	if err != nil {
		log.Error("Failed to initialize disk manager: %v", err)
		os.Exit(1)
	}

	p, err := pager.New(dm, cfg.PageSize)
	if err != nil {
		log.Error("Failed to initialize pager: %v", err)
		_ = dm.Close()
		os.Exit(1)
	}

	tm, err := catalog.NewTableManager(p)
	if err != nil {
		log.Error("Failed to initialize table manager: %v", err)
		_ = dm.Close()
		os.Exit(1)
	}

	// 3. Setup Network Server configuration
	netCfg := network.Config{
		TCPAddr:        *tcpAddr,
		HTTPAddr:       *httpAddr,
		Username:       *username,
		Password:       *password,
		DataDir:        *dataDir,
		MaxConnections: 100,
	}

	srv, err := network.NewServer(netCfg, tm, log)
	if err != nil {
		log.Error("Failed to initialize network server: %v", err)
		_ = tm.Close()
		_ = dm.Close()
		os.Exit(1)
	}

	// 4. Start server
	if *mcpMode {
		log.Info("Running in MCP mode over stdio...")
		// Redirect logs away from stdout so we don't pollute stdio JSON-RPC stream
		// We can change log output or set LevelError
		log = logger.New(logger.LevelError)
		srv.RunMCPServer(*policyName)
		_ = tm.Close()
		_ = dm.Close()
		os.Exit(0)
	}

	if err := srv.Start(); err != nil {
		log.Error("Failed to start network server: %v", err)
		_ = tm.Close()
		_ = dm.Close()
		os.Exit(1)
	}

	log.Info("FlamingoDB is ready to accept connections")

	// 5. Wait for termination signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Info("Received signal %v, shutting down...", sig)

	// Shutdown order: network server first, then table manager, then disk manager
	if err := srv.Close(); err != nil {
		log.Error("Error closing network server: %v", err)
	}
	if err := tm.Close(); err != nil {
		log.Error("Error closing table manager: %v", err)
	}
	if err := dm.Close(); err != nil {
		log.Error("Error closing disk manager: %v", err)
	}

	log.Info("FlamingoDB shutdown complete.")
}
