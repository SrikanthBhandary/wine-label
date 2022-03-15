package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/sawtooth-sdk-go/logging"
	flags "github.com/jessevdk/go-flags"

	cl "wine-client/client"
)

type Opts struct {
	Verbose []bool `short:"v" long:"verbose" description:"Enable more verbose output"`
	Version bool   `short:"V" long:"version" description:"Display version information"`
}

var DISTRIBUTION_VERSION string

var logger *logging.Logger = logging.Get()

func init() {
	if len(DISTRIBUTION_VERSION) == 0 {
		DISTRIBUTION_VERSION = "Unknown"
	}
}

func main() {
	arguments := os.Args[1:]
	for _, arg := range arguments {
		if arg == "-V" || arg == "--version" {
			fmt.Println("HP BC" + " (Hyperledger Sawtooth) version " + DISTRIBUTION_VERSION)
			os.Exit(0)
		}
	}

	var opts Opts
	parser := flags.NewParser(&opts, flags.Default)
	parser.Command.Name = "intkey"

	// Add sub-commands
	commands := []cl.Command{
		&cl.Set{},
	}
	for _, cmd := range commands {
		err := cmd.Register(parser.Command)
		if err != nil {
			logger.Errorf("Couldn't register command %v: %v", cmd.Name(), err)
			os.Exit(1)
		}
	}

	remaining, err := parser.Parse()
	if e, ok := err.(*flags.Error); ok {
		if e.Type == flags.ErrHelp {
			return
		} else {
			os.Exit(1)
		}
	}

	if len(remaining) > 0 {
		fmt.Println("Error: Unrecognized arguments passed: ", remaining)
		os.Exit(2)
	}

	switch len(opts.Verbose) {
	case 2:
		logger.SetLevel(logging.DEBUG)
	case 1:
		logger.SetLevel(logging.INFO)
	default:
		logger.SetLevel(logging.WARN)
	}

	// If a sub-command was passed, run it
	if parser.Command.Active == nil {
		os.Exit(2)
	}

	name := parser.Command.Active.Name
	for _, cmd := range commands {
		if cmd.Name() == name {
			err := cmd.Run()
			if err != nil {
				fmt.Println("Error: ", err)
				os.Exit(1)
			}
			return
		}
	}

	fmt.Println("Error: Command not found: ", name)
}

func GetClient(args cl.Command, readFile bool) (cl.WineLabelClient, error) {
	url := args.UrlPassed()
	if url == "" {
		url = cl.DEFAULT_URL
	}
	keyfile := ""
	if readFile {
		var err error
		keyfile, err = cl.GetKeyfile(args.KeyfilePassed())
		if err != nil {
			return cl.WineLabelClient{}, err
		}
	}
	return cl.NewWineLabelClient(url, keyfile)
}
