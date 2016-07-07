// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multicorecsv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"math/rand"
	"testing"
)

var writeTests = []struct {
	Input   [][]string
	Output  string
	UseCRLF bool
}{
	{Input: [][]string{{"abc"}}, Output: "abc\n"},
	{Input: [][]string{{"abc"}}, Output: "abc\r\n", UseCRLF: true},
	{Input: [][]string{{`"abc"`}}, Output: `"""abc"""` + "\n"},
	{Input: [][]string{{`a"b`}}, Output: `"a""b"` + "\n"},
	{Input: [][]string{{`"a"b"`}}, Output: `"""a""b"""` + "\n"},
	{Input: [][]string{{" abc"}}, Output: `" abc"` + "\n"},
	{Input: [][]string{{"abc,def"}}, Output: `"abc,def"` + "\n"},
	{Input: [][]string{{"abc", "def"}}, Output: "abc,def\n"},
	{Input: [][]string{{"abc"}, {"def"}}, Output: "abc\ndef\n"},
	{Input: [][]string{{"abc\ndef"}}, Output: "\"abc\ndef\"\n"},
	{Input: [][]string{{"abc\ndef"}}, Output: "\"abc\r\ndef\"\r\n", UseCRLF: true},
	{Input: [][]string{{"abc\rdef"}}, Output: "\"abcdef\"\r\n", UseCRLF: true},
	{Input: [][]string{{"abc\rdef"}}, Output: "\"abc\rdef\"\n", UseCRLF: false},
	{Input: [][]string{{""}}, Output: "\n"},
	{Input: [][]string{{"", ""}}, Output: ",\n"},
	{Input: [][]string{{"", "", ""}}, Output: ",,\n"},
	{Input: [][]string{{"", "", "a"}}, Output: ",,a\n"},
	{Input: [][]string{{"", "a", ""}}, Output: ",a,\n"},
	{Input: [][]string{{"", "a", "a"}}, Output: ",a,a\n"},
	{Input: [][]string{{"a", "", ""}}, Output: "a,,\n"},
	{Input: [][]string{{"a", "", "a"}}, Output: "a,,a\n"},
	{Input: [][]string{{"a", "a", ""}}, Output: "a,a,\n"},
	{Input: [][]string{{"a", "a", "a"}}, Output: "a,a,a\n"},
	{Input: [][]string{{`\.`}}, Output: "\"\\.\"\n"},
}

func TestWrite(t *testing.T) {
	for n, tt := range writeTests {
		b := &bytes.Buffer{}
		f := NewWriter(b)
		f.UseCRLF = tt.UseCRLF
		err := f.WriteAll(tt.Input)
		if err != nil {
			t.Errorf("Unexpected error: %s\n", err)
		}
		err = f.Close()
		if err != nil {
			t.Errorf("Unexpected error: %s\n", err)
		}
		out := b.String()
		if out != tt.Output {
			t.Errorf("#%d: out=%q want %q", n, out, tt.Output)
		}
	}
}

type errorWriter struct{}

func (e errorWriter) Write(b []byte) (int, error) {
	return 0, errors.New("Test")
}

func TestError(t *testing.T) {
	b := &bytes.Buffer{}
	f := NewWriter(b)
	f.Write([]string{"abc"})
	f.Flush()
	err := f.Error()

	if err != nil {
		t.Errorf("Unexpected error: %s\n", err)
	}

	f = NewWriter(errorWriter{})
	f.Write([]string{"abc"})
	f.Flush()
	err = f.Error()
	if err == nil {
		t.Error("Error should not be nil")
	} else if want, got := "Test", err.Error(); want != got {
		t.Errorf("Wanted %s, got %s", want, got)
	}
}

func benchmarkWrite(b *testing.B, chunkSize int) {
	ir := &infiniteWriter{}
	writer := NewWriterSized(ir, chunkSize)
	writer.Comma = '\t'
	writer.ChunkSize = chunkSize
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, line := range sliceData {
			err := writer.Write(line)
			if err != nil {
				b.Fatalf("could not write data: %s", err)
			}
		}
		writer.Flush()
	}
	b.StopTimer()
	writer.Close()
}

func BenchmarkWrite1(b *testing.B) {
	benchmarkWrite(b, 1)
}

func BenchmarkWrite10(b *testing.B) {
	benchmarkWrite(b, 10)
}

func BenchmarkWrite50(b *testing.B) {
	benchmarkWrite(b, 50)
}

func BenchmarkWrite100(b *testing.B) {
	benchmarkWrite(b, 100)
}

type infiniteWriter struct {
	buf bytes.Buffer
}

func (iw *infiniteWriter) Write(b []byte) (int, error) {
	i, err := iw.buf.Write(b)
	iw.buf.Reset()
	return i, err
}

func BenchmarkEncodingCSVWrite(b *testing.B) {
	ir := &infiniteWriter{}
	writer := csv.NewWriter(ir)
	writer.Comma = '\t'
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, line := range sliceData {
			err := writer.Write(line)
			if err != nil {
				b.Fatalf("could not write data: %s", err)
			}
		}
	}
	b.StopTimer()
}

func init() {
	for x := 0; x < 1000; x++ { // 1000 rows
		line := make([]string, 0, 40)
		for y := 0; y < 40; y++ { // 40 columns
			// no newline! \n
			length := rand.Intn(50) // 50 chars max
			field := ""
			for z := 0; z < length; z++ {
				field += string(rand.Intn(112) + 35)
			}
			line = append(line, field)
		}
		sliceData = append(sliceData, line)
	}
}

var sliceData = [][]string{}
