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
var assetID = fmt.Sprintf("asset%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)

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
	// contract := network.GetContract(chaincodeName)

	// Context used for event listening
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		// Listen for events emitted by subsequent transactions
		done := startChaincodeEventListening(ctx, network)

		// firstBlockNumber := createAsset(contract)
		// updateAsset(contract)
		// transferAsset(contract)
		// deleteAsset(contract)

		// // Replay events from the block containing the first transaction
		// replayChaincodeEvents(ctx, network, firstBlockNumber)
		<-done

	}
}

// func createAsset(contract *client.Contract) uint64 {
// 	fmt.Printf("\n--> Submit transaction: CreateAsset, %s owned by Sam with appraised value 100\n", assetID)

// 	_, commit, err := contract.SubmitAsync("CreateAsset", client.WithArguments(assetID, assetID,
// 		"ITXISP1412",
// 		"AVATRICE ELECTROLUX TUTTO SPENTO",
// 		"20230110",
// 		"Z2",
// 		"Amalia",
// 		"Gennarelli",
// 		"IT",
// 		"Amalia.Gennarelli@gmail.com",
// 		"069863536",
// 		"Via Calipso 00042",
// 		"Anzio",
// 		"IT",
// 		"00042",
// 		"1239040",
// 		"11",
// 		"Nexure",
// 		"20201007",
// 		"20221007",
// 		"914916405",
// 		"20201007",
// 		"Grando",
// 		"2 LW",
// 		"00",
// 		"03700396"))
// 	if err != nil {
// 		panic(fmt.Errorf("failed to submit transaction: %w", err))
// 	}

// 	status, err := commit.Status()
// 	if err != nil {
// 		panic(fmt.Errorf("failed to get transaction commit status: %w", err))
// 	}

// 	if !status.Successful {
// 		panic(fmt.Errorf("failed to commit transaction with status code %v", status.Code))
// 	}

// 	fmt.Println("\n*** CreateAsset committed successfully")

// 	return status.BlockNumber
// }

// func updateAsset(contract *client.Contract) {
// 	fmt.Printf("\n--> Submit transaction: UpdateAsset, %s update appraised value to 200\n", assetID)

// 	_, err := contract.SubmitTransaction("UpdateTicketStatus", assetID, "blue")
// 	if err != nil {
// 		panic(fmt.Errorf("failed to submit transaction: %w", err))
// 	}

// 	fmt.Println("\n*** UpdateAsset committed successfully")
// }

// func transferAsset(contract *client.Contract) {
// 	fmt.Printf("\n--> Submit transaction: TransferAsset, %s to Mary\n", assetID)

// 	_, err := contract.SubmitTransaction("UpdateTicketStatus", assetID, "White")
// 	if err != nil {
// 		panic(fmt.Errorf("failed to submit transaction: %w", err))
// 	}

// 	fmt.Println("\n*** TransferAsset committed successfully")
// }

// func deleteAsset(contract *client.Contract) {
// 	fmt.Printf("\n--> Submit transaction: DeleteAsset, %s\n", assetID)

// 	_, err := contract.SubmitTransaction("DeleteAsset", assetID)
// 	if err != nil {
// 		panic(fmt.Errorf("failed to submit transaction: %w", err))
// 	}

// 	fmt.Println("\n*** DeleteAsset committed successfully")
// }

// func replayChaincodeEvents(ctx context.Context, network *client.Network, startBlock uint64) {
// 	fmt.Println("\n*** Start chaincode event replay")

// 	events, err := network.ChaincodeEvents(ctx, chaincodeName, client.WithStartBlock(startBlock))
// 	if err != nil {
// 		panic(fmt.Errorf("failed to start chaincode event listening: %w", err))
// 	}

// 	for {
// 		select {
// 		case <-time.After(10 * time.Second):
// 			panic(errors.New("timeout waiting for event replay"))

// 		case event := <-events:
// 			asset := formatJSON(event.Payload)
// 			fmt.Printf("\n<-- Chaincode event replayed: %s - %s\n", event.EventName, asset)

// 			if event.EventName == "DeleteAsset" {
// 				// Reached the last submitted transaction so return to stop listening for events
// 				return
// 			}
// 		}
// 	}
// }

func startChaincodeEventListening(ctx context.Context, network *client.Network) chan bool {
	done := make(chan bool)
	fmt.Println("\n*** Start chaincode event listening")

	events, err := network.ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		panic(fmt.Errorf("failed to start chaincode event listening: %w", err))
	}

	go func() {
		for event := range events {
			fmt.Printf("\n<-- Chaincode event received: %s - %s\n", event.EventName, formatJSON(event.Payload))
			handleEvent(event.Payload, event.EventName)
			if err != nil {
				fmt.Printf("\n<-- Unable to unmarshal the venet payload : %s", formatJSON(event.Payload))
			}
			fmt.Printf("\n<-- Chaincode event received: %s - %s\n", event.EventName, formatJSON(event.Payload))
			done <- true
		}
	}()
	return done
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
		executeFunctionCall(data, goDotEnvVariable("POWERAPPS_URL"))
	case "UpdateTicket":
		executeFunctionCall(data, goDotEnvVariable("FUNCTION_HANDLER_UPDATE"))
		executeFunctionCall(data, goDotEnvVariable("POWERAPPS_URL"))
	case "SubmitClaim":
		executeFunctionCall(data, goDotEnvVariable("FUNCTION_HANDLER_CLAIM"))
	case "UpdateTicketStatus":
		executeFunctionCall(data, goDotEnvVariable("FUNCTION_HANDLER_UPDATE"))
	case "AddParts":
		executeFunctionCall(data, goDotEnvVariable("FUNCTION_HANDLER_UPDATE"))

	}

}

// Function to invoke HTTP Post to call the event handle azure function
func executeFunctionCall(body []byte, url string) error {
	method := "POST"
	fmt.Println("Invoke URL %s", url)

	// JSON body
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Connection cannot be establish %v", err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {

		fmt.Println("Unable to connect to the URL %v", err)
		return err
	}
	defer res.Body.Close()

	response, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Unable to read the response %v", err)
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
