package gaia

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestGetBalance(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := balanceResponse{}
		resp.Balance.Denom = "uatom"
		resp.Balance.Amount = "5000000"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	balance, err := client.GetBalance(context.Background(), "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if balance != "5000000" {
		t.Errorf("balance = %q, want %q", balance, "5000000")
	}
}

func TestGetBalance_EmptyAmount(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := balanceResponse{}
		resp.Balance.Denom = "uatom"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	balance, err := client.GetBalance(context.Background(), "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if balance != "0" {
		t.Errorf("balance = %q, want %q", balance, "0")
	}
}

func TestGetBalance_NotFound(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBalance(context.Background(), "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetBalance_ServerError(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetBalance(context.Background(), "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetAccount(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := accountResponse{}
		resp.Account.AccountNumber = "12345"
		resp.Account.Sequence = "7"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	info, err := client.GetAccount(context.Background(), "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if info.AccountNumber != "12345" {
		t.Errorf("AccountNumber = %q, want %q", info.AccountNumber, "12345")
	}
	if info.Sequence != "7" {
		t.Errorf("Sequence = %q, want %q", info.Sequence, "7")
	}
}

func TestGetAccount_NotFound(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetAccount(context.Background(), "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetTransactionStatus(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := txResponse{}
		resp.TxResponse.TxHash = "ABCD1234"
		resp.TxResponse.Height = "100"
		resp.TxResponse.Code = 0
		resp.TxResponse.GasUsed = "50000"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	status, err := client.GetTransactionStatus(context.Background(), "ABCD1234")
	if err != nil {
		t.Fatalf("GetTransactionStatus: %v", err)
	}
	if status.TxHash != "ABCD1234" {
		t.Errorf("TxHash = %q, want %q", status.TxHash, "ABCD1234")
	}
	if status.Height != "100" {
		t.Errorf("Height = %q, want %q", status.Height, "100")
	}
	if status.Code != 0 {
		t.Errorf("Code = %d, want 0", status.Code)
	}
	if status.GasUsed != "50000" {
		t.Errorf("GasUsed = %q, want %q", status.GasUsed, "50000")
	}
}

func TestGetTransactionStatus_Failed(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := txResponse{}
		resp.TxResponse.TxHash = "FAIL1234"
		resp.TxResponse.Height = "99"
		resp.TxResponse.Code = 5
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	status, err := client.GetTransactionStatus(context.Background(), "FAIL1234")
	if err != nil {
		t.Fatalf("GetTransactionStatus: %v", err)
	}
	if status.Code != 5 {
		t.Errorf("Code = %d, want 5", status.Code)
	}
}

func TestGetTransactionStatus_NotFound(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetTransactionStatus(context.Background(), "MISSING")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetTransactionStatus_InvalidJSON(t *testing.T) {
	client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	})

	_, err := client.GetTransactionStatus(context.Background(), "BADJSON")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid cosmos address",
			address: "cosmos1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02",
			wantErr: false,
		},
		{
			name:    "wrong prefix",
			address: "osmo1hsk6jryyqjfhp5dhc55tc9jtckygx0eph6dd02",
			wantErr: true,
		},
		{
			name:    "empty string",
			address: "",
			wantErr: true,
		},
		{
			name:    "random string",
			address: "notanaddress",
			wantErr: true,
		},
		{
			name:    "ethereum address",
			address: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD45",
			wantErr: true,
		},
		{
			name:    "wrong data length (10 bytes)",
			address: "cosmos1qypqxpq9qcrsszg2789qmz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress(%q) error = %v, wantErr %v", tt.address, err, tt.wantErr)
			}
		})
	}
}

func TestFormatUATOM(t *testing.T) {
	tests := []struct {
		name  string
		uatom *big.Int
		want  string
	}{
		{
			name:  "zero",
			uatom: big.NewInt(0),
			want:  "0.000000",
		},
		{
			name:  "one uatom",
			uatom: big.NewInt(1),
			want:  "0.000001",
		},
		{
			name:  "one atom",
			uatom: big.NewInt(1_000_000),
			want:  "1.000000",
		},
		{
			name:  "fractional",
			uatom: big.NewInt(1_234_567),
			want:  "1.234567",
		},
		{
			name:  "large amount",
			uatom: big.NewInt(123_456_789_012),
			want:  "123456.789012",
		},
		{
			name:  "small fraction",
			uatom: big.NewInt(500_000),
			want:  "0.500000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUATOM(tt.uatom)
			if got != tt.want {
				t.Errorf("FormatUATOM(%s) = %q, want %q", tt.uatom, got, tt.want)
			}
		})
	}
}
