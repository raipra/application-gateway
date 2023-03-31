/*
Copyright 2022 IBM All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/joho/godotenv"
)

const (
	channelName   = "mychannel"
	chaincodeName = "ticket"
)

var now = time.Now()
var assetID = "101215487"

func main() {
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	gateway, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer gateway.Close()

	network := gateway.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)
	// Context used for event listening
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for events emitted by subsequent transactions
	startChaincodeEventListening(ctx, network)
	updateAsset(contract)
}
func updateAsset(contract *client.Contract) {
	fmt.Printf("\n--> Submit transaction: UpdateAsset, %s update appraised value to 200\n", assetID)

	_, err := contract.SubmitTransaction("UpdateTicketStatus", assetID, "Accepted")
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Println("\n*** UpdateAsset committed successfully")
}

func startChaincodeEventListening(ctx context.Context, network *client.Network) {
	fmt.Println("\n*** Start chaincode event listening")

	events, err := network.ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		panic(fmt.Errorf("failed to start chaincode event listening: %w", err))
	}

	go func() {
		for event := range events {
			fmt.Printf("\n<-- Chaincode event received: %s - %s\n", event.EventName, formatJSON(event.Payload))
			executeFunctionCall(event.Payload)
			if err != nil {
				fmt.Printf("\n<-- Unable to unmarshal the venet payload : %s", formatJSON(event.Payload))
			}
			fmt.Printf("\n<-- Chaincode event received: %s - %s\n", event.EventName, formatJSON(event.Payload))
		}
	}()

}
func formatJSON(data []byte) string {
	var result bytes.Buffer
	if err := json.Indent(&result, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}

	return result.String()
}

func handleEvent(data []byte, eventName string) {
	//asset := formatJSON(data)
	switch eventName {
	case "CreateAsset":
		executeFunctionCall(data)

	case "UpdateAsset":
		executeFunctionCall(data)
	}

}

// Function to invoke HTTP Post to call the event handle azure function
func executeFunctionCall(body []byte) error {
	url := goDotEnvVariable("FUNCTION_HANDLER_HOST")
	method := "POST"

	// JSON body
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println(err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer res.Body.Close()

	response, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(string(response))
	return nil
}

// use godot package to load/read the .env file and
// return the value of the key
func goDotEnvVariable(key string) string {
	// load .env file
	err := godotenv.Load("connection.env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// load .env file
	envVariable := os.Getenv(key)

	return envVariable
}
