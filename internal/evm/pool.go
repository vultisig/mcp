package evm

import (
	"context"
	"fmt"
	"math/big"
	"sync"
)

type entry struct {
	client  *Client
	chainID *big.Int
}

// Pool manages a set of EVM clients, one per chain, with lazy initialization.
type Pool struct {
	urls    map[string]string
	mu      sync.Mutex
	clients map[string]*entry
}

func NewPool(urls map[string]string) *Pool {
	return &Pool{
		urls:    urls,
		clients: make(map[string]*entry),
	}
}

// Get returns the client and chain ID for the named EVM chain.
// The client is created on first call and cached for subsequent calls.
func (p *Pool) Get(ctx context.Context, chainName string) (*Client, *big.Int, error) {
	p.mu.Lock()
	e, ok := p.clients[chainName]
	p.mu.Unlock()
	if ok {
		return e.client, e.chainID, nil
	}

	url, ok := p.urls[chainName]
	if !ok || url == "" {
		return nil, nil, fmt.Errorf("no RPC URL configured for chain %q", chainName)
	}

	client, err := NewClient(url)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s RPC: %w", chainName, err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("get chain ID for %s: %w", chainName, err)
	}

	p.mu.Lock()
	if existing, ok := p.clients[chainName]; ok {
		p.mu.Unlock()
		client.Close()
		return existing.client, existing.chainID, nil
	}
	e = &entry{client: client, chainID: chainID}
	p.clients[chainName] = e
	p.mu.Unlock()

	return client, chainID, nil
}

// Close closes all open clients.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.clients {
		e.client.Close()
	}
}
