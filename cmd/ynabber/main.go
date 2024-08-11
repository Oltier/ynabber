package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/carlmjohnson/versioninfo"
	"github.com/kelseyhightower/envconfig"
	"github.com/martinohansen/ynabber"
	"github.com/martinohansen/ynabber/reader/nordigen"
	"github.com/martinohansen/ynabber/writer/json"
	"github.com/martinohansen/ynabber/writer/ynab"
	"log"
	"os"
	"strings"
)

type MyEvent struct {
	Name string `json:"name"`
}

func HandleLambdaRequest(ctx context.Context, event *MyEvent) (*string, error) {
	log.Println("Version:", versioninfo.Short())

	// Read config from env
	var cfg ynabber.Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Check that some values are valid
	cfg.YNAB.Cleared = strings.ToLower(cfg.YNAB.Cleared)
	if cfg.YNAB.Cleared != "cleared" &&
		cfg.YNAB.Cleared != "uncleared" &&
		cfg.YNAB.Cleared != "reconciled" {
		log.Fatal("YNAB_CLEARED must be one of cleared, uncleared or reconciled")
	}

	if cfg.Debug {
		log.Printf("Config: %+v\n", cfg)
	}

	ynabber := ynabber.Ynabber{}
	for _, reader := range cfg.Readers {
		switch reader {
		case "nordigen":
			ynabber.Readers = append(ynabber.Readers, nordigen.NewReader(&cfg))
		default:
			log.Fatalf("Unknown reader: %s", reader)
		}
	}
	for _, writer := range cfg.Writers {
		switch writer {
		case "ynab":
			ynabber.Writers = append(ynabber.Writers, ynab.Writer{Config: &cfg})
		case "json":
			ynabber.Writers = append(ynabber.Writers, json.Writer{})
		default:
			log.Fatalf("Unknown writer: %s", writer)
		}
	}

	err = run(ynabber)
	if err != nil {
		return nil, err
	} else {
		message := fmt.Sprintf("Run succeeded")
		log.Printf("%s", message)
		return &message, nil
	}
}

func run(y ynabber.Ynabber) error {
	var transactions []ynabber.Transaction

	// Read transactions from all readers
	for _, reader := range y.Readers {
		t, err := reader.Bulk()
		if err != nil {
			return fmt.Errorf("reading: %w", err)
		}
		transactions = append(transactions, t...)
	}

	// Write transactions to all writers
	for _, writer := range y.Writers {
		err := writer.Bulk(transactions)
		if err != nil {
			return fmt.Errorf("writing: %w", err)
		}
	}
	return nil
}

func main() {
	isLambda := len(os.Getenv("LAMBDA_TASK_ROOT")) > 0
	if isLambda {
		lambda.Start(HandleLambdaRequest)
	} else {
		event := &MyEvent{Name: "cica"}
		HandleLambdaRequest(context.TODO(), event)
	}
}
