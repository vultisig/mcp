package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/config"
	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/tools"
	"github.com/vultisig/mcp/internal/vault"
)

func main() {
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

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
