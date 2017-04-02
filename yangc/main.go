package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"github.com/paranpen/yangc/pkg/yang"
	"github.com/spf13/cobra"
)

var mainCmd = &cobra.Command{
	Use:   os.Args[0],
	Short: "Tool to translate Yang Models to Unit Data API",
}

var yangFileName string

func init() {
	mainCmd.PersistentFlags().StringVarP(&yangFileName, "file", "f", "test.yang", "yang file name")
}

func main() {
	if _, err := mainCmd.ExecuteC(); err != nil {
		os.Exit(-1)
	}
}

func doCompile(fileName string) []*yang.Entry {
	var entries []*yang.Entry

	ms := yang.NewModules()
	files := make([]string, 0, 10)
	files = append(files, fileName)

	if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err == nil {
			err = ms.Parse(string(data), "<STDIN>")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	for _, name := range files {
		if err := ms.Read(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
	}

	// Process the read files, exiting if any errors were found.
	exitIfError(ms.Process())

	// Keep track of the top level modules we read in.
	// Those are the only modules we want to print below.
	mods := map[string]*yang.Module{}
	var names []string

	for _, m := range ms.Modules {
		if mods[m.Name] == nil {
			mods[m.Name] = m
			names = append(names, m.Name)
		}
	}
	sort.Strings(names)
	entries = make([]*yang.Entry, len(names))
	for x, n := range names {
		// yang.PrintNode(os.Stdout, mods[n])
		entries[x] = yang.ToEntry(mods[n])
	}
	return entries
}

// exitIfError writes errs to standard error and exits with an exit status of 1.
// If errs is empty then exitIfError does nothing and simply returns.
func exitIfError(errs []error) {
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
