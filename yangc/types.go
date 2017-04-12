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

// kind2header maps base yang types to C types.
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

func init() {
	var headerCmd = &cobra.Command{
		Use:   "header",
		Short: "yangc go generate all types in C format",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doHeader(os.Stdout, entries)
		},
	}
	mainCmd.AddCommand(headerCmd)
	var typeCmd = &cobra.Command{
		Use:   "type",
		Short: "yangc to generate enum types in C format",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doType(os.Stdout, entries)
		},
	}
	var tableCmd = &cobra.Command{
		Use:   "table",
		Short: "yangc to generate table struct in C format",
		Run: func(cmd *cobra.Command, args []string) {
			entries := doCompile(yangFileName)
			doTable(os.Stdout, entries)
		},
	}
	headerCmd.AddCommand(typeCmd, tableCmd)
}

// doHeader generate all types from entries tree
func doHeader(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
		if len(e.Dir) == 0 {
			continue // skip modules that have nothing in them
		}
		pf := &protofile{
			fixedNames: map[string]string{},
			messages:   map[string]*messageInfo{},
		}
		pf.printHeader(w, e, false)
		for _, se := range e.Dir {
			pf.WriteHeaders(w, se, true, true)
		}
	}
	/* types := Types{}
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
	} */
}

// doEnum generate enum file from node tree
func doType(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
		if len(e.Dir) == 0 {
			continue // skip modules that have nothing in them
		}
		pf := &protofile{
			fixedNames: map[string]string{},
			messages:   map[string]*messageInfo{},
		}
		pf.printHeader(w, e, false)
		for _, se := range e.Dir {
			pf.WriteHeaders(w, se, true, false)
		}
	}
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
		pf.printHeader(w, e, false)
		for _, se := range e.Dir {
			pf.WriteHeaders(w, se, false, true)
		}
	}
}

// Children returns all the children nodes of e that are not RPC nodes.
func childrenEntries(e *yang.Entry) []*yang.Entry {
	var names []string
	for k, se := range e.Dir {
		if se.RPC == nil {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		return nil
	}
	// sort.Strings(names)
	children := make([]*yang.Entry, len(names))
	for x, n := range names {
		children[x] = e.Dir[n]
	}
	return children
}

// WriteTypedefs print all typedefs
func (pf *protofile) WriteHeaders(w io.Writer, e *yang.Entry, typePrint bool, listPrint bool) {
	messageName := pf.fullName(e)
	mi := pf.messages[messageName]
	if mi == nil {
		mi = &messageInfo{
			fields: map[string]int{},
		}
		pf.messages[messageName] = mi
	}

	if e.GetKind() == "Typedef" {
		if typePrint {
			if e.Description != "" {
				fmt.Fprintln(indent.NewWriter(w, "\n// "), e.Description)
			}
			fmt.Fprintf(w, "typedef %s {\n", pf.messageName(e)) // matching brace }
			printNodeTypedef(w, e.Node)
			fmt.Fprintf(w, "}\n") // { to match the brace below to keep brace matching working
		}
		return
	}

	if listPrint {
		if e.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "\n// "), e.Description)
		}
		fmt.Fprintf(w, "struct %s {\n", pf.messageName(e)) // matching brace }
	}

	nodes := childrenEntries(e)
	for _, se := range nodes {
		var kind string
		if se.Type.Kind == yang.Yenum {
			if typePrint {
				kind = pf.fixName(se.Name)
				fmt.Fprintf(w, "  enum %s {", kind)
				if protoWithSource {
					fmt.Fprintf(w, " // %s", yang.Source(se.Node))
				}
				fmt.Fprintln(w)

				for i, n := range se.Type.Enum.Names() {
					fmt.Fprintf(w, "    %s_%s = %d;\n", kind, strings.ToUpper(pf.fieldName(n)), i)
				}
				fmt.Fprintf(w, "  };\n")
			}
		} else {
			if listPrint {
				if se.Description != "" {
					fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
				}
				if len(se.Dir) > 0 || se.Type == nil {
					pf.WriteHeaders(indent.NewWriter(w, "  "), se, typePrint, listPrint)
				}
				if len(se.Dir) > 0 || se.Type == nil {
					kind = pf.messageName(se)
				} else {
					kind = kind2proto[se.Type.Kind]
				}
				k := se.Name
				name := pf.fieldName(k)
				fmt.Fprintf(w, "%s %s = %d;\n", kind, name, mi.tag(name, kind, se.ListAttr != nil))
			}
		}
	}
	if listPrint {
		fmt.Fprintln(w, "}") // { to match the brace below to keep brace matching working
	}
}

// printTypedefs prints node n to w, recursively.
// TODO(borman): display more information
func printNodeTypedef(w io.Writer, n yang.Node) {
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	fmt.Fprintf(w, "%s [%s]\n", n.NName(), n.Kind())
	switch n.Kind() {
	case "module":
		fmt.Fprintf(w, "// %s.h C enum file generated by Yang Compiler\n", n.NName())
	case "typedef":
		n = yang.ChildNode(n, "enumeration")
		if n != nil {
			printEnumType(w, n)
		}
		return
	case "enum":
		fmt.Fprintf(w, "%s = ", n.NName())
	case "leaf":
		n = yang.ChildNode(n, "enumeration")
		if n != nil {
			printEnumType(w, n)
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
				printNodeTypedef(indent.NewWriter(w, "    "), n)
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
					printNodeTypedef(indent.NewWriter(w, "    "), n)
				}
			}
		}
	}
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

/* printType prints type t in a moderately human readable format to w.
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
} */
