package cli

import (
	"context"
	"fmt"

	"github.com/dotdak/go-templater/version"
	"github.com/peterbourgon/ff/v3/ffcli"
)

var versionCmd = &ffcli.Command{
	Name:       "version",
	ShortUsage: "version [flags]",
	ShortHelp:  "Print go-templater version",
	FlagSet:    newFlagSet("version"),
	Exec: func(ctx context.Context, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("too many non-flag arguments: %q", args)
		}

		fmt.Printf("Version %s\n", version.GitCommit)
		return nil
	},
}
