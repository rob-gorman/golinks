package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/rob-gorman/golinks-dns/internal/log"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	flags := flag.NewFlagSet(args[0], flag.ExitOnError)
	/*
		define flags
	*/
	flags.Parse(args[1:])

	log := log.Default().NewWithContext()
	log.Infow("starting server", "args", args)

	// handle unrecognized arguments
	if flags.NArg() > 0 {
		// return fmt.Errorf("unrecognized arguments: %v", flags.Args()) // or however you want to handle it
	}

	return nil
}
