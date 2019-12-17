package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/urfave/cli"
)

const (
	dirArgName = "dir"
)

func main() {
	app := cli.NewApp()
	app.Name = "zapdir"
	app.Usage = "Delete a directory"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  dirArgName,
			Value: "",
			Usage: "Directory to delete",
		},
	}

	app.Action = func(c *cli.Context) error {
		dir := c.String(dirArgName)

		if dir == "" {
			return errors.New("dir must be supplied")
		}

		// DestroyLayer requires an absolute path.
		dir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}

		if _, err := os.Stat(dir); err != nil {
			return err
		}

		if err := wclayer.DestroyLayer(dir); err != nil {
			return err
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(cli.ErrWriter, err)
		os.Exit(1)
	}
}
