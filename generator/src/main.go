package main

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/sekerez/polka/generator/src/spammer"
)

const (
	envPath = "generator/env/generator.env"
)

func main() {

	// Send a hello?
	helloPtr := flag.Bool("h", false, "whether to send a hello GET request")

	// Send random payments?
	workerPtr := flag.Uint("w", 3000, "the number of workers")
	transactionsPtr := flag.Uint("t", 100, "the number of transactions sent")

	// Measure performance?
	measurePtr := flag.Bool("m", false, "whether to measure request time")

	// Request a snapshot or request a settlement?
	getSnapshotPtr := flag.Bool("gs", false, "whether to get a snapshot")
	settleBalancesPtr := flag.Bool("sb", false, "whether to settle balances given a snapshot")

	// Parse flags
	flag.Parse()

	// Get url to services
	if err := godotenv.Load(envPath); err != nil {
		log.Fatalf("Environmental variables failed to load: %s\n", err)
	}

	mainDest := os.Getenv("MAINURL")
	helloDest := os.Getenv("HELLOURL")
	settleDest := os.Getenv("SETTLERURL")

	if *getSnapshotPtr {
		_, err := spammer.GetSnapshot(settleDest)
		if err != nil {
			log.Fatalf("Error requesting snapshot: %s", err.Error())
		}
		return
	}

	if *settleBalancesPtr {
		err := spammer.SettleBalances(settleDest)
		if err != nil {
			log.Printf("Error requesting snapshot: %s", err.Error())
			return
		}
		log.Printf("Balances have been settled successfully.")
		return
	}

	// Say hello if asked!
	if *helloPtr {
		spammer.SayHello(helloDest)
	}

	log.Printf("Sending %d transactions with %d workers", *transactionsPtr, *workerPtr)
	badReqs := spammer.PaymentSpammer(mainDest, *workerPtr, *transactionsPtr, *measurePtr)
	log.Printf("Of all requests, %d were successful and %d failed.", *transactionsPtr-uint(badReqs), badReqs)
}
