package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/restic/restic/internal/errors"
	"github.com/restic/restic/internal/restic"
	"github.com/restic/restic/internal/walker"
)

var cmdLs = &cobra.Command{
	Use:   "ls [flags] [snapshot-ID] [dir...]",
	Short: "List files in a snapshot",
	Long: `
The "ls" command allows listing files and directories in a snapshot.

The special snapshot-ID "latest" can be used to list files and directories of the latest snapshot in the repository.
`,
	DisableAutoGenTag: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLs(lsOptions, globalOptions, args)
	},
}

// LsOptions collects all options for the ls command.
type LsOptions struct {
	ListLong  bool
	Host      string
	Tags      restic.TagLists
	Paths     []string
	Recursive bool
}

var lsOptions LsOptions

func init() {
	cmdRoot.AddCommand(cmdLs)

	flags := cmdLs.Flags()
	flags.BoolVarP(&lsOptions.ListLong, "long", "l", false, "use a long listing format showing size and mode")

	flags.StringVarP(&lsOptions.Host, "host", "H", "", "only consider snapshots for this `host`, when no snapshot ID is given")
	flags.Var(&lsOptions.Tags, "tag", "only consider snapshots which include this `taglist`, when no snapshot ID is given")
	flags.StringArrayVar(&lsOptions.Paths, "path", nil, "only consider snapshots which include this (absolute) `path`, when no snapshot ID is given")
	flags.BoolVar(&lsOptions.Recursive, "recursive", false, "include files in subfolders of the listed directories")
}

func runLs(opts LsOptions, gopts GlobalOptions, args []string) error {
	if len(args) == 0 && opts.Host == "" && len(opts.Tags) == 0 && len(opts.Paths) == 0 {
		return errors.Fatal("Invalid arguments, either give one or more snapshot IDs or set filters.")
	}

	repo, err := OpenRepository(gopts)
	if err != nil {
		return err
	}

	if err = repo.LoadIndex(gopts.ctx); err != nil {
		return err
	}

	// extract any specific directories to walk
	dirs := args[1:]

	ctx, cancel := context.WithCancel(gopts.ctx)
	defer cancel()
	for sn := range FindFilteredSnapshots(ctx, repo, opts.Host, opts.Tags, opts.Paths, args[:1]) {
		Verbosef("snapshot %s of %v at %s):\n", sn.ID().Str(), sn.Paths, sn.Time)

		err := walker.Walk(ctx, repo, *sn.Tree, nil, func(nodepath string, node *restic.Node, err error) (bool, error) {
			if err != nil {
				return false, err
			}
			if node == nil {
				return false, nil
			}

			// apply any directory filters
			if len(dirs) > 0 {
				var nodeDir string
				if !opts.Recursive {
					// only needed for exact directory match; i.e. no subfolders
					nodeDir = filepath.Dir(nodepath)
				}
				var match bool
				for _, dir := range dirs {
					if opts.Recursive {
						if strings.HasPrefix(nodepath, dir) {
							match = true
							break
						}
					} else {
						if nodeDir == dir {
							match = true
							break
						}
					}
				}
				if !match {
					return true, nil
				}
			}

			Printf("%s\n", formatNode(nodepath, node, lsOptions.ListLong))
			return false, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
