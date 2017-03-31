// Copyright 2015 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paranpen/yangc/pkg/indent"
	"github.com/paranpen/yangc/pkg/yang"
	"github.com/spf13/cobra"
)

var (
	typesDebug   bool
	typesVerbose bool
)

func init() {
	typesVerbose = true
	var typesCmd = &cobra.Command{
		Use:   "types",
		Short: "yangc with types format",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doTypes(os.Stdout, entries)
		},
	}
	mainCmd.AddCommand(typesCmd)
}

func doTypes(w io.Writer, entries []*yang.Entry) {
	types := Types{}
	for _, e := range entries {
		types.AddEntry(e)
	}

	for t := range types {
		printType(w, t, typesVerbose)
	}
	if typesDebug {
		for _, e := range entries {
			showall(w, e)
		}
	}
}

// Types keeps track of all the YangTypes defined.
type Types map[*yang.YangType]struct{}

// AddEntry adds all types defined in e and its decendents to t.
func (t Types) AddEntry(e *yang.Entry) {
	if e == nil {
		return
	}
	if e.Type != nil {
		t[e.Type.Root] = struct{}{}
	}
	for _, d := range e.Dir {
		t.AddEntry(d)
	}
}

// printType prints type t in a moderately human readable format to w.
func printType(w io.Writer, t *yang.YangType, verbose bool) {
	if verbose && t.Base != nil {
		base := yang.Source(t.Base)
		if base == "unknown" {
			base = "unnamed type"
		}
		fmt.Fprintf(w, "%s: ", base)
	}
	fmt.Fprintf(w, "%s", t.Root.Name)
	if t.Kind.String() != t.Root.Name {
		fmt.Fprintf(w, "(%s)", t.Kind)
	}
	if t.Units != "" {
		fmt.Fprintf(w, " units=%s", t.Units)
	}
	if t.Default != "" {
		fmt.Fprintf(w, " default=%q", t.Default)
	}
	if t.FractionDigits != 0 {
		fmt.Fprintf(w, " fraction-digits=%d", t.FractionDigits)
	}
	if len(t.Length) > 0 {
		fmt.Fprintf(w, " length=%s", t.Length)
	}
	if t.Kind == yang.YinstanceIdentifier && !t.OptionalInstance {
		fmt.Fprintf(w, " required")
	}
	if t.Kind == yang.Yleafref && t.Path != "" {
		fmt.Fprintf(w, " path=%q", t.Path)
	}
	if len(t.Pattern) > 0 {
		fmt.Fprintf(w, " pattern=%s", strings.Join(t.Pattern, "|"))
	}
	b := yang.BaseTypedefs[t.Kind.String()].YangType
	if len(t.Range) > 0 && !t.Range.Equal(b.Range) {
		fmt.Fprintf(w, " range=%s", t.Range)
	}
	if len(t.Type) > 0 {
		fmt.Fprintf(w, "union{\n")
		for _, t := range t.Type {
			printType(indent.NewWriter(w, "  "), t, verbose)
		}
		fmt.Fprintf(w, "}")
	}
	fmt.Fprintf(w, ";\n")
}

func showall(w io.Writer, e *yang.Entry) {
	if e == nil {
		return
	}
	if e.Type != nil {
		fmt.Fprintf(w, "\n%s\n  ", e.Node.Statement().Location())
		printType(w, e.Type.Root, false)
	}
	for _, d := range e.Dir {
		showall(w, d)
	}
}