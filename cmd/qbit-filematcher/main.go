package main

import (
	"context"
	"fmt"
	"os"

	"github.com/styper/qbit-filematcher-go/cli"
)

func main() {
	app := cli.New()
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
