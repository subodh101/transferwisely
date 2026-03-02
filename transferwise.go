package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// transfer-wise api paths
const (
	transfersAPIPath      = "v1/transfers"
	quotesAPIPath         = "v2/quotes"
	liveRateAPIPath       = "v1/rates"
	cancelTransferAPIPath = "v1/transfers/{transferId}/cancel"
)

// transfer-wise hosts
const (
	hostProduction = "api.transferwise.com"
	hostSandbox    = "api.sandbox.transferwise.tech"
)

// fallback values for optional env variables
const (
	fallbackInterval = "1"
	fallbackMargin   = "0"
)

// other constants
const PRODUCTION = "production"
const SANDBOX = "sandbox"

// error messages
const ErrNoCurrentTransferFound = "error: no current transfer found, please create a transfer before proceeding"
const ErrEnvVarMissingOrInvalid = "error: make sure env variables ENV, API_TOKEN are both provided and are valid"

// env vars
var envVar = getEnv("ENV", "")
var hostVar = getHost(envVar)
var apiTokenVar = getEnv("API_TOKEN", "")
var marginVar = getEnv("MARGIN", fallbackMargin)
var intervalVar = getEnv("INTERVAL", fallbackInterval)

// HTTPClient interface
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	Client HTTPClient
)

func init() {
	Client = &http.Client{Timeout: 10 * time.Second}
}

func getTransfer() ([]Transfer, error) {
	// Filter specifically for transfers waiting for funds
	params := url.Values{
		"limit":  {"2"},
		"offset": {"0"},
		"status": {"incoming_payment_waiting"},
	}
	url := &url.URL{RawQuery: params.Encode(), Host: hostVar, Scheme: "https", Path: transfersAPIPath}

	response, code, err := callExternalAPI(http.MethodGet, url.String(), nil)
	if err != nil || code != http.StatusOK {
		return nil, fmt.Errorf("error GET transfer list API: %v : %v", code, err)
	}

	var transfersList []Transfer
	err = mapstructure.Decode(response, &transfersList)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if len(transfersList) == 0 {
		return nil, fmt.Errorf(ErrNoCurrentTransferFound)
	}

	// Enrich each transfer with Quote details (Profile and Amounts)
	for i := range transfersList {
		quoteDetail, err := getDetailByQuoteId(transfersList[i].QuoteUuid)
		if err != nil {
			log.Printf("Warning: could not get quote details for transfer %d: %v", transfersList[i].Id, err)
			continue
		}
		transfersList[i].SourceAmount = quoteDetail.SourceAmount
		transfersList[i].TargetAmount = quoteDetail.TargetAmount
		transfersList[i].Profile = quoteDetail.Profile
	}

	return transfersList, nil
}

func checkAndProcess() {
	if envVar == "" || apiTokenVar == "" {
		log.Println(ErrEnvVarMissingOrInvalid)
		return
	}

	// 1. Get all transfers
	transfers, err := getTransfer()
	if err != nil {
		log.Println(err)
		return
	}

	if len(transfers) == 0 {
		log.Println(ErrNoCurrentTransferFound)
		return
	}

	// 2. Create a map to cache rates for this cycle
	// Key format: "SOURCE-TARGET" (e.g., "JPY-INR")
	rateCache := make(map[string]float64)
	margin, _ := strconv.ParseFloat(marginVar, 64)

	for _, transfer := range transfers {
		// This now works because we added 'Status' to the struct above
		if transfer.Status != "outgoing_payment_sent" &&
			transfer.Status != "booked" &&
			transfer.Status != "incoming_payment_waiting" {
			continue
		}

		currencyPair := fmt.Sprintf("%s-%s", transfer.SourceCurrency, transfer.TargetCurrency)

		currentRate, exists := rateCache[currencyPair]
		if !exists {
			liveRate, err := getLiveRate(transfer.SourceCurrency, transfer.TargetCurrency)
			if err != nil {
				log.Printf("Error getting live rate for %s: %v", currencyPair, err)
				continue
			}
			// FIX: getLiveRate returns a float64 directly, not a struct
			currentRate = liveRate
			rateCache[currencyPair] = currentRate
			log.Printf("Fetched fresh rate for %s: %f", currencyPair, currentRate)
		}

		// 4. Perform the comparison logic
		if currentRate > (transfer.Rate + margin) {
			log.Printf("!!! ACTION REQUIRED | Transfer ID: %d | Live Rate: %f > Booked Rate: %f",
				transfer.Id, currentRate, transfer.Rate)
			// Placeholder for your cancel/re-create logic
		} else {
			log.Printf("|| NO ACTION NEEDED | Transfer ID: %d | Live Rate: %f || Booked Rate: %f ||",
				transfer.Id, currentRate, transfer.Rate)
		}
	}
}

func getLiveRate(source string, target string) (float64, error) {
	params := url.Values{"source": {source}, "target": {target}}
	url := &url.URL{RawQuery: params.Encode(), Host: hostVar, Scheme: "https", Path: liveRateAPIPath}

	response, code, err := callExternalAPI(http.MethodGet, url.String(), nil)
	if err != nil || code != http.StatusOK {
		return 0, fmt.Errorf("error GET live rate API: %v : %v", code, err)
	}

	var liveRate []LiveRate
	err = mapstructure.Decode(response, &liveRate)
	if err != nil {
		return 0, fmt.Errorf("error decoding live rate response: %v", err)
	}

	return liveRate[0].Rate, nil
}

func createTransfer(oldTransfer Transfer) (Transfer, error) {
	quoteId, err := generateQuote(oldTransfer.SourceCurrency, oldTransfer.TargetCurrency, oldTransfer.TargetAmount, oldTransfer.Profile)
	if err != nil {
		return Transfer{}, fmt.Errorf("createTransfer: %v", err)
	}

	createRequest := CreateTransferRequest{
		TargetAccount:         oldTransfer.TargetAccount,
		QuoteUuid:             quoteId,
		CustomerTransactionId: uuid.New().String(),
		Details:               oldTransfer.Details,
	}
	request, _ := json.Marshal(createRequest)

	url := &url.URL{Host: hostVar, Scheme: "https", Path: transfersAPIPath}
	response, code, err := callExternalAPI(http.MethodPost, url.String(), request)
	if err != nil || code != http.StatusOK {
		return Transfer{}, fmt.Errorf("error POST create transfer API: %v : %v", code, err)
	}

	var newTransfer Transfer
	err = mapstructure.Decode(response, &newTransfer)
	if err != nil {
		return Transfer{}, fmt.Errorf("error decoding response: %v", err)
	}

	cancelResult, err := cancelTransfer(oldTransfer.Id)
	if !cancelResult || err != nil {
		log.Println("Error deleting old transfer")
	}

	// For the log message, we can now show how much Source (JPY) was saved
	newTransfer.TargetAmount = oldTransfer.TargetAmount
	return newTransfer, nil
}

func cancelTransfer(transferId uint64) (bool, error) {
	path := strings.Replace(cancelTransferAPIPath, "{transferId}", strconv.FormatUint(transferId, 10), 1)

	url := &url.URL{Host: hostVar, Scheme: "https", Path: path}
	_, code, err := callExternalAPI(http.MethodPut, url.String(), nil)
	if err != nil || code != http.StatusOK {
		return false, fmt.Errorf("error PUT cancel transfer API: %v : %v", code, err)
	}

	return true, nil
}

func generateQuote(source string, target string, targetAmount float64, profile uint64) (string, error) {
	quoteRequest := CreateQuoteRequest{
		SourceCurrency: source,
		TargetCurrency: target,
		TargetAmount:   targetAmount,
		Profile:        profile,
	}

	request, _ := json.Marshal(quoteRequest)

	url := &url.URL{Host: hostVar, Scheme: "https", Path: quotesAPIPath}
	response, code, err := callExternalAPI(http.MethodPost, url.String(), request)
	if err != nil || code != http.StatusOK {
		return "", fmt.Errorf("error POST quote API: %v : %v", code, err)
	}

	var quote QuoteDetail
	err = mapstructure.Decode(response, &quote)
	if err != nil {
		return "", fmt.Errorf("error decoding quote response: %v", err)
	}

	return quote.Id, nil
}

func getDetailByQuoteId(quoteUuid string) (QuoteDetail, error) {
	path := quotesAPIPath + "/" + quoteUuid
	url := &url.URL{Host: hostVar, Scheme: "https", Path: path}

	response, code, err := callExternalAPI(http.MethodGet, url.String(), nil)
	if err != nil || code != http.StatusOK {
		return QuoteDetail{}, fmt.Errorf("error GET quote detail API: %v : %v", code, err)
	}

	var quoteDetail QuoteDetail
	err = mapstructure.Decode(response, &quoteDetail)
	if err != nil {
		return QuoteDetail{}, fmt.Errorf("error decoding to quote detail: %v : %v", code, err)
	}

	return quoteDetail, nil
}

func callExternalAPI(method string, url string, reqBody []byte) (response interface{}, code int, err error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error creating external api request: %v", err)
	}
	req.Header.Add("Authorization", "Bearer "+apiTokenVar)
	req.Header.Add("Content-Type", "application/json")

	res, err := Client.Do(req)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error calling external api: %v", err)
	}
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("error decoding json response: %v", err)
	}
	code = res.StatusCode
	_ = res.Body.Close()

	return
}

func getHost(envVar string) string {
	switch strings.ToLower(envVar) {
	case SANDBOX:
		return hostSandbox
	case PRODUCTION:
		return hostProduction
	default:
		return ""
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

type Transfer struct {
	Id             uint64          `json:"id"`
	Status         string          `json:"status"`
	Profile        uint64          `json:"profile"`
	TargetAccount  uint64          `json:"targetAccount"`
	SourceAmount   float64         `json:"sourceAmount"`
	TargetAmount   float64         `json:"targetAmount"`
	Rate           float64         `json:"rate"`
	QuoteUuid      string          `json:"quote"`
	SourceCurrency string          `json:"sourceCurrency"`
	TargetCurrency string          `json:"targetCurrency"`
	Details        TransferDetails `json:"details"`
}

type TransferDetails struct {
	Reference       string `json:"reference"`
	TransferPurpose string `json:"transferPurpose"`
	SourceOfFunds   string `json:"sourceOfFunds"`
}

type QuoteDetail struct {
	Id             string  `json:"id"`
	SourceAmount   float64 `json:"sourceAmount"`
	TargetAmount   float64 `json:"targetAmount"`
	Rate           float64 `json:"rate"`
	SourceCurrency string  `json:"source"`
	TargetCurrency string  `json:"target"`
	Profile        uint64  `json:"profile"`
}

type LiveRate struct {
	Rate float64 `json:"rate"`
}

type CreateTransferRequest struct {
	TargetAccount         uint64          `json:"targetAccount"`
	QuoteUuid             string          `json:"quoteUuid"`
	CustomerTransactionId string          `json:"customerTransactionId"`
	Details               TransferDetails `json:"details"`
}

type CreateQuoteRequest struct {
	SourceCurrency string  `json:"sourceCurrency"`
	TargetCurrency string  `json:"targetCurrency"`
	TargetAmount   float64 `json:"targetAmount"`
	Profile        uint64  `json:"profile"`
}
