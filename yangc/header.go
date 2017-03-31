package main

import (
	"fmt"
	"io"
	"os"

	"github.com/paranpen/yangc/pkg/yang"
	"github.com/spf13/cobra"
)

func init() {
	var headerCmd = &cobra.Command{
		Use:   ".",
		Short: "yangc to generate C header files",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doHeader(os.Stdout, entries)
		},
	}
	mainCmd.AddCommand(headerCmd)
}

func doHeader(w io.Writer, entries []*yang.Entry) {
	// do stuff
	tds := yang.TypeDict.Typedefs()
	printTypedefs(tds) // look up by nodes?
}

// Print type dictionary (by twkim)
func printTypedefs(tds []*yang.Typedef) {
	for _, v := range tds {
		// case enumeration
		kind := (*v.YangType).Kind
		fmt.Printf("%s %s {\n", kind, v.Name)
		fmt.Printf("}\n")
	}
}
