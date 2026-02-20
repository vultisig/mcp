// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package aavev3contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// WalletBalanceMetaData contains all meta data concerning the WalletBalance contract.
var WalletBalanceMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"users\",\"type\":\"address[]\"},{\"internalType\":\"address[]\",\"name\":\"tokens\",\"type\":\"address[]\"}],\"name\":\"batchBalanceOf\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"provider\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"}],\"name\":\"getUserWalletBalances\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
}

// WalletBalanceABI is the input ABI used to generate the binding from.
// Deprecated: Use WalletBalanceMetaData.ABI instead.
var WalletBalanceABI = WalletBalanceMetaData.ABI

// WalletBalance is an auto generated Go binding around an Ethereum contract.
type WalletBalance struct {
	WalletBalanceCaller     // Read-only binding to the contract
	WalletBalanceTransactor // Write-only binding to the contract
	WalletBalanceFilterer   // Log filterer for contract events
}

// WalletBalanceCaller is an auto generated read-only Go binding around an Ethereum contract.
type WalletBalanceCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletBalanceTransactor is an auto generated write-only Go binding around an Ethereum contract.
type WalletBalanceTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletBalanceFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type WalletBalanceFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// WalletBalanceSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type WalletBalanceSession struct {
	Contract     *WalletBalance    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// WalletBalanceCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type WalletBalanceCallerSession struct {
	Contract *WalletBalanceCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// WalletBalanceTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type WalletBalanceTransactorSession struct {
	Contract     *WalletBalanceTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// WalletBalanceRaw is an auto generated low-level Go binding around an Ethereum contract.
type WalletBalanceRaw struct {
	Contract *WalletBalance // Generic contract binding to access the raw methods on
}

// WalletBalanceCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type WalletBalanceCallerRaw struct {
	Contract *WalletBalanceCaller // Generic read-only contract binding to access the raw methods on
}

// WalletBalanceTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type WalletBalanceTransactorRaw struct {
	Contract *WalletBalanceTransactor // Generic write-only contract binding to access the raw methods on
}

// NewWalletBalance creates a new instance of WalletBalance, bound to a specific deployed contract.
func NewWalletBalance(address common.Address, backend bind.ContractBackend) (*WalletBalance, error) {
	contract, err := bindWalletBalance(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &WalletBalance{WalletBalanceCaller: WalletBalanceCaller{contract: contract}, WalletBalanceTransactor: WalletBalanceTransactor{contract: contract}, WalletBalanceFilterer: WalletBalanceFilterer{contract: contract}}, nil
}

// NewWalletBalanceCaller creates a new read-only instance of WalletBalance, bound to a specific deployed contract.
func NewWalletBalanceCaller(address common.Address, caller bind.ContractCaller) (*WalletBalanceCaller, error) {
	contract, err := bindWalletBalance(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &WalletBalanceCaller{contract: contract}, nil
}

// NewWalletBalanceTransactor creates a new write-only instance of WalletBalance, bound to a specific deployed contract.
func NewWalletBalanceTransactor(address common.Address, transactor bind.ContractTransactor) (*WalletBalanceTransactor, error) {
	contract, err := bindWalletBalance(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &WalletBalanceTransactor{contract: contract}, nil
}

// NewWalletBalanceFilterer creates a new log filterer instance of WalletBalance, bound to a specific deployed contract.
func NewWalletBalanceFilterer(address common.Address, filterer bind.ContractFilterer) (*WalletBalanceFilterer, error) {
	contract, err := bindWalletBalance(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &WalletBalanceFilterer{contract: contract}, nil
}

// bindWalletBalance binds a generic wrapper to an already deployed contract.
func bindWalletBalance(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := WalletBalanceMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_WalletBalance *WalletBalanceRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _WalletBalance.Contract.WalletBalanceCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_WalletBalance *WalletBalanceRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletBalance.Contract.WalletBalanceTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_WalletBalance *WalletBalanceRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _WalletBalance.Contract.WalletBalanceTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_WalletBalance *WalletBalanceCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _WalletBalance.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_WalletBalance *WalletBalanceTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletBalance.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_WalletBalance *WalletBalanceTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _WalletBalance.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address user, address token) view returns(uint256)
func (_WalletBalance *WalletBalanceCaller) BalanceOf(opts *bind.CallOpts, user common.Address, token common.Address) (*big.Int, error) {
	var out []interface{}
	err := _WalletBalance.contract.Call(opts, &out, "balanceOf", user, token)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address user, address token) view returns(uint256)
func (_WalletBalance *WalletBalanceSession) BalanceOf(user common.Address, token common.Address) (*big.Int, error) {
	return _WalletBalance.Contract.BalanceOf(&_WalletBalance.CallOpts, user, token)
}

// BalanceOf is a free data retrieval call binding the contract method 0xf7888aec.
//
// Solidity: function balanceOf(address user, address token) view returns(uint256)
func (_WalletBalance *WalletBalanceCallerSession) BalanceOf(user common.Address, token common.Address) (*big.Int, error) {
	return _WalletBalance.Contract.BalanceOf(&_WalletBalance.CallOpts, user, token)
}

// BatchBalanceOf is a free data retrieval call binding the contract method 0xb59b28ef.
//
// Solidity: function batchBalanceOf(address[] users, address[] tokens) view returns(uint256[])
func (_WalletBalance *WalletBalanceCaller) BatchBalanceOf(opts *bind.CallOpts, users []common.Address, tokens []common.Address) ([]*big.Int, error) {
	var out []interface{}
	err := _WalletBalance.contract.Call(opts, &out, "batchBalanceOf", users, tokens)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// BatchBalanceOf is a free data retrieval call binding the contract method 0xb59b28ef.
//
// Solidity: function batchBalanceOf(address[] users, address[] tokens) view returns(uint256[])
func (_WalletBalance *WalletBalanceSession) BatchBalanceOf(users []common.Address, tokens []common.Address) ([]*big.Int, error) {
	return _WalletBalance.Contract.BatchBalanceOf(&_WalletBalance.CallOpts, users, tokens)
}

// BatchBalanceOf is a free data retrieval call binding the contract method 0xb59b28ef.
//
// Solidity: function batchBalanceOf(address[] users, address[] tokens) view returns(uint256[])
func (_WalletBalance *WalletBalanceCallerSession) BatchBalanceOf(users []common.Address, tokens []common.Address) ([]*big.Int, error) {
	return _WalletBalance.Contract.BatchBalanceOf(&_WalletBalance.CallOpts, users, tokens)
}

// GetUserWalletBalances is a free data retrieval call binding the contract method 0x02405343.
//
// Solidity: function getUserWalletBalances(address provider, address user) view returns(address[], uint256[])
func (_WalletBalance *WalletBalanceCaller) GetUserWalletBalances(opts *bind.CallOpts, provider common.Address, user common.Address) ([]common.Address, []*big.Int, error) {
	var out []interface{}
	err := _WalletBalance.contract.Call(opts, &out, "getUserWalletBalances", provider, user)

	if err != nil {
		return *new([]common.Address), *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	out1 := *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return out0, out1, err

}

// GetUserWalletBalances is a free data retrieval call binding the contract method 0x02405343.
//
// Solidity: function getUserWalletBalances(address provider, address user) view returns(address[], uint256[])
func (_WalletBalance *WalletBalanceSession) GetUserWalletBalances(provider common.Address, user common.Address) ([]common.Address, []*big.Int, error) {
	return _WalletBalance.Contract.GetUserWalletBalances(&_WalletBalance.CallOpts, provider, user)
}

// GetUserWalletBalances is a free data retrieval call binding the contract method 0x02405343.
//
// Solidity: function getUserWalletBalances(address provider, address user) view returns(address[], uint256[])
func (_WalletBalance *WalletBalanceCallerSession) GetUserWalletBalances(provider common.Address, user common.Address) ([]common.Address, []*big.Int, error) {
	return _WalletBalance.Contract.GetUserWalletBalances(&_WalletBalance.CallOpts, provider, user)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_WalletBalance *WalletBalanceTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _WalletBalance.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_WalletBalance *WalletBalanceSession) Receive() (*types.Transaction, error) {
	return _WalletBalance.Contract.Receive(&_WalletBalance.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_WalletBalance *WalletBalanceTransactorSession) Receive() (*types.Transaction, error) {
	return _WalletBalance.Contract.Receive(&_WalletBalance.TransactOpts)
}
