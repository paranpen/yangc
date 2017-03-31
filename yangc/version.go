package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	mainCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of yangc",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("yangc Unit Data API Generator v0.1")
	},
}
