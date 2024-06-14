package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-resty/resty/v2"
)

type Abi []interface{}

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
	ID      int             `json:"id"`
}
type JSONRPCResponse1 struct {
	Result interface{} `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type GasPriceResponse struct {
	Result string `json:"result"`
}

type BlacklistAddress struct {
	Address string `json:"address"`
	Comment string `json:"comment"`
	Date    string `json:"date"`
}

func ExtractFunctionsFromAbi(abiJson string) map[string]int {
	var abi Abi
	err := json.Unmarshal([]byte(abiJson), &abi)
	if err != nil {
		fmt.Println("Error parsing ABI:", err)
		return nil
	}

	functionNames := make(map[string]int)
	for _, item := range abi {
		if fn, ok := item.(map[string]interface{}); ok {
			name, ok := fn["name"].(string)
			if ok {
				functionNames[name] = 0
			}
		}
	}

	return functionNames
}

func getTransactionHashesForBlock(blockNumber string) ([]string, error) {
	providerUrl := os.Getenv("NODE_URL")

	// Define the JSON payload
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockByNumber",
		"params":  []interface{}{blockNumber, false}, // Set to false to get transaction hashes
		"id":      1,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return nil, err
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", providerUrl, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return nil, err
	}

	// Set the Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// Send the request using the default HTTP client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// Unmarshal the response into a map
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return nil, err
	}

	newContent := body
	errr := os.WriteFile("block.json", []byte(newContent), 0644)
	if errr != nil {
		log.Fatal(errr)
	}

	// Extract transaction hashes from the response
	transactions := []string{}
	if result, ok := response["result"].(map[string]interface{}); ok {

		if txs, ok := result["transactions"].([]interface{}); ok {

			for _, tx := range txs {
				if txHash, ok := tx.(string); ok {
					transactions = append(transactions, txHash)
				}
			}
		}
	}

	return transactions, nil
}

func GetAllFunctionCalls(smartContractAddress string, smartContractAbi string, blockNumber string) (map[string]int, error) {

	// initialize empty map to store method calls
	methodCounter := ExtractFunctionsFromAbi(smartContractAbi)

	// Returns only transaction hashes
	transactionHashes, err := getTransactionHashesForBlock(blockNumber)
	if err != nil {
		log.Fatalf("Failed to get transaction hashes: %v", err)
	}

	myABI, err := abi.JSON(strings.NewReader(smartContractAbi))
	if err != nil {
		log.Fatal(err)
	}

	providerUrl := os.Getenv("NODE_URL")

	client, err := ethclient.Dial(providerUrl)
	if err != nil {
		log.Fatal(err)
	}
	for _, tx := range transactionHashes {
		txHash := common.HexToHash(tx)

		// fetch each transaction by hash in the block
		tx, _, err := client.TransactionByHash(context.Background(), txHash)

		if err != nil {
			log.Fatal(err)
		}
		// GetTransactionBaseInfo(tx)
		if tx.To().Hex() == smartContractAddress {

			// decode tx data from the abi provided and increment the method counter
			methodCounter = DecodeTransactionInputData(&myABI, tx.Data(), methodCounter)
		}
	}

	// time.Sleep(15 * time.Second)
	return methodCounter, err
}

func DecodeTransactionInputData(contractABI *abi.ABI, data []byte, methodCounter map[string]int) map[string]int {
	methodSigData := data[:4]
	method, _ := contractABI.MethodById(methodSigData)
	methodCounter[method.Name]++
	return methodCounter
}

func DecodeTransactionLogs(receipt *types.Receipt, contractABI *abi.ABI, eventCounter map[string]int) map[string]int {
	for _, vLog := range receipt.Logs {
		event, err := contractABI.EventByID(vLog.Topics[0])
		if err != nil {
			log.Fatal(err)
		}

		eventCounter[event.Name]++

	}
	return eventCounter
}

func GetTransactionReceipt(client *ethclient.Client, txHash common.Hash) *types.Receipt {
	receipt, err := client.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		log.Fatal(err)
	}
	return receipt
}

func GetAllEvents(smartContractAddress string, smartContractAbi string, blockNumber string) (map[string]int, error) {

	eventCounter := ExtractFunctionsFromAbi(smartContractAbi)

	transactionHashes, err := getTransactionHashesForBlock(blockNumber)
	if err != nil {
		log.Fatalf("Failed to get transaction hashes: %v", err)
	}

	myABI, err := abi.JSON(strings.NewReader(smartContractAbi))
	if err != nil {
		log.Fatal(err)
	}

	providerUrl := os.Getenv("NODE_URL")

	client, err := ethclient.Dial(providerUrl)
	if err != nil {
		log.Fatal(err)
	}
	for _, tx := range transactionHashes {
		txHash := common.HexToHash(tx)
		tx, _, err := client.TransactionByHash(context.Background(), txHash)
		if err != nil {
			log.Fatal(err)
		}
		// GetTransactionBaseInfo(tx)
		// DecodeTransactionInputData(&myABI, tx.Data(), methodCounter)
		if tx.To().Hex() == smartContractAddress {
			receipt1 := GetTransactionReceipt(client, txHash)
			eventCounter = DecodeTransactionLogs(receipt1, &myABI, eventCounter)
		}
	}

	// time.Sleep(15 * time.Second)
	return eventCounter, err
}

func GetValueOfTransactionsForSmartcontract(smartContractAddress string, smartContractAbi string, blockNumber string) (string, error) {

	var txValue string

	transactionHashes, err := getTransactionHashesForBlock(blockNumber)
	if err != nil {
		log.Fatalf("Failed to get transaction hashes: %v", err)
	}

	providerUrl := os.Getenv("NODE_URL")

	client, err := ethclient.Dial(providerUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Example threshold value
	valueLimit, _ := new(big.Int).SetString("10000000000", 10)

	for _, tx := range transactionHashes {
		txHash := common.HexToHash(tx)
		tx, _, err := client.TransactionByHash(context.Background(), txHash)
		if err != nil {
			log.Fatal(err)
		}
		if tx.To().Hex() == smartContractAddress {
			fmt.Print(tx.Value().String())
			txValue = tx.Value().String()
			if tx.Value().Cmp(valueLimit) > 0 {
				fmt.Printf("Transaction value %s is greater than the Value Limit. For Below Tx Hash\n %s", tx.Value().String(), txHash)
			}
		}
	}

	return txValue, err
}

func getTxSender(tx *types.Transaction) (common.Address, error) {
	chainID := big.NewInt(31337)
	signer := types.NewLondonSigner(chainID)
	sender, err := signer.Sender(tx)
	if err != nil {
		return common.Address{}, err
	}

	return sender, nil
}
func CheckSmartcontractTxAddresses(smartContractAddress string, smartContractAbi string, blockNumber string) {

	transactionHashes, err := getTransactionHashesForBlock(blockNumber)
	if err != nil {
		log.Fatalf("Failed to get transaction hashes: %v", err)
	}

	providerUrl := os.Getenv("NODE_URL")

	client, err := ethclient.Dial(providerUrl)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	blacklistAddressesStr, err := os.Open("resources/blacklistAddresses.json")
	if err != nil {
		log.Fatal("Failed to retrieve transaction:", err)
	}

	byteValue, _ := ioutil.ReadAll(blacklistAddressesStr)

	var blacklistAddresses []BlacklistAddress
	err = json.Unmarshal(byteValue, &blacklistAddresses)
	if err != nil {
		return
	}

	// fmt.Print("🚀 ~ funcCheckSmartcontractTxAddresses ~ blacklistAddresses:", blacklistAddresses)

	for _, txHashStr := range transactionHashes {

		txHash := common.HexToHash(txHashStr)
		tx, _, err := client.TransactionByHash(context.Background(), common.HexToHash((txHash.String())))
		if err != nil {
			log.Fatal("Failed to retrieve transaction:", err)
		}

		// Get the sender
		sender, err := getTxSender(tx)
		if err != nil {
			log.Fatal("Failed to get transaction sender:", err)
		}
		// fmt.Printf("Transaction was sent by: %s\n", sender.Hex())
		// fmt.Printf("Blacklist Address : %s\n", blacklistAddresses.Address)
		for _, addr := range blacklistAddresses {
			if sender.Hex() == addr.Address {
				fmt.Print("🚀🚀🚀🚀🚀   ALERT !!!!, a transaction found from blacklisted addreses.🚀🚀🚀🚀🚀 \n")
				fmt.Printf("🚀🚀🚀🚀🚀   Tx Hash : %s. \n\n\n", txHash)
				return
			}
		}

	}
}

func GetAllTransactions(smartContractAddress string, blockNumber string) ([]types.Receipt, error) {

	var transactions []types.Receipt
	var failedTx int
	var successfulTx int
	transactionHashes, err := getTransactionHashesForBlock(blockNumber)
	if err != nil {
		log.Fatalf("Failed to get transaction hashes: %v", err)
	}
	if err != nil {
		log.Fatal(err)
	}

	providerUrl := os.Getenv("NODE_URL")

	client, err := ethclient.Dial(providerUrl)
	if err != nil {
		log.Fatal(err)
	}
	for _, tx := range transactionHashes {
		txHash := common.HexToHash(tx)
		tx, _, err := client.TransactionByHash(context.Background(), txHash)
		if err != nil {
			log.Fatal(err)
		}
		// GetTransactionBaseInfo(tx)
		if tx.To().Hex() == smartContractAddress {
			receipt1 := GetTransactionReceipt(client, txHash)
			fmt.Printf("🚀 receipt.status : %v\n\n", receipt1.Status)
			if receipt1.Status == 0 {
				failedTx += 1
			} else if receipt1.Status == 1 {
				successfulTx += 1
			}
			transactions = append(transactions, *receipt1)
		}
	}
	fmt.Printf("🚀-------------Failed Tx Count : %v----------- 🚀\n🚀---------------Successful Tx Count : %v --------------🚀\n\n", failedTx, successfulTx)

	// time.Sleep(15 * time.Second)
	return transactions, err
}

type (
	RawABIResponse struct {
		Status  *string `json:"status"`
		Message *string `json:"message"`
		Result  *string `json:"result"`
	}
)

func GetContractRawABI(address string) (*RawABIResponse, error) {

	apiKey := os.Getenv("ETHERSCAN_API_KEY")
	client := resty.New()
	rawABIResponse := &RawABIResponse{}
	resp, err := client.R().
		SetQueryParams(map[string]string{
			"module":  "contract",
			"action":  "getabi",
			"address": address,
			"apikey":  apiKey,
		}).
		SetResult(rawABIResponse).
		Get("https://api.etherscan.io/api")

	if err != nil {
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf(fmt.Sprintf("Get contract raw abi failed: %s", resp))
	}
	if *rawABIResponse.Status != "1" {
		return nil, fmt.Errorf(fmt.Sprintf("Get contract raw abi failed: %s", *rawABIResponse.Result))
	}

	return rawABIResponse, nil
}

func GetSmartContractABI(contractAddress, etherscanAPIKey string) *abi.ABI {

	rawABIResponse, err := GetContractRawABI(contractAddress)
	if err != nil {
		log.Fatal(err)
	}

	contractABI, err := abi.JSON(strings.NewReader(*rawABIResponse.Result))
	if err != nil {
		log.Fatal(err)
	}
	return &contractABI
}

func sendRPCRequest(url string, method string, params interface{}) (*JSONRPCResponse, error) {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result JSONRPCResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("RPC Error: %d %s", result.Error.Code, result.Error.Message)
	}

	return &result, nil
}

func monitorBlock() (string, string, string, string, string, error) {
	providerUrl := os.Getenv("NODE_URL")
	result, err := sendRPCRequest(providerUrl, "eth_blockNumber", []interface{}{})
	if err != nil {
		log.Fatalf("Failed to retrieve block number: %v", err)
	}
	// fmt.Print("🚀 ~ funcmonitorBlock ~ result: eth_blockNumber", result)

	var blockNumber string
	err = json.Unmarshal(result.Result, &blockNumber)
	if err != nil {
		log.Fatalf("Failed to parse block number: %v", err)
	}

	result, err = sendRPCRequest(providerUrl, "eth_getBlockByNumber", []interface{}{blockNumber, false})
	if err != nil {
		log.Fatalf("Failed to retrieve block: %v", err)
	}
	// fmt.Print("🚀 ~ funcmonitorBlock ~ result eth_getBlockByNumber:", result)

	var block struct {
		Timestamp string `json:"timestamp"`
		GasLimit  string `json:"gaslimit"`
		GasUsed   string `json:"gasused"`
		Size      string `json:"size"`
	}
	err = json.Unmarshal(result.Result, &block)
	if err != nil {
		log.Fatalf("Failed to parse block: %v", err)
	}

	return blockNumber, block.Timestamp, block.GasLimit, block.GasUsed, block.Size, nil
}

func monitorTransactionPoolStatus() (string, string, error) {
	providerUrl := os.Getenv("NODE_URL")

	result, err := sendRPCRequest(providerUrl, "txpool_status", []interface{}{})
	if err != nil {
		log.Fatalf("Failed to get monitor transaction pool status: %v", err)
	}

	var pool struct {
		PendingTx string `json:"pending"`
		QueuedTx  string `json:"queued"`
	}
	err = json.Unmarshal(result.Result, &pool)
	if err != nil {
		log.Fatalf("Failed to parse pool: %v", err)
	}
	return pool.PendingTx, pool.QueuedTx, nil
}

func FetchCurrentGasPrice() (string, error) {
	providerUrl := os.Getenv("NODE_URL")

	method := "eth_gasPrice"
	params := []interface{}{}

	response, err := sendRPCRequest(providerUrl, method, params)
	if err != nil {
		return "", fmt.Errorf("failed to send RPC request: %w", err)
	}

	gasPrice := string(response.Result)

	return gasPrice, nil
}

func BlockchainMonitoring() {
	for {
		BlockHeight, Timestamp, GasLimit, GasUsed, Size, err := monitorBlock()
		fmt.Printf("\n🚀-------------Timestamp--------------🚀  \n%v\n\n", Timestamp)
		fmt.Printf("🚀-------------GasLimit--------------🚀  \n%v\n\n", GasLimit)
		fmt.Printf("🚀-------------GasUsed--------------🚀  \n%v\n\n", GasUsed)
		fmt.Printf("🚀-------------Block Size--------------🚀  \n%v\n\n", Size)
		fmt.Printf("🚀-------------BlockHeight--------------🚀  \n%v\n\n", BlockHeight)
		if err != nil {
			log.Fatalf("Failed to parse block: %v", err)
		}
		GasPrice, err := FetchCurrentGasPrice()
		if err != nil {
			log.Fatalf("Failed to parse block: %v", err)
		}
		fmt.Printf("🚀-------------GasPrice--------------🚀  \n%v\n\n", GasPrice)
		AllPendingTransactions, AllQueuedTransactions, err := monitorTransactionPoolStatus()
		fmt.Printf("🚀-------------AllPendingTransactions--------------🚀  \n%v\n\n", AllPendingTransactions)
		fmt.Printf("🚀-------------AllQueuedTransactions--------------🚀  \n%v\n\n", AllQueuedTransactions)
		if err != nil {
			log.Fatalf("Failed to parse block: %v", err)
		}

		time.Sleep(15 * time.Second) // Sleep interval is set to 15 because usually 12 to 13 seconds is the block mining time

	}
}

func SmartContractMonitoring(smartContractAddress string, blockNumber string, smartContractABI string) {

	allMethods, err := GetAllFunctionCalls(smartContractAddress, smartContractABI, blockNumber)
	if err != nil {
		fmt.Printf("Error while fetching function calls : %s\n", err)
	}
	fmt.Printf("🚀------------- All methods --------------🚀  \n%v\n\n", allMethods)

	allEvents, err := GetAllEvents(smartContractAddress, smartContractABI, blockNumber)
	if err != nil {
		fmt.Printf("Error while fetching events : %s\n", err)
	}
	fmt.Printf("🚀------------- All Events --------------🚀  \n%v\n\n", allEvents)

	allTransactions, err := GetAllTransactions(smartContractAddress, blockNumber)
	if err != nil {
		fmt.Printf("Error while fetching all transactions : %s\n", err)
	}
	fmt.Printf("🚀------------- All Transactions --------------🚀  \n%v\n\n", allTransactions)
	fmt.Printf("🚀-------------Transactions length : %v-------------🚀\n\n", len(allTransactions))

	txValues, err := GetValueOfTransactionsForSmartcontract(smartContractAddress, smartContractABI, blockNumber)
	if err != nil {
		fmt.Printf("Error while fetching all transactions : %s\n", err)
	}
	fmt.Print("🚀 ~ funcSmartContractMonitoring ~ txValues:", txValues)
	CheckSmartcontractTxAddresses(smartContractAddress, smartContractABI, blockNumber)

}

func main() {

	// Change the block number with the one to be queried
	blockNumber := "0xB"

	// Deployed smartcontract address on anvil local blockchain
	smartContractAddress := "0x2279B7A0a67DB372996a5FaB50D91eAA73d2eBe6" //"0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512"

	//  This is a sample smartcontract address deployed on mainnet and verified (As we can extract abi of any verified smart contract only )
	verifiedSmartContractAddress := "0x893411580e590D62dDBca8a703d61Cc4A8c7b2b9"

	// To Extract the ABI we need to have an ETHERSCAN_API_KEY to get information from etherscan
	ETHERSCAN_API_KEY := os.Getenv("ETHERSCAN_API_KEY")

	// This is our smartcontract ABI which is stored in `example/abi.json`, you may change this with your one
	contractABIFromFile, _ := os.ReadFile("resources/abi.json")
	contractABIString := string(contractABIFromFile)

	fmt.Printf("                               🚀*****************************************************************************************🚀\n\n")
	fmt.Printf("                               🚀                               SMARTCONTRACT MONITORING                                  🚀\n\n")
	fmt.Printf("                               🚀*****************************************************************************************🚀\n\n\n")

	SmartContractMonitoring(smartContractAddress, blockNumber, contractABIString)

	abi := GetSmartContractABI(verifiedSmartContractAddress, ETHERSCAN_API_KEY)
	fmt.Printf("🚀-------------ABI for Verified Mainnet SC: %s : ---------🚀\n%v\n\n", smartContractAddress, abi)
	fmt.Printf("                               🚀*****************************************************************************************🚀\n\n")
	fmt.Printf("                               🚀                               BLOCKCHAIN MONITORING                                     🚀\n\n")
	fmt.Printf("                               🚀*****************************************************************************************🚀\n\n\n")

	BlockchainMonitoring()

}
