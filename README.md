[![Build Status](https://travis-ci.org/mzimmerman/multicorecsv.svg)](https://travis-ci.org/mzimmerman/multicorecsv) [![Coverage](http://gocover.io/_badge/github.com/mzimmerman/multicorecsv)](http://gocover.io/github.com/mzimmerman/multicorecsv)  [![GoReport](https://goreportcard.com/badge/mzimmerman/multicorecsv)](http://goreportcard.com/report/mzimmerman/multicorecsv)  [![GoDoc](https://godoc.org/github.com/mzimmerman/multicorecsv?status.svg)](https://godoc.org/github.com/mzimmerman/multicorecsv)  [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

# multicorecsv
A multicore csv reader library in Go

## No newline support
- muticorecsv does not support CSV files with properly quoted/escaped newlines!  If you have \n in your data fields, multicorecsv will not work for you.

## API Changes
- multicorecsv is an *almost* drop in replacement for encoding/csv.  There's only one new requirement -- if you aren't going to read to the end of the data (including errors, you must use the Close() method.  Best practice is a defer r.Close() even if you plan to read (you handle errors right?!)
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
- multicorecsv splits up the data by line, then gives out lines for different cores to parse before putting it back in proper line order for the reader

### Performance Tweaks
- Prior to calling Read, you can set the ChunkSize (how many lines are sent to each goroutine at a time)
- ChunkSize defaults at 50 - for shorter lines of data, give it a higher value, for larger lines, give it less
- 50 is the sweet spot for the data generated in the benchmarks

## Metrics (finally!)
- tests run on Intel(R) Core(TM) i7-4710HQ CPU @ 2.50GHz
- multicorecsv beats encoding/csv by ~3x with 8 CPUs and is about equal on single core tasks
- Benchmarks on other hardware is appreciated!
```
BenchmarkRead1            200000             29991 ns/op
BenchmarkRead1-2          300000             21063 ns/op
BenchmarkRead1-4          500000             16536 ns/op
BenchmarkRead1-8          500000             17537 ns/op
BenchmarkRead10           200000             29688 ns/op
BenchmarkRead10-2         500000             19029 ns/op
BenchmarkRead10-4        1000000             12571 ns/op
BenchmarkRead10-8         500000             10481 ns/op
BenchmarkRead50           200000             31392 ns/op
BenchmarkRead50-2         300000             19499 ns/op
BenchmarkRead50-4         500000             14036 ns/op
BenchmarkRead50-8        1000000              9000 ns/op
BenchmarkRead100          200000             31480 ns/op
BenchmarkRead100-2        300000             19975 ns/op
BenchmarkRead100-4       1000000             13445 ns/op
BenchmarkRead100-8       1000000             10389 ns/op
BenchmarkRead1000         200000             33330 ns/op
BenchmarkRead1000-2       500000             17230 ns/op
BenchmarkRead1000-4      1000000             12326 ns/op
BenchmarkRead1000-8       500000             11027 ns/op
BenchmarkEncodingCSV      200000             29548 ns/op
BenchmarkEncodingCSV-2    300000             28419 ns/op
BenchmarkEncodingCSV-4    300000             28137 ns/op
BenchmarkEncodingCSV-8    300000             27986 ns/op
```
