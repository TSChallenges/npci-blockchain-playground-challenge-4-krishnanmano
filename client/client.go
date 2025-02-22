package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
)

const (
	channelName   = "assetchannel"
	chaincodeName = "basic"
)

func main() {
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	gateway, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithHash(hash.SHA256),
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

	// TODO: Create a new investor
	_, err = contract.SubmitTransaction("CreateUser", "investor3", "1000000000")
	if err != nil {
		log.Fatalf("Failed to create investor: %v", err)
	}
	fmt.Println("Invetor successfully created.")

	// TODO: Register a new asset
	_, err = contract.SubmitTransaction("RegisterAsset", "asset3", "company1", "bond", "300", "10", "30", "10", "7")
	if err != nil {
		log.Fatalf("Failed to register asset: %v", err)
	}
	fmt.Println("asset registered successfully.")

	// TODO: Subscribe to the asset
	_, err = contract.SubmitTransaction("SubscribeAsset", "asset3", "30", "investor3")
	if err != nil {
		log.Fatalf("Failed to subscribe asset: %v", err)
	}
	fmt.Println("asset subscribed successfully.")

	// TODO: Redeem assets
	_, err = contract.SubmitTransaction("RedeemAsset", "asset3", "10", "investor3")
	if err != nil {
		log.Fatalf("Failed to reedeem asset: %v", err)
	}
	fmt.Println("asset reedeemed successfully.")

	// TODO: Check the investor's portfolio balance
	result, err := contract.EvaluateTransaction("GetPortfolio", "investor3")
	if err != nil {
		log.Fatalf("Failed to get loan balance: %v", err)
	}
	fmt.Printf("Portfolio details: %s", string(result))

	// Replay events from the block containing the first transaction
	replayChaincodeEvents(ctx, network, 0)
}

func startChaincodeEventListening(ctx context.Context, network *client.Network) {
	fmt.Println("\n*** Start chaincode event listening")

	events, err := network.ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		panic(fmt.Errorf("failed to start chaincode event listening: %w", err))
	}

	go func() {
		for event := range events {
			asset := formatJSON(event.Payload)
			fmt.Printf("\n<-- Chaincode event received: %s - %s\n", event.EventName, asset)
		}
	}()
}

func replayChaincodeEvents(ctx context.Context, network *client.Network, startBlock uint64) {
	fmt.Println("\n*** Start chaincode event replay")

	events, err := network.ChaincodeEvents(ctx, chaincodeName, client.WithStartBlock(startBlock))
	if err != nil {
		panic(fmt.Errorf("failed to start chaincode event listening: %w", err))
	}

	for {
		select {
		case <-time.After(10 * time.Second):
			panic(errors.New("timeout waiting for event replay"))

		case event := <-events:
			asset := formatJSON(event.Payload)
			fmt.Printf("\n<-- Chaincode event replayed: %s - %s\n", event.EventName, asset)

			if event.EventName == "DeleteAsset" {
				// Reached the last submitted transaction so return to stop listening for events
				return
			}
		}
	}
}

func formatJSON(data []byte) string {
	var result bytes.Buffer
	if err := json.Indent(&result, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return result.String()
}
