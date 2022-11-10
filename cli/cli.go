package cli

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
)

var ErrLog = log.New(os.Stderr, "gotem", log.LstdFlags|log.Lshortfile)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func Run(args []string) error {
	rootCmd := ffcli.Command{
		Name:       "go-templater",
		ShortUsage: "gotem [flags] <subcommand> [command flags]",
		ShortHelp:  "Make initiating project more easier",
		Subcommands: []*ffcli.Command{
			versionCmd,
			genCmd,
		},
		FlagSet: genCmd.FlagSet,
		Exec:    genCmd.Exec,
	}

	if err := rootCmd.Parse(args); err != nil {
		return err
	}

	return rootCmd.Run(context.Background())
}
