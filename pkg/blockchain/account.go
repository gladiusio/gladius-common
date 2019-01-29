package blockchain

import (
	"errors"
	"github.com/gladiusio/gladius-common/pkg/utils"
	"io/ioutil"
	"math"
	"path/filepath"
	"strings"

	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
)

// GladiusAccountManager is a type that allows the user to create a keystore file,
// create an in it, and preform actions on the first account stored.
type GladiusAccountManager struct {
	keystore *keystore.KeyStore
}

// NewGladiusAccountManager creates a new gladius account manager
func NewGladiusAccountManager() *GladiusAccountManager {
	base, err := utils.GetGladiusBase()
	if err != nil {
		return nil
	}

	var walletDir = viper.GetString("Wallet.Directory")
	
	ks := keystore.NewKeyStore(
		walletDir,
		keystore.LightScryptN,
		keystore.LightScryptP)

	return &GladiusAccountManager{keystore: ks}
}

// Determines if account is unlocked by signing a blank hash
func (ga GladiusAccountManager) Unlocked() bool {
	if !ga.HasAccount() {
		return false
	}

	account, err := ga.GetAccount()
	if err != nil {
		return false
	}

	_, err = ga.Keystore().SignHash(*account, []byte("00000000000000000000000000000000"))
	if err != nil {
		return false
	}

	return true
}

// Checks if AccountManager has an account
func (ga GladiusAccountManager) HasAccount() bool {
	return len(ga.keystore.Accounts()) > 0
}

// Keystore gets the keystore associated with the account manager
func (ga GladiusAccountManager) Keystore() *keystore.KeyStore {
	return ga.keystore
}

//UnlockAccount Unlocks the account
func (ga GladiusAccountManager) UnlockAccount(passphrase string) (bool, error) {
	account, err := ga.GetAccount()
	if err != nil {
		return false, err
	}

	err = ga.Keystore().Unlock(*account, passphrase)
	if err == nil {
		return true, nil
	}

	return false, err
}

// CreateAccount will create an account if there isn't one already
func (ga GladiusAccountManager) CreateAccount(passphrase string) (accounts.Account, error) {
	ks := ga.Keystore()
	if len(ga.Keystore().Accounts()) < 1 {
		return ks.NewAccount(passphrase)
	}

	return accounts.Account{}, errors.New("gladius account already exists")

}

// GetAccountAddress gets the account address
func (ga GladiusAccountManager) GetAccountAddress() (*common.Address, error) {
	account, err := ga.GetAccount()
	if err != nil {
		return nil, err
	}

	return &account.Address, nil
}

// GetAccount gets the actual account type
func (ga GladiusAccountManager) GetAccount() (*accounts.Account, error) {
	store := ga.Keystore()
	if len(store.Accounts()) < 1 {
		return nil, errors.New("account retrieval error, no existing accounts found")
	}

	account := store.Accounts()[0]

	return &account, nil
}

type BalanceType int32

const (
	ETH BalanceType = 0
	GLA BalanceType = 1
)

type PrettyBalance struct {
	Value 	float64 `json:"value"`
	Symbol 	string `json:"symbol"`
	Name 	string `json:"name"`
}

type Balance struct {
	RawValue  		uint64 			`json:"value"`
	BalanceType		BalanceType		`json:"balanceType"`
	PrettyBalance  	*PrettyBalance 	`json:"formattedBalance"`
	UsdBalance  	*PrettyBalance 	`json:"usdBalance,omitempty"`
}

func GetAccountBalance(address common.Address, symbol BalanceType) (Balance, error) {
	var resp *http.Response
	var err error
	var symbolString string
	var symbolName string

	glaTokenAddress := "0x71d01db8d6a2fbea7f8d434599c237980c234e4c"

	switch symbol {
	case ETH:
		resp, err = http.Get("https://api.etherscan.io/api?module=account&action=balance&address=" + address.String() + "&tag=latest&apikey=3VRW685YYESSYIFVND3DVN9ZNF4BTT1GB8")
		symbolString = "ETH"
		symbolName = "Ethereum"
		break
	case GLA:
		resp, err = http.Get("https://api.etherscan.io/api?module=account&action=tokenbalance&contractaddress=" + glaTokenAddress + "&address=" + address.String() + "&apikey=3VRW685YYESSYIFVND3DVN9ZNF4BTT1GB8")
		symbolString = "GLA"
		symbolName = "Gladius"
		break
	}

	if err != nil {
		return Balance{}, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	type etherscanResult struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  string `json:"result"`
	}

	result := etherscanResult{}
	json.Unmarshal(body, &result)

	balanceInt, err := strconv.ParseUint(result.Result, 10, 64)
	if err != nil {
		return Balance{}, err
	}

	floatMoved := float64(1.0)
	floatBalance := math.Round(floatMoved * 100) / 100

	switch symbol {
	case ETH:
		floatMoved = float64(balanceInt) / 10000000000000000000
		floatBalance = math.Round(floatMoved * 1000) / 100
		break
	case GLA:
		floatMoved = float64(balanceInt) / 100000000
		floatBalance = math.Round(floatMoved * 100) / 100
		break
	}

	//stringBalance := strconv.FormatFloat(floatBalance, 'f', 2, 64)
	balance := Balance{
		RawValue: balanceInt, 
		BalanceType: symbol,
		PrettyBalance: &PrettyBalance{
			Value: floatBalance,
			Symbol: symbolString,
			Name: symbolName,
		},
	}

	return balance, nil
}

type TransactionOptions struct {
	Filters *TransactionFilter `json:"filters"`
}

type TransactionFilter struct {
	EthTransfer bool `json:"eth_transfer"`
}

type EtherscanTransactionsResponse struct {
	Status       string                 `json:"status"`
	Message      string                 `json:"message"`
	Transactions []EtherscanTransaction `json:"result"`
}

type EtherscanTransaction struct {
	BlockNumber       string `json:"blockNumber"`
	TimeStamp         string `json:"timeStamp"`
	Hash              string `json:"hash"`
	Nonce             string `json:"nonce"`
	BlockHash         string `json:"blockHash"`
	TransactionIndex  string `json:"transactionIndex"`
	From              string `json:"from"`
	To                string `json:"to"`
	Value             string `json:"value"`
	Gas               string `json:"gas"`
	GasPrice          string `json:"gasPrice"`
	IsError           string `json:"isError"`
	TxReceiptStatus   string `json:"txreceipt_status"`
	Input             string `json:"input"`
	ContractAddress   string `json:"contractAddress"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	GasUsed           string `json:"gasUsed"`
	Confirmations     string `json:"confirmations"`
}

func getTransactions(url string) (EtherscanTransactionsResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return EtherscanTransactionsResponse{}, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	transactionsResponse := EtherscanTransactionsResponse{}
	json.Unmarshal(body, &transactionsResponse)

	return transactionsResponse, nil
}

func GetEthereumAccountTransactions(address common.Address) (EtherscanTransactionsResponse, error) {
	return getTransactions("https://api.etherscan.io/api?module=account&action=txlist&address=" + address.String() + "&startblock=0&endblock=latest&sort=asc&apikey=3VRW685YYESSYIFVND3DVN9ZNF4BTT1GB8")
}

func GetGladiusAccountTransactions(address common.Address) (EtherscanTransactionsResponse, error) {
	return getTransactions("https://api.etherscan.io/api?module=account&action=tokentx&address=" + address.String() + "&startblock=0&endblock=latest&sort=asc&apikey=3VRW685YYESSYIFVND3DVN9ZNF4BTT1GB8")
}

// GetAuth gets the authenticator for the go bindings of our smart contracts
func (ga GladiusAccountManager) GetAuth(passphrase string) (*bind.TransactOpts, error) {
	account, err := ga.GetAccount()
	if err != nil {
		return nil, err
	}
	// Create a JSON blob with the same passphrase used to decrypt it
	key, err := ga.Keystore().Export(*account, passphrase, passphrase)
	if err != nil {
		return nil, err
	}

	// Create a transactor from the key file
	auth, err := bind.NewTransactor(strings.NewReader(string(key)), passphrase)
	if err != nil {
		return nil, err
	}

	return auth, nil
}
