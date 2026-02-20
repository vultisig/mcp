package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/config"
	"github.com/vultisig/mcp/internal/ethereum"
	mcplog "github.com/vultisig/mcp/internal/logging"
	"github.com/vultisig/mcp/internal/tools"
	"github.com/vultisig/mcp/internal/vault"
)

func main() {
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :8080). If empty, serves over stdio.")
	flag.Parse()

	cfg := config.Load()

	logger := log.New(os.Stderr, "[mcp] ", log.LstdFlags|log.Lmicroseconds)

	ethClient, err := ethereum.NewClient(cfg.ETHRPCURL)
	if err != nil {
		logger.Fatalf("failed to create ethereum client: %v", err)
	}
	defer ethClient.Close()

	chainID, err := ethClient.ChainID(context.Background())
	if err != nil {
		logger.Fatalf("failed to detect chain ID: %v", err)
	}
	logger.Printf("connected to chain %s (RPC: %s)", chainID.String(), cfg.ETHRPCURL)

	store := vault.NewStore()

	hooks := mcplog.NewHooks(logger)

	s := server.NewMCPServer("vultisig-mcp", "0.1.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
		server.WithToolHandlerMiddleware(mcplog.NewToolMiddleware(logger)),
		server.WithRecovery(),
	)

	tools.RegisterAll(s, store, ethClient, chainID)

	if *httpAddr != "" {
		httpServer := server.NewStreamableHTTPServer(s)
		logger.Printf("listening on %s/mcp (HTTP mode)", *httpAddr)
		if err := httpServer.Start(*httpAddr); err != nil {
			logger.Fatalf("http server error: %v", err)
		}
	} else {
		logger.Printf("serving on stdio")
		if err := server.ServeStdio(s); err != nil {
			logger.Fatalf("server error: %v", err)
		}
	}
}
