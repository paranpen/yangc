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

package yang

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// These are the default ranges defined by the YANG standard.
var (
	Int8Range  = mustParseRanges("-128..127")
	Int16Range = mustParseRanges("-32768..32767")
	Int32Range = mustParseRanges("-2147483648..2147483647")
	Int64Range = mustParseRanges("-9223372036854775808..9223372036854775807")

	Uint8Range  = mustParseRanges("0..255")
	Uint16Range = mustParseRanges("0..65535")
	Uint32Range = mustParseRanges("0..4294967295")
	Uint64Range = mustParseRanges("0..18446744073709551615")

	Decimal64Range = mustParseRanges("min..max")
)

const (
	MaxInt64        = 1<<63 - 1 // maximum value of a signed int64
	MinInt64        = -1 << 63  // minimum value of a signed int64
	AbsMinInt64     = 1 << 63   // the absolute value of MinInt64
	MaxEnum         = 1<<31 - 1 // maximum value of an enum
	MinEnum         = -1 << 31  // minimum value of an enum
	MaxBitfieldSize = 1 << 32   // maximum number of bits in a bitfield
)

type NumberKind int

const (
	Positive  = NumberKind(iota) // Number is non-negative
	Negative                     // Number is negative
	MinNumber                    // Number is minimum value allowed for range
	MaxNumber                    // Number is maximum value allowed for range
)

// A Number is in the range of [-(1<<64) - 1, (1<<64)-1].  This range
// is the union of uint64 and int64.
// to indicate
type Number struct {
	Kind    NumberKind
	Value   uint64
	Decimal float64
}

var maxNumber = Number{Kind: MaxNumber}
var minNumber = Number{Kind: MinNumber}

// FromInt creates a Number from an int64.
func FromInt(i int64) Number {
	if i < 0 {
		return Number{Kind: Negative, Value: uint64(-i)}
	}
	return Number{Value: uint64(i)}
}

// FromUint creates a Number from a uint64.
func FromUint(i uint64) Number {
	return Number{Value: i}
}

// ParseNumber returns s as a Number.  Numbers may be represented in decimal,
// octal, or hexidecimal using the standard prefix notations (e.g., 0 and 0x)
func ParseNumber(s string) (n Number, err error) {
	s = strings.TrimSpace(s)
	switch s {
	case "max":
		return maxNumber, nil
	case "min":
		return minNumber, nil
	case "":
		return n, errors.New("converting empty string to number")
	case "+", "-":
		return n, errors.New("sign with no value")
	}

	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		n.Kind = Negative
		s = s[1:]
	}
	//n.Value, err = strconv.ParseUint(s, 0, 64)
	// handle Decimal Type
	parts := strings.Split(s, ".")
	if len(parts) == 1 {
		// There is no decimal point, we can just parse the original string as
		// an int
		n.Value, err = strconv.ParseUint(s, 0, 64)
		return n, err
	} else if len(parts) > 2 {
		return n, errors.New("can't convert to decimal: too many .s")
	}

	// strip the insignificant digits for more accurate comparisons.
	dec, err := strconv.ParseFloat(s, 64)

	n.Value = uint64(dec)
	n.Decimal = dec
	return n, err
}

// String returns n as a string in decimal.
func (n Number) String() string {
	switch n.Kind {
	case Negative:
		return "-" + strconv.FormatUint(n.Value, 10)
	case MinNumber:
		return "min"
	case MaxNumber:
		return "max"
	}
	return strconv.FormatUint(n.Value, 10)
}

// Int returns n as an int64.  It returns an error if n overflows an int64.
func (n Number) Int() (int64, error) {
	switch n.Kind {
	case MinNumber:
		return MinInt64, nil
	case MaxNumber:
		return MaxInt64, nil
	case Negative:
		switch {
		case n.Value == AbsMinInt64:
			return MinInt64, nil
		case n.Value < AbsMinInt64:
			return -int64(n.Value), nil
		}
	default:
		if n.Value <= MaxInt64 {
			return int64(n.Value), nil
		}
	}
	return 0, errors.New("signed integer overflow")
}

// add adds i to n without checking overflow.  We really only need to be
// able to add 1 for our code.
func (n Number) add(i uint64) Number {
	switch n.Kind {
	case MinNumber:
		return n
	case MaxNumber:
		return n
	case Negative:
		if n.Value <= i {
			n.Value = i - n.Value
			n.Kind = Positive
		} else {
			n.Value -= i
		}
	default:
		n.Value += i
	}
	return n
}

// Less returns true if n is less than m.
func (n Number) Less(m Number) bool {
	switch {
	case m.Kind == MinNumber:
		return false
	case n.Kind == MinNumber:
		return true
	case n.Kind == MaxNumber:
		return false
	case m.Kind == MaxNumber:
		return true
	case n.Kind == Negative && m.Kind != Negative:
		return true
	case n.Kind != Negative && m.Kind == Negative:
		return false
	case n.Kind == Negative:
		return n.Value > m.Value
	default:
		return n.Value < m.Value
	}
}

// Equal returns true if m equals n.  It provides symmetry with the Less
// method.
func (n Number) Equal(m Number) bool {
	return n == m
}

// YRange is a single range of consecutive numbers, inclusive.
type YRange struct {
	Min Number
	Max Number
}

// Valid returns false if r is not a valid range (min > max).
func (r YRange) Valid() bool {
	return !r.Max.Less(r.Min)
}

// A YangRange is a set of non-overlapping ranges.
type YangRange []YRange

// ParseRanges parses s into a series of ranges.  Each individual range is
// in s is separated by the pipe character (|).  The min and max value of
// a range are separated by "..".  An error is returned if the range is
// invalid.  The resulting range is sorted and coalesced.
func ParseRanges(s string) (YangRange, error) {
	parts := strings.Split(s, "|")
	r := make(YangRange, len(parts))
	for i, s := range parts {
		parts := strings.Split(s, "..")
		min, err := ParseNumber(parts[0])
		if err != nil {
			return nil, err
		}
		var max Number
		switch len(parts) {
		case 1:
			max = min
		case 2:
			max, err = ParseNumber(parts[1])
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("two many ..'s in %s", s)
		}
		if max.Less(min) {
			return nil, fmt.Errorf("%s less than %s", max, min)
		}
		r[i] = YRange{min, max}
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}

	return coalesce(r), nil
}

// coalesce coalesces r into as few ranges as possible.  For example,
// 1..5|6..10 would become 1..10.  r is assumed to be sorted.
// r is assumed to be valid (see Validate)
func coalesce(r YangRange) YangRange {
	// coalesce the ranges if we have more than 1.
	if len(r) < 2 {
		return r
	}
	cr := make(YangRange, len(r))
	i := 0
	cr[i] = r[0]
	for _, r1 := range r[1:] {
		// r1.Min is always at least as large as cr[i].Min
		// Cases are:
		// r1 is contained in cr[i]
		// r1 starts inside of cr[i]
		// r1.Min cr[i].Max+1
		// r1 is beyond cr[i]
		if cr[i].Max.add(1).Less(r1.Min) {
			// r1 starts after cr[i], this is a new range
			i++
			cr[i] = r1
		} else if cr[i].Max.Less(r1.Max) {
			cr[i].Max = r1.Max
		}
	}
	return cr[:i+1]
}

func mustParseRanges(s string) YangRange {
	r, err := ParseRanges(s)
	if err != nil {
		panic(err)
	}
	return r
}

// String returns r as a string using YANG notation, either a simple
// value if min == max or min..max.
func (r YRange) String() string {
	if r.Min.Equal(r.Max) {
		return r.Min.String()
	}
	return r.Min.String() + ".." + r.Max.String()
}

// String returns the ranges r using YANG notation.  Individual ranges
// are separated by pipes (|).
func (r YangRange) String() string {
	s := make([]string, len(r))
	for i, r := range r {
		s[i] = r.String()
	}
	return strings.Join(s, "|")
}

func (r YangRange) Len() int      { return len(r) }
func (r YangRange) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r YangRange) Less(i, j int) bool {
	switch {
	case r[i].Min.Less(r[j].Min):
		return true
	case r[j].Min.Less(r[i].Min):
		return false
	default:
		return r[i].Max.Less(r[j].Max)
	}
}

// Validate sorts r and returns an error if r has either an invalid range or has
// overlapping ranges.
func (r YangRange) Validate() error {
	sort.Sort(r)
	switch {
	case len(r) == 0:
		return nil
	case !r[0].Valid():
		return errors.New("invalid number")
	}
	p := r[0]

	for _, n := range r[1:] {
		if n.Min.Less(p.Max) {
			return errors.New("overlapping ranges")
		}
	}
	return nil
}

// Equal returns true if ranges r and q are identically equivalent.
// TODO(borman): should we coalesce ranges in the comparison?
func (r YangRange) Equal(q YangRange) bool {
	if len(r) != len(q) {
		return false
	}
	for i, r := range r {
		if r != q[i] {
			return false
		}
	}
	return true
}

// Contains returns true if all possible values in s are also possible values
// in r.  An empty range is assumed to be min..max.
func (r YangRange) Contains(s YangRange) bool {
	if len(s) == 0 || len(r) == 0 {
		return true
	}

	rc := make(chan YRange)
	go func() {
		for _, v := range r {
			rc <- v
		}
		close(rc)
	}()

	// All ranges are sorted and coalesced which means each range
	// in s must exist

	// We know rc will always produce at least one value
	rr, ok := <-rc
	for _, ss := range s {
		// min is always within range
		if ss.Min.Kind != MinNumber {
			for rr.Max.Less(ss.Min) {
				rr, ok = <-rc
				if !ok {
					return false
				}
			}
		}
		if (ss.Max.Kind == MaxNumber) || (ss.Min.Kind == MinNumber) {
			continue
		}
		if ss.Min.Less(rr.Min) || rr.Max.Less(ss.Max) {
			return false
		}
	}
	return true
}
