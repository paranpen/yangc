// A tablefile collects the produced table along with meta information.
type tablefile struct {
	buf          bytes.Buffer
	fixedNames   map[string]string // maps a fixed name back to its origial name.
	errs         []error
	messages     map[string]*messageInfo
}

// A messageInfo contains tag information about fields in a message.
type messageInfo struct {
	last   int
	fields map[string]int
}

func doTable(w io.Writer, entries []*yang.Entry) {
	for _, e := range entries {
    printTables(w, e.Node)
  }
}

// Children returns all the children nodes of e that are not RPC nodes.
func children(e *yang.Entry) []*yang.Entry {
	var names []string
	for k, se := range e.Dir {
		if se.RPC == nil {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	children := make([]*yang.Entry, len(names))
	for x, n := range names {
		children[x] = e.Dir[n]
	}
	return children
}

// printNode writes e, formatted almost like a tablebuf message, to w.
func (pf *tablefile) printNode(w io.Writer, e *yang.Entry, nest bool) {
	nodes := children(e)

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

	for i, se := range nodes {
		k := se.Name
		if se.Description != "" {
			fmt.Fprintln(indent.NewWriter(w, "  // "), se.Description)
		}
		if nest && (len(se.Dir) > 0 || se.Type == nil) {
			pf.printNode(indent.NewWriter(w, "  "), se, true)
		}
		prefix := "  "
		if se.ListAttr != nil {
			prefix = "  /* array */ "
		}
		name := pf.fieldName(k)
		printed := false
		var kind string
		if len(se.Dir) > 0 || se.Type == nil {
			kind = pf.messageName(se)
		} else if se.Type.Kind == yang.Ybits {
			values := dedup(se.Type.Bit.Values())
			asComment := false
			switch {
			case len(values) > 0 && values[len(values)-1] > 63:
				if i != 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "  // *WARNING* bitfield %s has more than 64 positions\n", name)
				kind = "uint64"
				asComment = true
			case len(values) > 0 && values[len(values)-1] > 31:
				if i != 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "  // bitfield %s to large for enum\n", name)
				kind = "uint64"
				asComment = true
			default:
				kind = pf.fixName(se.Name)
			}
			if !asComment {
				fmt.Fprintf(w, "  enum %s {\n", kind)
				fmt.Fprintf(w, "    %s_FIELD_NOT_SET = 0;\n", kind)
			} else {
				fmt.Fprintf(w, "  // Values:\n")
			}
			names := map[int64][]string{}
			for n, v := range se.Type.Bit.NameMap() {
				names[v] = append(names[v], n)
			}
			for _, v := range values {
				ns := names[v]
				sort.Strings(ns)
				if asComment {
					for _, n := range ns {
						fmt.Fprintf(w, "  //   %s = 1 << %d\n", n, v)
					}
				} else {
					n := strings.ToUpper(pf.fieldName(ns[0]))
					fmt.Fprintf(w, "    %s_%s = %d;\n", kind, n, 1<<uint(v))
					for _, n := range ns[1:] {
						n = strings.ToUpper(pf.fieldName(n))
						fmt.Fprintf(w, "    // %s = %d; (DUPLICATE VALUE)\n", n, 1<<uint(v))
					}
				}
			}
			if !asComment {
				fmt.Fprintf(w, "  };\n")
			}
		} else if se.Type.Kind == yang.Ydecimal64 {
			kind = "Decimal64"
			pf.hasDecimal64 = true
		} else if se.Type.Kind == yang.Yenum {
			kind = pf.fixName(se.Name)
			fmt.Fprintf(w, "  enum %s {", kind)
			if tableWithSource {
				fmt.Fprintf(w, " // %s", yang.Source(se.Node))
			}
			fmt.Fprintln(w)

			for i, n := range se.Type.Enum.Names() {
				fmt.Fprintf(w, "    %s_%s = %d;\n", kind, strings.ToUpper(pf.fieldName(n)), i)
			}
			fmt.Fprintf(w, "  };\n")
		} else if se.Type.Kind == yang.Yunion {
			types := pf.unionTypes(se.Type, map[string]bool{})
			switch len(types) {
			case 0:
				fmt.Fprintf(w, "    // *WARNING* union %s has no types\n", se.Name)
				printed = true
			case 1:
				kind = types[0]
			default:
				iw := w
				kind = pf.fixName(se.Name)
				if se.ListAttr != nil {
					fmt.Fprintf(w, "  message %s {\n", kind)
					iw = indent.NewWriter(w, "  ")
				}
				fmt.Fprintf(iw, "  oneof %s {", kind) // matching brace }
				if tableWithSource {
					fmt.Fprintf(w, " // %s", yang.Source(se.Node))
				}
				fmt.Fprintln(w)
				for _, tkind := range types {
					fmt.Fprintf(iw, "    %s %s_%s = %d;\n", tkind, kind, tkind, mi.tag(name, tkind, false))
				}
				// { to match the brace below to keep brace matching working
				fmt.Fprintf(iw, "  }\n")
				if se.ListAttr != nil {
					fmt.Fprintf(w, "  }\n")
				} else {
					printed = true
				}
			}
		} else {
			kind = kind2table[se.Type.Kind]
		}
		if !printed {
			fmt.Fprintf(w, "%s%s %s = %d;", prefix, kind, name, mi.tag(name, kind, se.ListAttr != nil))
			if tableWithSource {
				fmt.Fprintf(w, " // %s", yang.Source(se.Node))
			}
			fmt.Fprintln(w)
		}
	}
	// { to match the brace below to keep brace matching working
	fmt.Fprintln(w, "}")
}