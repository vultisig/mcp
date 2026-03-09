package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func mockXRPServer(t *testing.T, sequence uint32, ledger uint32, fee uint64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		method, _ := req["method"].(string)
		w.Header().Set("Content-Type", "application/json")

		switch method {
		case "account_info":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"status": "success",
					"account_data": map[string]any{
						"Sequence": sequence,
					},
				},
			})
		case "ledger":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"status":       "success",
					"ledger_index": ledger,
				},
			})
		case "fee":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"status": "success",
					"drops": map[string]any{
						"base_fee": strconv.FormatUint(fee, 10),
					},
				},
			})
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
		}
	}))
}

func setupXRPVault(t *testing.T) (*vault.Store, string) {
	t.Helper()
	store := vault.NewStore()
	store.Set("default", vault.Info{
		ECDSAPublicKey: testECDSAPubKey,
		ChainCode:      testChainCode,
	})
	addr, _, _, err := address.GetAddress(testECDSAPubKey, testChainCode, common.XRP)
	if err != nil {
		t.Fatalf("derive XRP address: %v", err)
	}
	return store, addr
}

func TestBuildXRPSend_Basic(t *testing.T) {
	store, senderAddr := setupXRPVault(t)
	srv := mockXRPServer(t, 42, 1000000, 12)
	defer srv.Close()

	client := xrpclient.NewClient(srv.URL)
	handler := handleBuildXRPSend(store, client)

	req := callToolReq("build_xrp_send", map[string]any{
		"to":     "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"amount": "1000000",
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &result)
	if err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["chain"] != "Ripple" {
		t.Errorf("expected chain Ripple, got %v", result["chain"])
	}
	if result["account"] != senderAddr {
		t.Errorf("expected account %s, got %v", senderAddr, result["account"])
	}
	if result["destination"] != "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh" {
		t.Errorf("unexpected destination: %v", result["destination"])
	}
	if result["action"] != "transfer" {
		t.Errorf("expected action transfer, got %v", result["action"])
	}
	if result["transaction_type"] != "Payment" {
		t.Errorf("expected transaction_type Payment, got %v", result["transaction_type"])
	}
}

func TestBuildXRPSend_WithMemo(t *testing.T) {
	store, _ := setupXRPVault(t)
	srv := mockXRPServer(t, 42, 1000000, 12)
	defer srv.Close()

	client := xrpclient.NewClient(srv.URL)
	handler := handleBuildXRPSend(store, client)

	req := callToolReq("build_xrp_send", map[string]any{
		"to":     "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"amount": "1000000",
		"memo":   "SWAP:ETH.ETH:0x1234",
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &result)
	if err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["action"] != "swap" {
		t.Errorf("expected action swap, got %v", result["action"])
	}
	if result["memo"] != "SWAP:ETH.ETH:0x1234" {
		t.Errorf("unexpected memo: %v", result["memo"])
	}
}

func TestBuildXRPSend_InvalidAddress(t *testing.T) {
	store, _ := setupXRPVault(t)
	srv := mockXRPServer(t, 42, 1000000, 12)
	defer srv.Close()

	client := xrpclient.NewClient(srv.URL)
	handler := handleBuildXRPSend(store, client)

	req := callToolReq("build_xrp_send", map[string]any{
		"to":     "not-valid-xrp",
		"amount": "1000000",
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for invalid address")
	}
}
