/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"fmt"
	"reflect"
	"testing"
)

type conversionTest struct {
	s, d interface{} // source and destination

	// following are used if they're non-zero
	wantint  int64
	wantuint uint64
	wantstr  string
	wanterr  string
}

// Target variables for scanning into.
var (
	scanstr    string
	scanint    int
	scanint8   int8
	scanint16  int16
	scanint32  int32
	scanuint8  uint8
	scanuint16 uint16
)

var conversionTests = []conversionTest{
	// Exact conversions (destination pointer type matches source type)
	{s: "foo", d: &scanstr, wantstr: "foo"},
	{s: 123, d: &scanint, wantint: 123},

	// To strings
	{s: []byte("byteslice"), d: &scanstr, wantstr: "byteslice"},
	{s: 123, d: &scanstr, wantstr: "123"},
	{s: int8(123), d: &scanstr, wantstr: "123"},
	{s: int64(123), d: &scanstr, wantstr: "123"},
	{s: uint8(123), d: &scanstr, wantstr: "123"},
	{s: uint16(123), d: &scanstr, wantstr: "123"},
	{s: uint32(123), d: &scanstr, wantstr: "123"},
	{s: uint64(123), d: &scanstr, wantstr: "123"},
	{s: 1.5, d: &scanstr, wantstr: "1.5"},

	// Strings to integers
	{s: "255", d: &scanuint8, wantuint: 255},
	{s: "256", d: &scanuint8, wanterr: `string "256" overflows uint8`},
	{s: "256", d: &scanuint16, wantuint: 256},
	{s: "-1", d: &scanint, wantint: -1},
	{s: "foo", d: &scanint, wanterr: `converting string "foo" to a int: parsing "foo": invalid argument`},
}

func intValue(intptr interface{}) int64 {
	return reflect.Indirect(reflect.ValueOf(intptr)).Int()
}

func uintValue(intptr interface{}) uint64 {
	return reflect.Indirect(reflect.ValueOf(intptr)).Uint()
}

func TestConversions(t *testing.T) {
	for n, ct := range conversionTests {
		err := copyConvert(ct.d, ct.s)
		errstr := ""
		if err != nil {
			errstr = err.String()
		}
		errf := func(format string, args ...interface{}) {
			base := fmt.Sprintf("copyConvert #%d: for %v (%T) -> %T, ", n, ct.s, ct.s, ct.d)
			t.Errorf(base+format, args...)
		}
		if errstr != ct.wanterr {
			errf("got error %q, want error %q", errstr, ct.wanterr)
		}
		if ct.wantstr != "" && ct.wantstr != scanstr {
			errf("want string %q, got %q", ct.wantstr, scanstr)
		}
		if ct.wantint != 0 && ct.wantint != intValue(ct.d) {
			errf("want int %d, got %d", ct.wantint, intValue(ct.d))
		}
		if ct.wantuint != 0 && ct.wantuint != uintValue(ct.d) {
			errf("want uint %d, got %d", ct.wantuint, uintValue(ct.d))
		}
	}
}

func TestNullableString(t *testing.T) {
	var ns NullableString
	copyConvert(&ns, []byte("foo"))
	if !ns.Ok {
		t.Errorf("expecting ok")
	}
	if ns.String != "foo" {
		t.Errorf("expecting foo; got %q", ns.String)
	}
	copyConvert(&ns, nil)
	if ns.Ok {
		t.Errorf("expecting not ok on nil")
	}
	if ns.String != "" {
		t.Errorf("expecting blank on nil; got %q", ns.String)
	}
}
