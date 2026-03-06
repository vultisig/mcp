package etherscan

import "encoding/json"

// apiResponse is the standard Etherscan V2 envelope.
type apiResponse struct {
	Status  string          `json:"status"`  // "1" = success, "0" = error
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// SourceInfo holds contract source/verification data from getsourcecode.
type SourceInfo struct {
	ContractName    string `json:"ContractName"`
	CompilerVersion string `json:"CompilerVersion"`
	ABI             string `json:"ABI"`
	Proxy           string `json:"Proxy"`          // "1" if proxy
	Implementation  string `json:"Implementation"` // implementation address if proxy
}

// Transaction is a single entry from the txlist endpoint.
type Transaction struct {
	Hash            string `json:"hash"`
	From            string `json:"from"`
	To              string `json:"to"`
	Value           string `json:"value"`            // wei
	Gas             string `json:"gas"`
	GasUsed         string `json:"gasUsed"`
	GasPrice        string `json:"gasPrice"`         // wei
	IsError         string `json:"isError"`           // "0" = success, "1" = error
	TxReceiptStatus string `json:"txreceipt_status"`  // "1" = success
	BlockNumber     string `json:"blockNumber"`
	TimeStamp       string `json:"timeStamp"`         // unix
	FunctionName    string `json:"functionName"`       // e.g. "transfer(address,uint256)"
	MethodID        string `json:"methodId"`            // e.g. "0xa9059cbb"
	Input           string `json:"input"`
}
