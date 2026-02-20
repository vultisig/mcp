package main

import (
	"flag"
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/config"
	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/tools"
	"github.com/vultisig/mcp/internal/vault"
)

func main() {
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :8080). If empty, serves over stdio.")
	flag.Parse()

	cfg := config.Load()

	ethClient, err := ethereum.NewClient(cfg.ETHRPCURL)
	if err != nil {
		log.Fatalf("failed to create ethereum client: %v", err)
	}
	defer ethClient.Close()

	store := vault.NewStore()

	s := server.NewMCPServer("vultisig-mcp", "0.1.0",
		server.WithToolCapabilities(true),
	)

	tools.RegisterAll(s, store, ethClient)

	if *httpAddr != "" {
		httpServer := server.NewStreamableHTTPServer(s)
		log.Printf("MCP server listening on %s/mcp", *httpAddr)
		if err := httpServer.Start(*httpAddr); err != nil {
			log.Fatalf("http server error: %v", err)
		}
	} else {
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
