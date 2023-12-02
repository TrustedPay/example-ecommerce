package main

import (
	"context"
	"net"

	"github.com/TrustedPay/tp-term/pkg/tpterm"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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

	// TODO: request nonce from bank

	// Generate a transaction
	transaction := tpterm.Transaction{
		Nonce:           0,
		TransactionData: []byte{},
		Amount:          100,
		CardNumber:      "8888888888888888",
		CardExp:         "01/99",
	}
	logrus.Printf("Attempting transaction of $%.2f on card %s (Exp. %s)", float64(transaction.Amount)/100, transaction.CardNumber, transaction.CardExp)

	// Ask TP Term to sign transaction
	tpSignature, err := tpClient.SignRequest(ctx, &transaction)
	if err != nil {
		logrus.Errorf("Error: %v", err)
		logrus.Fatalf("TP Term failed signature, cancelling transaction...")
	}

	logrus.Printf("Got valid signature for transaction: %x", tpSignature.TransactionSignature)
}
