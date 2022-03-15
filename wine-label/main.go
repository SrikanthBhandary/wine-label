package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/hyperledger/sawtooth-sdk-go/logging"
	"github.com/hyperledger/sawtooth-sdk-go/processor"
	flags "github.com/jessevdk/go-flags"

	intkey "wine-label/handler"
)

type Opts struct {
	Verbose []bool `short:"v" long:"verbose" description:"Increase verbosity"`
	Connect string `short:"C" long:"connect" description:"Validator component endpoint to connect to" default:"tcp://localhost:4004"`
	Queue   uint   `long:"max-queue-size" description:"Set the maximum queue size before rejecting process requests" default:"100"`
	Threads uint   `long:"worker-thread-count" description:"Set the number of worker threads to use for processing requests in parallel" default:"0"`
}

func main() {
	var opts Opts

	logger := logging.Get()

	parser := flags.NewParser(&opts, flags.Default)
	remaining, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			logger.Errorf("Failed to parse args: %v", err)
			os.Exit(2)
		}
	}

	if len(remaining) > 0 {
		fmt.Printf("Error: Unrecognized arguments passed: %v\n", remaining)
		os.Exit(2)
	}

	endpoint := opts.Connect

	switch len(opts.Verbose) {
	case 2:
		logger.SetLevel(logging.DEBUG)
	case 1:
		logger.SetLevel(logging.INFO)
	default:
		logger.SetLevel(logging.WARN)
	}

	prefix := intkey.Hexdigest("wine-label")[:6]
	fmt.Println("Prefix :", prefix)
	handler := intkey.NewWineLabelHandler(prefix)
	processor := processor.NewTransactionProcessor(endpoint)
	processor.SetMaxQueueSize(opts.Queue)
	if opts.Threads > 0 {
		processor.SetThreadCount(opts.Threads)
	}
	processor.AddHandler(handler)
	processor.ShutdownOnSignal(syscall.SIGINT, syscall.SIGTERM)
	err = processor.Start()
	if err != nil {
		logger.Error("Processor stopped: ", err)
	}

}
