package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"

	btcsdk "github.com/vultisig/recipes/sdk/btc"
	"github.com/vultisig/recipes/sdk/swap"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/coingecko"
	"github.com/vultisig/mcp/internal/config"
	evmclient "github.com/vultisig/mcp/internal/evm"
	mcplog "github.com/vultisig/mcp/internal/logging"
	"github.com/vultisig/mcp/internal/skills"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/tools"
	"github.com/vultisig/mcp/internal/vault"
)

func main() {
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :8080). If empty, serves over stdio.")
	flag.Parse()

	logger := log.New(os.Stderr, "[mcp] ", log.LstdFlags|log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("failed to load config: %v", err)
	}

	evmPool := evmclient.NewPool(cfg.EVM.ToURLMap())
	defer evmPool.Close()

	store := vault.NewStore()
	cgClient := coingecko.NewClient(cfg.CoinGeckoAPIKey)
	bcClient := blockchair.NewClient(cfg.BlockchairURL)

	hooks := mcplog.NewHooks(logger)

	s := server.NewMCPServer("vultisig-mcp", "0.1.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, true),
		server.WithHooks(hooks),
		server.WithToolHandlerMiddleware(mcplog.NewToolMiddleware(logger)),
		server.WithRecovery(),
	)

	swapSvc := swap.NewService()
	utxoBuilder := btcsdk.Mainnet()
	tcClient := thorchain.NewClient(cfg.ThorchainURL)
	tools.RegisterAll(s, store, evmPool, cgClient, bcClient, swapSvc, utxoBuilder, tcClient)
	skills.RegisterMCPResources(s)

	if *httpAddr != "" {
		mcpHandler := server.NewStreamableHTTPServer(s)

		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpHandler)
		skillHandler := skills.NewHandler(logger)
		mux.Handle("/skills", skillHandler)
		mux.Handle("/skills/", skillHandler)

		logger.Printf("listening on %s (HTTP mode)", *httpAddr)
		srv := &http.Server{Addr: *httpAddr, Handler: mux}
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatalf("http server error: %v", err)
		}
	} else {
		logger.Printf("serving on stdio")
		if err := server.ServeStdio(s); err != nil {
			logger.Fatalf("server error: %v", err)
		}
	}
}
