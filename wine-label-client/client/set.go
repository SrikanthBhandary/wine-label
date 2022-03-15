package client

import (
	"github.com/jessevdk/go-flags"
)

// All subcommands implement this interface
type Command interface {
	Register(*flags.Command) error
	Name() string
	KeyfilePassed() string
	UrlPassed() string
	Run() error
}

type Set struct {
	Args struct {
		Id       string `positional-arg-name:"id" required:"true" description:"id of the wine label"`
		Location string `positional-arg-name:"location" required:"true" description:"location"`
		Long     string `positional-arg-name:"long" required:"true" description:"long"`
		Lat      string `positional-arg-name:"lat" required:"true" description:"lat"`
	} `positional-args:"true"`
	Url     string `long:"url" description:"Specify URL of REST API"`
	Keyfile string `long:"keyfile" description:"Identify file containing user's private key"`
	Wait    uint   `long:"wait" description:"Set time, in seconds, to wait for transaction to commit"`
}

func (args *Set) Name() string {
	return "set"
}

func (args *Set) KeyfilePassed() string {
	return args.Keyfile
}

func (args *Set) UrlPassed() string {
	return args.Url
}

func (args *Set) Register(parent *flags.Command) error {
	_, err := parent.AddCommand(args.Name(), "Sets an intkey value", "Sends an intkey transaction to set <name> to <value>.", args)
	if err != nil {
		return err
	}
	return nil
}

func (args *Set) Run() error {
	// Construct client
	id := args.Args.Id
	location := args.Args.Location
	long := args.Args.Long
	lat := args.Args.Lat

	wait := args.Wait

	WineLabelClient, err := GetClient(args, true)
	if err != nil {
		return err
	}
	_, err = WineLabelClient.Set(id, location, long, lat, wait)
	return err
}

func GetClient(args Command, readFile bool) (WineLabelClient, error) {
	url := args.UrlPassed()
	if url == "" {
		url = DEFAULT_URL
	}
	keyfile := ""
	if readFile {
		var err error
		keyfile, err = GetKeyfile(args.KeyfilePassed())
		if err != nil {
			return WineLabelClient{}, err
		}
	}
	return NewWineLabelClient(url, keyfile)
}
