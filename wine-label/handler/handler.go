/**
 * Copyright 2017 Intel Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * ------------------------------------------------------------------------------
 */

package handler

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"

	cbor "github.com/brianolson/cbor_go"
	"github.com/hyperledger/sawtooth-sdk-go/logging"
	"github.com/hyperledger/sawtooth-sdk-go/processor"
	"github.com/hyperledger/sawtooth-sdk-go/protobuf/processor_pb2"
)

var logger *logging.Logger = logging.Get()

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

type WineLabelHandler struct {
	namespace string
}

func NewWineLabelHandler(namespace string) *WineLabelHandler {
	return &WineLabelHandler{
		namespace: namespace,
	}
}

const (
	MIN_VALUE       = 0
	MAX_VALUE       = 4294967295
	MAX_NAME_LENGTH = 20
	FAMILY_NAME     = "wine-label"
)

func (self *WineLabelHandler) FamilyName() string {
	return FAMILY_NAME
}

func (self *WineLabelHandler) FamilyVersions() []string {
	return []string{"1.0"}
}

func (self *WineLabelHandler) Namespaces() []string {
	return []string{self.namespace}
}

func (self *WineLabelHandler) Apply(request *processor_pb2.TpProcessRequest, context *processor.Context) error {
	payloadData := request.GetPayload()
	if payloadData == nil {
		return &processor.InvalidTransactionError{Msg: "Must contain payload"}
	}
	var payload WineLabelPayload
	err := DecodeCBOR(payloadData, &payload)
	if err != nil {
		return &processor.InvalidTransactionError{
			Msg: fmt.Sprint("Failed to decode payload: ", err),
		}
	}

	if err != nil {
		logger.Error("Bad payload: ", payloadData)
		return &processor.InternalError{Msg: fmt.Sprint("Failed to decode payload: ", err)}
	}

	verb := payload.Verb
	id := payload.WineLabelID
	state := &Payload{payload.WineLabelID, payload.PrintedAt, payload.Lattitude, payload.Longitude}

	if len(id) == 0 {
		return &processor.InvalidTransactionError{
			Msg: fmt.Sprintf(
				"Should be valid wine label ID",
				MAX_NAME_LENGTH),
		}
	}
	if !(verb == "set" || verb == "del") {
		return &processor.InvalidTransactionError{Msg: fmt.Sprintf("Invalid verb: %v", verb)}
	}

	hashed_labled_id := Hexdigest(payload.WineLabelID)
	address := self.namespace + hashed_labled_id[len(hashed_labled_id)-64:]
	results, err := context.GetState([]string{address})
	if err != nil {
		return err
	}

	data, exists := results[address]
	if exists && verb == "del" {
		data, _ = EncodeCBOR(Payload{})
	} else {
		data, _ = EncodeCBOR(state)
	}

	addresses, err := context.SetState(map[string][]byte{
		address: data,
	})
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return &processor.InternalError{Msg: "No addresses in set response"}
	}

	return nil
}

func EncodeCBOR(value interface{}) ([]byte, error) {
	data, err := cbor.Dumps(value)
	return data, err
}

func DecodeCBOR(data []byte, pointer interface{}) error {
	defer func() error {
		if recover() != nil {
			return &processor.InvalidTransactionError{Msg: "Failed to decode payload"}
		}
		return nil
	}()
	err := cbor.Loads(data, pointer)
	if err != nil {
		return err
	}
	return nil
}

func Hexdigest(str string) string {
	hash := sha512.New()
	hash.Write([]byte(str))
	hashBytes := hash.Sum(nil)
	return strings.ToLower(hex.EncodeToString(hashBytes))
}
