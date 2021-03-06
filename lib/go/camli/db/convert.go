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

// Type conversions for Scan.

package db

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// copyConvert copies to dest the value in src, converting it if possible
// An error is returned if the copy would result in loss of information.
// dest should be a pointer type.
func copyConvert(dest, src interface{}) os.Error {
	// Common cases, without reflect.  Fall through.
	switch s := src.(type) {
	case string:
		switch d := dest.(type) {
		case *string:
			*d = s
			return nil
		}
	case []byte:
		switch d := dest.(type) {
		case *string:
			*d = string(s)
			return nil
		case *[]byte:
			*d = s
			return nil
		}
	}

	sv := reflect.ValueOf(src)

	switch d := dest.(type) {
	case *string:
		switch sv.Kind() {
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			*d = fmt.Sprintf("%v", src)
			return nil
		}
	}

	if scanner, ok := dest.(ScannerInto); ok {
		return scanner.ScanInto(src)
	}

	dpv := reflect.ValueOf(dest)
	if dpv.Kind() != reflect.Ptr {
		return os.NewError("destination not a pointer")
	}

	dv := reflect.Indirect(dpv)
	if dv.Kind() == sv.Kind() {
		dv.Set(sv)
		return nil
	}

	switch dv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if s, ok := asString(src); ok {
			i64, err := strconv.Atoi64(s)
			if err != nil {
				return fmt.Errorf("converting string %q to a %s: %v", s, dv.Kind(), err)
			}
			if dv.OverflowInt(i64) {
				return fmt.Errorf("string %q overflows %s", s, dv.Kind())
			}
			dv.SetInt(i64)
			return nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if s, ok := asString(src); ok {
			u64, err := strconv.Atoui64(s)
			if err != nil {
				return fmt.Errorf("converting string %q to a %s: %v", s, dv.Kind(), err)
			}
			if dv.OverflowUint(u64) {
				return fmt.Errorf("string %q overflows %s", s, dv.Kind())
			}
			dv.SetUint(u64)
			return nil
		}
	}

	return fmt.Errorf("unsupported driver -> Scan pair: %T -> %T", src, dest)
}

func asString(src interface{}) (s string, ok bool) {
	switch v := src.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	}
	return "", false
}
