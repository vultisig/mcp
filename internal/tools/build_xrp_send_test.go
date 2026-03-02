package tools

import (
	"encoding/hex"
	"strings"
	"testing"

	xrpgo "github.com/xyield/xrpl-go/binary-codec"
)

func TestBuildXRPLPayment_Simple(t *testing.T) {
	txBytes, err := buildXRPLPayment(
		"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
		1000000,
		42,
		12,
		75801900,
		"0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
		"",
	)
	if err != nil {
		t.Fatalf("buildXRPLPayment: %v", err)
	}

	if len(txBytes) == 0 {
		t.Fatal("expected non-empty tx bytes")
	}

	decoded, err := xrpgo.Decode(hex.EncodeToString(txBytes))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}

	if decoded["TransactionType"] != "Payment" {
		t.Errorf("TransactionType = %v, want Payment", decoded["TransactionType"])
	}
	if decoded["Account"] != "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh" {
		t.Errorf("Account = %v", decoded["Account"])
	}
	if decoded["Destination"] != "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN" {
		t.Errorf("Destination = %v", decoded["Destination"])
	}
	if decoded["Amount"] != "1000000" {
		t.Errorf("Amount = %v, want 1000000", decoded["Amount"])
	}
	if decoded["Fee"] != "12" {
		t.Errorf("Fee = %v, want 12", decoded["Fee"])
	}

	if _, ok := decoded["Memos"]; ok {
		t.Error("expected no Memos field for simple payment")
	}
}

func TestBuildXRPLPayment_WithMemo(t *testing.T) {
	memo := "=:ETH.ETH:0x1234567890abcdef1234567890abcdef12345678"
	txBytes, err := buildXRPLPayment(
		"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
		5000000,
		10,
		15,
		75802000,
		"0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
		memo,
	)
	if err != nil {
		t.Fatalf("buildXRPLPayment with memo: %v", err)
	}

	decoded, err := xrpgo.Decode(hex.EncodeToString(txBytes))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}

	memos, ok := decoded["Memos"]
	if !ok {
		t.Fatal("expected Memos field for swap payment")
	}

	memoList, ok := memos.([]any)
	if !ok || len(memoList) == 0 {
		t.Fatal("expected non-empty Memos array")
	}

	memoEntry, ok := memoList[0].(map[string]any)
	if !ok {
		t.Fatal("expected Memo entry to be a map")
	}

	memoObj, ok := memoEntry["Memo"].(map[string]any)
	if !ok {
		t.Fatal("expected Memo object to be a map")
	}

	memoData, ok := memoObj["MemoData"].(string)
	if !ok {
		t.Fatal("expected MemoData to be string")
	}

	dataBytes, err := hex.DecodeString(memoData)
	if err != nil {
		t.Fatalf("decode MemoData hex: %v", err)
	}
	if string(dataBytes) != memo {
		t.Errorf("MemoData decoded = %q, want %q", string(dataBytes), memo)
	}
}

func TestBuildXRPLPayment_Canonical(t *testing.T) {
	txBytes1, err := buildXRPLPayment(
		"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
		1000000, 1, 12, 100,
		"0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	txBytes2, err := buildXRPLPayment(
		"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
		1000000, 1, 12, 100,
		"0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	hex1 := hex.EncodeToString(txBytes1)
	hex2 := hex.EncodeToString(txBytes2)
	if hex1 != hex2 {
		t.Errorf("canonical encoding not deterministic:\n  %s\n  %s", hex1, hex2)
	}
}

func TestBuildXRPLPayment_PubKeyUppercase(t *testing.T) {
	txBytes, err := buildXRPLPayment(
		"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
		"rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
		1000000, 1, 12, 100,
		"0330e7fc9d56bb25d6893ba3f317ae5bcf33b3291bd63db32654a313222f7fd020",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := xrpgo.Decode(hex.EncodeToString(txBytes))
	if err != nil {
		t.Fatal(err)
	}

	pubKey, ok := decoded["SigningPubKey"].(string)
	if !ok {
		t.Fatal("missing SigningPubKey")
	}
	if pubKey != strings.ToUpper(pubKey) {
		t.Errorf("SigningPubKey should be uppercase, got %q", pubKey)
	}
}
