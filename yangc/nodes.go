package main

import (
	"os"

	"github.com/paranpen/yangc/pkg/yang"
	"github.com/spf13/cobra"
)

func init() {
	var nodesCmd = &cobra.Command{
		Use:   "nodes",
		Short: "print nodes",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			for _, e := range entries {
				yang.PrintNode(os.Stdout, e.Node)
			}
		},
	}
	mainCmd.AddCommand(nodesCmd)
}
