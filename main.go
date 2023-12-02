package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"

	"github.com/TrustedPay/tp-term/pkg/tpterm"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NonceResponse struct {
	Nonce         int64  `json:"nonce"`
	TransactionID string `json:"transactionId"`
}

type AuthorizeRequest struct {
	TransactionID string              `json:"transactionId"`
	Request       *tpterm.Transaction `json:"req"`
	Signature     []byte              `json:"signature"`
}

func main() {
	// Generate the TP Term gRPC Client
	var opts []grpc.DialOption = []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", addr)
		}),
	}
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial("/tmp/tp-term.sock", opts...)
	if err != nil {
		logrus.Fatalf("fail to dial: %v", err)
	}
	tpClient := tpterm.NewTPTermClient(conn)

	// Context for operations
	ctx := context.Background()

	resp, err := http.Get("http://localhost:8080/transaction/initialize")
	if err != nil {
		logrus.Fatalf("failed to get nonce from bank: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("failed to read nonce body: %v", err)
	}
	var nonceResp NonceResponse
	err = json.Unmarshal(bodyBytes, &nonceResp)
	if err != nil {
		logrus.Fatalf("failed to parse nonce response: %v", err)
	}
	logrus.Infof("Got nonce %d for transaction %s from bank", nonceResp.Nonce, nonceResp.TransactionID)

	// Generate a transaction
	transaction := tpterm.Transaction{
		Nonce:           nonceResp.Nonce,
		TransactionData: []byte{},
		Amount:          100,
		CardNumber:      "8888888888888888",
		CardExp:         "01/99",
	}
	logrus.Printf("Attempting transaction of $%.2f on card %s (Exp. %s)", float64(transaction.Amount)/100, transaction.CardNumber, transaction.CardExp)

	logrus.Infof("Sending request to TP Term for transaction signing...")
	// Ask TP Term to sign transaction
	tpSignature, err := tpClient.SignRequest(ctx, &transaction)
	if err != nil {
		logrus.Errorf("Error: %v", err)
		logrus.Fatalf("TP Term failed signature, cancelling transaction...")
	}

	logrus.Printf("Got valid signature for transaction:\nDigest: %x\nSignature: %x", tpSignature.TransactionDigest, tpSignature.TransactionSignature)

	logrus.Printf("Attempting to authorize transaction...")

	request := AuthorizeRequest{
		TransactionID: nonceResp.TransactionID,
		Request:       &transaction,
		Signature:     tpSignature.TransactionSignature,
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		logrus.Fatalf("failed to marshal authorize request bytes")
	}

	resp, err = http.Post("http://localhost:8080/transaction/authorize", "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		logrus.Fatalf("failed to POST authorize request: %v", err)
	}

	if resp.StatusCode == 200 {
		logrus.Infof("Transaction successful!")
	} else if resp.StatusCode == 403 {
		logrus.Warnf("Transaction failed. Not enough funds...")
	} else {
		logrus.Errorf("Something went wrong. Please check the logs...")
	}
}
