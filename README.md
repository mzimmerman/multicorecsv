[![Build Status](https://travis-ci.org/mzimmerman/multicorecsv.svg)](https://travis-ci.org/mzimmerman/multicorecsv) [![Coverage](http://gocover.io/_badge/github.com/mzimmerman/multicorecsv)](http://gocover.io/github.com/mzimmerman/multicorecsv)  [![GoReport](https://goreportcard.com/badge/mzimmerman/multicorecsv)](http://goreportcard.com/report/mzimmerman/multicorecsv)  [![GoDoc](https://godoc.org/github.com/mzimmerman/multicorecsv?status.svg)](https://godoc.org/github.com/mzimmerman/multicorecsv)  [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

# multicorecsv
A multicore csv library in Go which is ~3x faster than plain encoding/csv

## No newline support on multicorecsv.Reader!
- muticorecsv does not support reading CSV files with properly quoted/escaped newlines!  If you have \n in your source data fields, multicorecsv Read() will not work for you.

## API Changes from encoding/csv
- multicorecsv is an *almost* drop in replacement for encoding/csv.  There's only one new requirement, you must use the Close() method.  Best practice is a defer (reader/writer).Close()
```
func main() {
	in := `first_name,last_name,username
"Rob","Pike",rob
Ken,Thompson,ken
"Robert","Griesemer","gri"
`
	r := multicorecsv.NewReader(strings.NewReader(in))
	defer r.Close() // the underlying strings.Reader cannot be closed,
					// but that doesn't matter, multicorecsv needs to clean up
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(record)
	}
}
```


## Performance
- With Reader, multicorecsv splits up the data by line, then gives out lines for different cores to parse before putting it back in proper line order for the reader
- With Writer, multicorecsv sends batches of lines off to be encoded, then writes out the results in order

### Performance Tweaks
- Prior to calling Read or (anytime with Write), you can set the ChunkSize (how many lines are sent to each goroutine at a time)
- ChunkSize defaults at 50 - for shorter lines of data, give it a higher value, for larger lines, give it less
- 50 is a general sweet spot for the data generated in the benchmarks

## Metrics (finally!)
- tests run on Intel(R) Core(TM) i7-4710HQ CPU @ 2.50GHz
- multicorecsv Read() beats encoding/csv by ~3x with 8 CPUs and is about equal on single core tasks
- multicorecsv Write() beats encoding/csv by ~3x with 8 CPUs and is about equal on single core tasks
- Benchmarks on other hardware is appreciated!
```
BenchmarkRead1                     50000             32796 ns/op
BenchmarkRead1-2                  100000             22398 ns/op
BenchmarkRead1-4                  100000             17680 ns/op
BenchmarkRead1-8                  100000             16575 ns/op
BenchmarkRead1-16                 100000             16022 ns/op
BenchmarkRead10                    50000             32064 ns/op
BenchmarkRead10-2                 100000             19812 ns/op
BenchmarkRead10-4                 100000             14199 ns/op
BenchmarkRead10-8                 100000             10931 ns/op
BenchmarkRead10-16                200000             10726 ns/op
BenchmarkRead50                    50000             34506 ns/op
BenchmarkRead50-2                 100000             19202 ns/op
BenchmarkRead50-4                 100000             13262 ns/op
BenchmarkRead50-8                 200000             10555 ns/op
BenchmarkRead50-16                200000             10781 ns/op
BenchmarkRead100                   50000             35907 ns/op
BenchmarkRead100-2                100000             18461 ns/op
BenchmarkRead100-4                100000             13138 ns/op
BenchmarkRead100-8                200000             10364 ns/op
BenchmarkRead100-16               200000             10513 ns/op
BenchmarkRead1000                  50000             34773 ns/op
BenchmarkRead1000-2               100000             18581 ns/op
BenchmarkRead1000-4               100000             11184 ns/op
BenchmarkRead1000-8               200000             11484 ns/op
BenchmarkRead1000-16              100000             10061 ns/op
BenchmarkEncodingCSVRead               50000             27706 ns/op
BenchmarkEncodingCSVRead-2             50000             27765 ns/op
BenchmarkEncodingCSVRead-4             50000             28126 ns/op
BenchmarkEncodingCSVRead-8             50000             28090 ns/op
BenchmarkEncodingCSVRead-16            50000             28457 ns/op
BenchmarkWrite1                       50          25826817 ns/op
BenchmarkWrite1-2                    100          19699325 ns/op
BenchmarkWrite1-4                    100          15331869 ns/op
BenchmarkWrite1-8                    100          13768925 ns/op
BenchmarkWrite1-16                   100          13574201 ns/op
BenchmarkWrite10                      50          23285415 ns/op
BenchmarkWrite10-2                   100          12505588 ns/op
BenchmarkWrite10-4                   200           6953782 ns/op
BenchmarkWrite10-8                   200           6870859 ns/op
BenchmarkWrite10-16                  200           7186447 ns/op
BenchmarkWrite50                      50          23483132 ns/op
BenchmarkWrite50-2                   100          12319305 ns/op
BenchmarkWrite50-4                   200           6581955 ns/op
BenchmarkWrite50-8                   200           6711311 ns/op
BenchmarkWrite50-16                  200           6912013 ns/op
BenchmarkWrite100                     50          23413136 ns/op
BenchmarkWrite100-2                  100          12094800 ns/op
BenchmarkWrite100-4                  200           8499881 ns/op
BenchmarkWrite100-8                  200           7291376 ns/op
BenchmarkWrite100-16                 200           7288081 ns/op
BenchmarkWrite1000                    50          23722019 ns/op
BenchmarkWrite1000-2                  50          24078729 ns/op
BenchmarkWrite1000-4                  50          23667001 ns/op
BenchmarkWrite1000-8                  50          23790221 ns/op
BenchmarkWrite1000-16                 50          23639162 ns/op
BenchmarkEncodingCSVWrite             50          26048187 ns/op
BenchmarkEncodingCSVWrite-2           50          22749519 ns/op
BenchmarkEncodingCSVWrite-4           50          23521142 ns/op
BenchmarkEncodingCSVWrite-8           50          23634894 ns/op
BenchmarkEncodingCSVWrite-16          50          23854300 ns/op
```
