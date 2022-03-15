package client

import (
	bytes2 "bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	cbor "github.com/brianolson/cbor_go"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/sawtooth-sdk-go/protobuf/batch_pb2"
	"github.com/hyperledger/sawtooth-sdk-go/protobuf/transaction_pb2"
	"github.com/hyperledger/sawtooth-sdk-go/signing"
	"gopkg.in/yaml.v2"
)

const (
	// String literals
	FAMILY_NAME       string = "wine-label"
	FAMILY_VERSION    string = "1.0"
	DISTRIBUTION_NAME string = "sawtooth-intkey"
	DEFAULT_URL       string = "http://127.0.0.1:8008"

	// APIs
	BATCH_SUBMIT_API string = "batches"
	BATCH_STATUS_API string = "batch_statuses"
	STATE_API        string = "state"
	// Content types
	CONTENT_TYPE_OCTET_STREAM string = "application/octet-stream"
	// Integer literals
	FAMILY_NAMESPACE_ADDRESS_LENGTH uint = 6
	FAMILY_VERB_ADDRESS_LENGTH      uint = 64
)

type WineLabelClient struct {
	url    string
	signer *signing.Signer
}

type WineLabelPayload struct {
	Payload
	Verb string
}

type Payload struct {
	WineLabelID string
	PrintedAt   string
	Longitude   string
	Lattitude   string
}

func NewWineLabelClient(url string, keyfile string) (WineLabelClient, error) {
	var privateKey signing.PrivateKey
	if keyfile != "" {
		// Read private key file
		privateKeyStr, err := ioutil.ReadFile(keyfile)
		if err != nil {
			return WineLabelClient{},
				errors.New(fmt.Sprintf("Failed to read private key: %v", err))
		}
		// Get private key object
		privateKey = signing.NewSecp256k1PrivateKey(privateKeyStr)
	} else {
		privateKey = signing.NewSecp256k1Context().NewRandomPrivateKey()
	}
	cryptoFactory := signing.NewCryptoFactory(signing.NewSecp256k1Context())
	signer := cryptoFactory.NewSigner(privateKey)
	return WineLabelClient{url, signer}, nil
}

func (self WineLabelClient) Set(
	labelID, location, long, lat string, wait uint) (string, error) {
	return self.sendTransaction("set", labelID, location, long, lat, wait)
}

func (self WineLabelClient) Delete(
	labelID string, wait uint) (string, error) {
	return self.sendTransaction("delete", "", "", "", "", wait)
}

func (self WineLabelClient) List() ([]WineLabelPayload, error) {
	// API to call
	var payload []WineLabelPayload
	apiSuffix := fmt.Sprintf("%s?address=%s",
		STATE_API, self.getPrefix())
	response, err := self.sendRequest(apiSuffix, []byte{}, "", "")
	if err != nil {
		return payload, err
	}
	var toReturn []WineLabelPayload
	responseMap := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(response), &responseMap)
	if err != nil {
		return payload,
			errors.New(fmt.Sprintf("Error reading response: %v", err))
	}
	encodedEntries := responseMap["data"].([]interface{})
	for _, entry := range encodedEntries {
		entryData, ok := entry.(map[interface{}]interface{})
		if !ok {
			return payload,
				errors.New("Error reading entry data")
		}
		stringData, ok := entryData["data"].(string)
		if !ok {
			return payload,
				errors.New("Error reading string data")
		}
		decodedBytes, err := base64.StdEncoding.DecodeString(stringData)
		if err != nil {
			return payload,
				errors.New(fmt.Sprint("Error decoding: %v", err))
		}
		var foundMap WineLabelPayload
		err = cbor.Loads(decodedBytes, &foundMap)
		if err != nil {
			return payload,
				errors.New(fmt.Sprint("Error binary decoding: %v", err))
		}
		toReturn = append(toReturn, foundMap)
	}
	return toReturn, nil
}

func (self WineLabelClient) Show(labelID string) (string, error) {
	apiSuffix := fmt.Sprintf("%s/%s", STATE_API, self.getAddress(labelID))
	response, err := self.sendRequest(apiSuffix, []byte{}, "", labelID)
	if err != nil {
		return "", err
	}
	responseMap := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(response), &responseMap)
	if err != nil {
		return "", errors.New(fmt.Sprint("Error reading response: %v", err))
	}
	data, ok := responseMap["data"].(string)
	if !ok {
		return "", errors.New("Error reading as string")
	}
	responseData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", errors.New(fmt.Sprint("Error decoding response: %v", err))
	}
	var responseFinal WineLabelPayload
	err = cbor.Loads(responseData, &responseFinal)
	if err != nil {
		return "", errors.New(fmt.Sprint("Error binary decoding: %v", err))
	}
	return fmt.Sprintf("%v", responseFinal.WineLabelID), nil
}

func (self WineLabelClient) getStatus(
	batchId string, wait uint) (string, error) {

	// API to call
	apiSuffix := fmt.Sprintf("%s?id=%s&wait=%d",
		BATCH_STATUS_API, batchId, wait)
	response, err := self.sendRequest(apiSuffix, []byte{}, "", "")
	if err != nil {
		return "", err
	}

	responseMap := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(response), &responseMap)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error reading response: %v", err))
	}
	entry :=
		responseMap["data"].([]interface{})[0].(WineLabelPayload)
	return fmt.Sprint(entry.WineLabelID), nil
}

func (self WineLabelClient) sendRequest(
	apiSuffix string,
	data []byte,
	contentType string,
	name string) (string, error) {

	// Construct URL
	var url string
	if strings.HasPrefix(self.url, "http://") {
		url = fmt.Sprintf("%s/%s", self.url, apiSuffix)
	} else {
		url = fmt.Sprintf("http://%s/%s", self.url, apiSuffix)
	}
	fmt.Println("URL :", url)
	fmt.Println("data :", data)

	// Send request to validator URL
	var response *http.Response
	var err error
	if len(data) > 0 {
		response, err = http.Post(url, contentType, bytes2.NewBuffer(data))
		fmt.Println(response)
	} else {
		response, err = http.Get(url)
	}
	if err != nil {
		fmt.Println(err)
		return "", errors.New(
			fmt.Sprintf("Failed to connect to REST API: %v", err))
	}
	if response.StatusCode == 404 {
		return "", errors.New(fmt.Sprintf("No such key: %s", name))
	} else if response.StatusCode >= 400 {
		return "", errors.New(
			fmt.Sprintf("Error %d: %s", response.StatusCode, response.Status))
	}
	defer response.Body.Close()
	reponseBody, err := ioutil.ReadAll(response.Body)
	fmt.Println("Resposen body -- ")
	fmt.Println(reponseBody)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error reading response: %v", err))
	}
	return string(reponseBody), nil
}

func (self WineLabelClient) sendTransaction(
	verb string, labelID string, location, long, lat string, wait uint) (string, error) {
	// construct the payload information in CBOR format
	payloadData := WineLabelPayload{}
	payloadData.Verb = verb
	payloadData.WineLabelID = labelID
	payloadData.PrintedAt = location
	payloadData.Longitude = long
	payloadData.Lattitude = lat
	payload, err := cbor.Dumps(payloadData)

	fmt.Println("--------")
	fmt.Println(payloadData.WineLabelID)

	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to construct CBOR: %v", err))
	}

	// construct the address
	address := self.getAddress(labelID)

	fmt.Println("-------- address")
	fmt.Println(address)
	fmt.Println(self.signer.GetPublicKey().AsHex())

	// Construct TransactionHeader
	rawTransactionHeader := transaction_pb2.TransactionHeader{
		SignerPublicKey:  self.signer.GetPublicKey().AsHex(),
		FamilyName:       FAMILY_NAME,
		FamilyVersion:    FAMILY_VERSION,
		Dependencies:     []string{}, // empty dependency list
		Nonce:            strconv.Itoa(rand.Int()),
		BatcherPublicKey: self.signer.GetPublicKey().AsHex(),
		Inputs:           []string{address},
		Outputs:          []string{address},
		PayloadSha512:    Sha512HashValue(string(payload)),
	}
	transactionHeader, err := proto.Marshal(&rawTransactionHeader)
	if err != nil {
		return "", errors.New(
			fmt.Sprintf("Unable to serialize transaction header: %v", err))
	}

	// Signature of TransactionHeader
	transactionHeaderSignature := hex.EncodeToString(
		self.signer.Sign(transactionHeader))

	// Construct Transaction
	transaction := transaction_pb2.Transaction{
		Header:          transactionHeader,
		HeaderSignature: transactionHeaderSignature,
		Payload:         []byte(payload),
	}

	// Get BatchList
	rawBatchList, err := self.createBatchList(
		[]*transaction_pb2.Transaction{&transaction})
	if err != nil {
		return "", errors.New(
			fmt.Sprintf("Unable to construct batch list: %v", err))
	}
	batchId := rawBatchList.Batches[0].HeaderSignature
	batchList, err := proto.Marshal(&rawBatchList)
	fmt.Println("Batch ID:", batchId)
	if err != nil {
		return "", errors.New(
			fmt.Sprintf("Unable to serialize batch list: %v", err))
	}

	fmt.Println("--- Batch list ---")
	fmt.Println(batchList)
	if wait > 0 {

		fmt.Println("Entered")
		waitTime := uint(0)
		startTime := time.Now()
		response, err := self.sendRequest(
			BATCH_SUBMIT_API, batchList, CONTENT_TYPE_OCTET_STREAM, labelID)
		if err != nil {
			return "", err
		}
		for waitTime < wait {
			status, err := self.getStatus(batchId, wait-waitTime)
			if err != nil {
				return "", err
			}
			waitTime = uint(time.Now().Sub(startTime))
			if status != "PENDING" {
				return response, nil
			}
		}
		fmt.Println("Respoonse:")
		fmt.Println(response)
		return response, nil
	}

	return self.sendRequest(
		BATCH_SUBMIT_API, batchList, CONTENT_TYPE_OCTET_STREAM, labelID)
}

func (self WineLabelClient) getPrefix() string {
	return Sha512HashValue(FAMILY_NAME)[:FAMILY_NAMESPACE_ADDRESS_LENGTH]
}

func (self WineLabelClient) getAddress(name string) string {
	prefix := self.getPrefix()
	nameAddress := Sha512HashValue(name)[FAMILY_VERB_ADDRESS_LENGTH:]
	return prefix + nameAddress
}

func (self WineLabelClient) createBatchList(
	transactions []*transaction_pb2.Transaction) (batch_pb2.BatchList, error) {

	// Get list of TransactionHeader signatures
	transactionSignatures := []string{}
	for _, transaction := range transactions {
		transactionSignatures =
			append(transactionSignatures, transaction.HeaderSignature)
	}

	// Construct BatchHeader
	rawBatchHeader := batch_pb2.BatchHeader{
		SignerPublicKey: self.signer.GetPublicKey().AsHex(),
		TransactionIds:  transactionSignatures,
	}
	batchHeader, err := proto.Marshal(&rawBatchHeader)
	if err != nil {
		return batch_pb2.BatchList{}, errors.New(
			fmt.Sprintf("Unable to serialize batch header: %v", err))
	}

	// Signature of BatchHeader
	batchHeaderSignature := hex.EncodeToString(
		self.signer.Sign(batchHeader))

	// Construct Batch
	batch := batch_pb2.Batch{
		Header:          batchHeader,
		Transactions:    transactions,
		HeaderSignature: batchHeaderSignature,
	}

	// Construct BatchList
	return batch_pb2.BatchList{
		Batches: []*batch_pb2.Batch{&batch},
	}, nil
}

func Sha512HashValue(value string) string {
	hashHandler := sha512.New()
	hashHandler.Write([]byte(value))
	return strings.ToLower(hex.EncodeToString(hashHandler.Sum(nil)))
}

func GetKeyfile(keyfile string) (string, error) {
	if keyfile == "" {
		username, err := user.Current()
		if err != nil {
			return "", err
		}
		return path.Join(
			username.HomeDir, ".sawtooth", "keys", username.Username+".priv"), nil
	} else {
		return keyfile, nil
	}
}
