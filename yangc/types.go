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
	"reflect"
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
	typesDebug = true
	typesVerbose = true
	var typesCmd = &cobra.Command{
		Use:   "types",
		Short: "yangc with types format",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doTypes(os.Stdout, entries)
		},
	}
	var enumCmd = &cobra.Command{
		Use:   "enum",
		Short: "yangc to generate C enum files",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doEnum(os.Stdout, entries)
		},
	}
	var tableCmd = &cobra.Command{
		Use:   "table",
		Short: "yangc to generate C struct files",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doTable(os.Stdout, entries)
		},
	}
	mainCmd.AddCommand(typesCmd, enumCmd, tableCmd)
}

// doTable generate enum file from node tree
func doTable(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
		if len(e.Dir) == 0 {
			// skip modules that have nothing in them
			continue
		}
		pf := &protofile{
			fixedNames: map[string]string{},
			messages:   map[string]*messageInfo{},
		}
		pf.printHeader(w, e)
		for _, child := range children(e) {
			pf.printListNodes(w, child, true)
		}
	}
}

// doEnum generate enum file from node tree
func doEnum(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
		printNodeTypedefs(w, e.Node)
	}
}

// Print type dictionary (by twkim)
func printTypedef(w io.Writer, v *yang.Typedef) {
	// case enumeration
	kind := (*v.YangType).Kind
	fmt.Fprintf(w, "%s %s {\n", kind, v.Name)
	fmt.Fprintf(w, "}\n")
}

func printEnumTypedef(w io.Writer, name string, n yang.Node) {
	fmt.Fprintf(w, "typedef enum {\n")
	printEnumType(w, n)
	fmt.Fprintf(w, "} %s;\n", name)
}

func printEnumType(w io.Writer, n yang.Node) {
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	if n.Kind() == "enum" {
		fmt.Fprintf(w, "%s = ", n.NName())
	}
Loop:
	for i := nf - 1; i >= 0; i-- {
		ft := t.Field(i)
		yangStr := ft.Tag.Get("yang")
		if yangStr == "" {
			continue
		}
		parts := strings.Split(yangStr, ",")
		for _, p := range parts[1:] {
			if p == "nomerge" {
				continue Loop
			}
		}
		if parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
			continue
		}
		f := v.Field(i)
		if !f.IsValid() || f.IsNil() {
			continue
		}
		kind := ft.Type.Kind()
		if kind == reflect.Slice {
			sl := f.Len()
			for i := 0; i < sl; i++ {
				n = f.Index(i).Interface().(yang.Node)
				if _, ok := n.(*yang.Value); !ok {
					printEnumType(indent.NewWriter(w, "    "), n)
					fmt.Fprintf(w, "\n")
				}
			}
		} else if kind == reflect.Ptr {
			// fmt.Printf("(%v , %v)", i, ft.Name)
			if ft.Name == "Description" {
				n = f.Interface().(yang.Node)
				if v, ok := n.(*yang.Value); ok {
					fmt.Fprintf(w, " // %s", v.Name)
				}
			} else if ft.Name == "Value" {
				n = f.Interface().(yang.Node)
				if v, ok := n.(*yang.Value); ok {
					fmt.Fprintf(w, "%s ", v.Name)
				}
			}
		}
	}
}

// printTypedefs prints node n to w, recursively.
// TODO(borman): display more information
func printNodeTypedefs(w io.Writer, n yang.Node) {
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	// fmt.Fprintf(w, "%s [%s]\n", n.NName(), n.Kind())
	switch n.Kind() {
	case "module":
		fmt.Fprintf(w, "// %s.h C enum file generated by Yang Compiler\n", n.NName())
	case "typedef":
		name := n.NName()
		n = yang.ChildNode(n, "enumeration")
		if n != nil {
			printEnumTypedef(w, name, n)
		}
		return
	case "enum":
		fmt.Fprintf(w, "%s = ", n.NName())
	case "leaf":
		name := n.NName()
		n = yang.ChildNode(n, "enumeration")
		if n != nil {
			printEnumTypedef(w, name, n)
		}
		return
	case "list":
		fmt.Fprintf(w, "struct %s { }\n", n.NName())
	}

Loop:
	for i := 1; i <= nf; i++ {
		ft := t.Field(nf - i)
		yangStr := ft.Tag.Get("yang")
		if yangStr == "" {
			continue
		}
		parts := strings.Split(yangStr, ",")
		for _, p := range parts[1:] {
			if p == "nomerge" {
				continue Loop
			}
		}

		// Skip uppercase elements.
		if parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
			continue
		}

		f := v.Field(nf - i)
		if !f.IsValid() || f.IsNil() {
			continue
		}
		switch ft.Type.Kind() {
		case reflect.Ptr:
			n = f.Interface().(yang.Node)
			if v, ok := n.(*yang.Value); ok {
				if ft.Name == "Description" {
					fmt.Fprintf(w, "// %s\n", v.Name)
				} else {
					fmt.Fprintf(w, "%s, ", v.Name)
				}
			} else {
				printNodeTypedefs(indent.NewWriter(w, "    "), n)
			}
		case reflect.Slice:
			sl := f.Len()
			for i := 0; i < sl; i++ {
				n = f.Index(i).Interface().(yang.Node)
				if v, ok := n.(*yang.Value); ok {
					if ft.Name == "Description" {
						fmt.Fprintf(w, "// %s\n", v.Name)
					} else {
						fmt.Fprintf(w, "%s[%d] = %s\n", ft.Name, i, v.Name)
					}
				} else {
					printNodeTypedefs(indent.NewWriter(w, "    "), n)
				}
			}
		}
	}
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

// kind2proto maps base yang types to protocol buffer types.
// TODO(borman): do TODO types.
var kind2header = map[yang.TypeKind]string{
	yang.Yint8:   "int32",  // int in range [-128, 127]
	yang.Yint16:  "int32",  // int in range [-32768, 32767]
	yang.Yint32:  "int32",  // int in range [-2147483648, 2147483647]
	yang.Yint64:  "int64",  // int in range [-9223372036854775808, 9223372036854775807]
	yang.Yuint8:  "uint32", // int in range [0, 255]
	yang.Yuint16: "uint32", // int in range [0, 65535]
	yang.Yuint32: "uint32", // int in range [0, 4294967295]
	yang.Yuint64: "uint64", // int in range [0, 18446744073709551615]

	yang.Ybinary:             "bytes",       // arbitrary data
	yang.Ybits:               "INLINE-bits", // set of bits or flags
	yang.Ybool:               "bool",        // true or false
	yang.Ydecimal64:          "INLINE-d64",  // signed decimal number
	yang.Yempty:              "bool",        // value is its presense
	yang.Yenum:               "enum",        // enumerated strings
	yang.Yidentityref:        "string",      // reference to abstract identity
	yang.YinstanceIdentifier: "string",      // reference of a data tree node
	yang.Yleafref:            "string",      // reference to a leaf instance
	yang.Ystring:             "string",      // human readable string
	yang.Yunion:              "union",       // handled inline
}

// C Struct generation from Yang List
// printListNodes print list nodes (by twkim)
func (pf *protofile) printListNodes(w io.Writer, e *yang.Entry, nest bool) {
	if e.Description != "" {
		fmt.Fprintln(indent.NewWriter(w, "// "), e.Description)
	}

	messageName := pf.fullName(e)
	mi := pf.messages[messageName]
	if mi == nil {
		mi = &messageInfo{
			fields: map[string]int{},
		}
		pf.messages[messageName] = mi
	}

	fmt.Fprintf(w, "struct %s {\n", pf.messageName(e)) // matching brace }

	nodes := children(e)
	for _, se := range nodes {
		k := se.Name
		if se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		if nest && (len(se.Dir) > 0 || se.Type == nil) {
			pf.printNode(indent.NewWriter(w, "  "), se, true)
		}
		name := pf.fieldName(k)
		kind := kind2header[se.Type.Kind]
		fmt.Fprintf(w, "  %s %s;", kind, name)
		fmt.Fprintln(w)
	}

	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}
