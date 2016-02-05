// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multicorecsv

import (
	"encoding/csv"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

var readTests = []struct {
	Name               string
	Input              string
	Output             [][]string
	UseFieldsPerRecord bool // false (default) means FieldsPerRecord is -1

	// These fields are copied into the Reader
	Comma            rune
	Comment          rune
	FieldsPerRecord  int
	LazyQuotes       bool
	TrailingComma    bool
	TrimLeadingSpace bool

	Error  string
	Line   int // Expected error line if != 0
	Column int // Expected error column if line != 0
}{
	{
		Name:   "Simple",
		Input:  "a,b,c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "CRLF",
		Input:  "a,b\r\nc,d\r\n",
		Output: [][]string{{"a", "b"}, {"c", "d"}},
	},
	{
		Name:   "BareCR",
		Input:  "a,b\rc,d\r\n",
		Output: [][]string{{"a", "b\rc", "d"}},
	},
	//	{
	//		Name:               "RFC4180test",
	//		UseFieldsPerRecord: true,
	//		Input: `#field1,field2,field3
	//				"aaa","bb
	//				b","ccc"
	//				"a,a","b""bb","ccc"
	//				zzz,yyy,xxx
	//				`,
	//		Output: [][]string{
	//			{"#field1", "field2", "field3"},
	//			{"aaa", "bb\nb", "ccc"},
	//			{"a,a", `b"bb`, "ccc"},
	//			{"zzz", "yyy", "xxx"},
	//		},
	//	},
	{
		Name:   "NoEOLTest",
		Input:  "a,b,c",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "Semicolon",
		Comma:  ';',
		Input:  "a;b;c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	//	{
	//		Name: "MultiLine",
	//		Input: `"two
	//				line","one line","three
	//				line
	//				field"`,
	//		Output: [][]string{{"two\nline", "one line", "three\nline\nfield"}},
	//	},
	{
		Name:  "BlankLine",
		Input: "a,b,c\n\nd,e,f\n\n",
		Output: [][]string{
			{"a", "b", "c"},
			{"d", "e", "f"},
		},
	},
	{
		Name:               "BlankLineFieldCount",
		Input:              "a,b,c\n\nd,e,f\n\n",
		UseFieldsPerRecord: true,
		Output: [][]string{
			{"a", "b", "c"},
			{"d", "e", "f"},
		},
	},
	{
		Name:             "TrimSpace",
		Input:            " a,  b,   c\n",
		TrimLeadingSpace: true,
		Output:           [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "LeadingSpace",
		Input:  " a,  b,   c\n",
		Output: [][]string{{" a", "  b", "   c"}},
	},
	{
		Name:    "Comment",
		Comment: '#',
		Input:   "#1,2,3\na,b,c\n#comment",
		Output:  [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "NoComment",
		Input:  "#1,2,3\na,b,c",
		Output: [][]string{{"#1", "2", "3"}, {"a", "b", "c"}},
	},
	{
		Name:       "LazyQuotes",
		LazyQuotes: true,
		Input:      `a "word","1"2",a","b`,
		Output:     [][]string{{`a "word"`, `1"2`, `a"`, `b`}},
	},
	{
		Name:       "BareQuotes",
		LazyQuotes: true,
		Input:      `a "word","1"2",a"`,
		Output:     [][]string{{`a "word"`, `1"2`, `a"`}},
	},
	{
		Name:       "BareDoubleQuotes",
		LazyQuotes: true,
		Input:      `a""b,c`,
		Output:     [][]string{{`a""b`, `c`}},
	},
	{
		Name:  "BadDoubleQuotes",
		Input: `a""b,c`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 1,
	},
	{
		Name:             "TrimQuote",
		Input:            ` "a"," b",c`,
		TrimLeadingSpace: true,
		Output:           [][]string{{"a", " b", "c"}},
	},
	{
		Name:  "BadBareQuote",
		Input: `a "word","b"`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 2,
	},
	{
		Name:  "BadTrailingQuote",
		Input: `"a word",b"`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 10,
	},
	{
		Name:  "ExtraneousQuote",
		Input: `"a "word","b"`,
		Error: `extraneous " in field`, Line: 1, Column: 3,
	},
	//	{
	//		Name:               "BadFieldCount",
	//		UseFieldsPerRecord: true,
	//		Input:              "a,b,c\nd,e",
	//		Error:              "wrong number of fields", Line: 2,
	//	},
	//	{
	//		Name:               "BadFieldCount1",
	//		UseFieldsPerRecord: true,
	//		FieldsPerRecord:    2,
	//		Input:              `a,b,c`,
	//		Error:              "wrong number of fields", Line: 1,
	//	},
	//	{
	//		Name:   "FieldCount",
	//		Input:  "a,b,c\nd,e",
	//		Output: [][]string{{"a", "b", "c"}, {"d", "e"}},
	//	},
	{
		Name:   "TrailingCommaEOF",
		Input:  "a,b,c,",
		Output: [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:   "TrailingCommaEOL",
		Input:  "a,b,c,\n",
		Output: [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaSpaceEOF",
		TrimLeadingSpace: true,
		Input:            "a,b,c, ",
		Output:           [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaSpaceEOL",
		TrimLeadingSpace: true,
		Input:            "a,b,c, \n",
		Output:           [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaLine3",
		TrimLeadingSpace: true,
		Input:            "a,b,c\nd,e,f\ng,hi,",
		Output:           [][]string{{"a", "b", "c"}, {"d", "e", "f"}, {"g", "hi", ""}},
	},
	{
		Name:   "NotTrailingComma3",
		Input:  "a,b,c, \n",
		Output: [][]string{{"a", "b", "c", " "}},
	},
	{
		Name:          "CommaFieldTest",
		TrailingComma: true,
		Input: `x,y,z,w
x,y,z,
x,y,,
x,,,
,,,
"x","y","z","w"
"x","y","z",""
"x","y","",""
"x","","",""
"","","",""
`,
		Output: [][]string{
			{"x", "y", "z", "w"},
			{"x", "y", "z", ""},
			{"x", "y", "", ""},
			{"x", "", "", ""},
			{"", "", "", ""},
			{"x", "y", "z", "w"},
			{"x", "y", "z", ""},
			{"x", "y", "", ""},
			{"x", "", "", ""},
			{"", "", "", ""},
		},
	},
	{
		Name:             "TrailingCommaIneffective1",
		TrailingComma:    true,
		TrimLeadingSpace: true,
		Input:            "a,b,\nc,d,e",
		Output: [][]string{
			{"a", "b", ""},
			{"c", "d", "e"},
		},
	},
	{
		Name:             "TrailingCommaIneffective2",
		TrailingComma:    false,
		TrimLeadingSpace: true,
		Input:            "a,b,\nc,d,e",
		Output: [][]string{
			{"a", "b", ""},
			{"c", "d", "e"},
		},
	},
	// if there were more errors in the data, this could race as to which line
	{
		Name:  "Multicore error",
		Error: `bare " in non-quoted-field`, Line: 4, Column: 3,
		Input: `a,bb,c
a,bb,c
a,bb,c
a,b"b,c
a,bb,c
a,bb,c
a,bb,c
`,
	},
}

func TestRead(t *testing.T) {
	for _, tt := range readTests {
		r := NewReader(strings.NewReader(tt.Input))
		r.Comment = tt.Comment
		if tt.UseFieldsPerRecord {
			r.FieldsPerRecord = tt.FieldsPerRecord
		} else {
			r.FieldsPerRecord = -1
		}
		r.LazyQuotes = tt.LazyQuotes
		r.TrailingComma = tt.TrailingComma
		r.TrimLeadingSpace = tt.TrimLeadingSpace
		if tt.Comma != 0 {
			r.Comma = tt.Comma
		}
		out, err := r.ReadAll()
		perr, _ := err.(*csv.ParseError)
		if tt.Error != "" {
			if err == nil || !strings.Contains(err.Error(), tt.Error) {
				t.Errorf("%s: error %v, want error %q", tt.Name, err, tt.Error)
			} else if tt.Line != 0 && (tt.Line != perr.Line || tt.Column != perr.Column) {
				t.Errorf("%s: error at %d:%d expected %d:%d", tt.Name, perr.Line, perr.Column, tt.Line, tt.Column)
			}
		} else if err != nil {
			t.Errorf("%s: unexpected error %v", tt.Name, err)
		} else if !reflect.DeepEqual(out, tt.Output) {
			t.Errorf("%s: out=%q want %q", tt.Name, out, tt.Output)
		}
		r.Close()
	}
}

func TestClose(t *testing.T) {
	ir := &infiniteReader{
		data: data,
	}
	reader := NewReader(ir)
	reader.Comma = '\t'
	_, err := reader.Read() // start the process
	if err != nil {
		t.Errorf("Error reading from stream - %v", err)
	}
	reader.Close()
	for {
		_, err = reader.Read()
		if err == io.EOF {
			return
		}
	}
}

func benchmarkRead(b *testing.B, chunkSize int) {
	ir := &infiniteReader{
		data: data,
	}
	reader := NewReader(ir)
	reader.Comma = '\t'
	reader.ChunkSize = chunkSize
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.Read()
		if err != nil {
			b.Fatalf("could not read data: %s", err)
		}
	}
	b.StopTimer()
	reader.Close()
}

func BenchmarkRead1(b *testing.B) {
	benchmarkRead(b, 1)
}

func BenchmarkRead10(b *testing.B) {
	benchmarkRead(b, 10)
}

func BenchmarkRead50(b *testing.B) {
	benchmarkRead(b, 50)
}

func BenchmarkRead100(b *testing.B) {
	benchmarkRead(b, 100)
}

func BenchmarkRead1000(b *testing.B) {
	benchmarkRead(b, 1000)
}

type infiniteReader struct {
	loc  int
	data []byte
}

func (ir *infiniteReader) Read(buf []byte) (int, error) {
	if ir.loc == len(ir.data) {
		ir.loc = 0
	}
	l := copy(buf, ir.data[ir.loc:])
	ir.loc += l
	return l, nil
}

func BenchmarkEncodingCSV(b *testing.B) {
	ir := &infiniteReader{
		data: data,
	}
	reader := csv.NewReader(ir)
	reader.Comma = '\t'
	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.Read()
		if err != nil {
			b.Fatalf("could not read data: %s", err)
		}
	}
	b.StopTimer()
}

func init() {
	for x := 0; x < 1000; x++ { // 1000 rows
		for y := 0; y < 40; y++ { // 40 columns
			// no newline! \n
			length := rand.Intn(50) // 50 chars max
			for z := 0; z < length; z++ {
				data = append(data, byte(rand.Intn(112)+35))
			}
			data = append(data, '\t')
		}
		data = append(data, '\n')
	}
}

var data []byte
