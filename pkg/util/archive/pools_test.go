// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
/*
Contains code adapted from:

	https://github.com/moby/moby/blob/master/pkg/pools

Copyright 2013-2018 Docker, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	    https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/
package archive

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestBufioReaderPoolGetWithNoReaderShouldCreateOne(t *testing.T) {
	reader := BufioReader32KPool.Get(nil)
	if reader == nil {
		t.Fatalf("BufioReaderPool should have create a bufio.Reader but did not.")
	}
}

func TestBufioReaderPoolPutAndGet(t *testing.T) {
	sr := bufio.NewReader(strings.NewReader("foobar"))
	reader := BufioReader32KPool.Get(sr)
	if reader == nil {
		t.Fatalf("BufioReaderPool should not return a nil reader.")
	}
	// verify the first 3 byte
	buf1 := make([]byte, 3)
	_, err := reader.Read(buf1)
	if err != nil {
		t.Fatal(err)
	}
	if actual := string(buf1); actual != "foo" {
		t.Fatalf("The first letter should have been 'foo' but was %v", actual)
	}
	BufioReader32KPool.Put(reader)
	// Try to read the next 3 bytes
	_, err = sr.Read(make([]byte, 3))
	if err == nil || err != io.EOF {
		t.Fatalf("The buffer should have been empty, issue an EOF error.")
	}
}
