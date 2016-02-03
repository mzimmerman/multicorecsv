[![Build Status](https://travis-ci.org/mzimmerman/multicorecsv.svg)](https://travis-ci.org/mzimmerman/multicorecsv) [![Coverage](http://gocover.io/_badge/github.com/mzimmerman/multicorecsv)](http://gocover.io/github.com/mzimmerman/multicorecsv)

# multicorecsv
A multicore csv reader library in Go

## No newline support
- muticorecsv does not support CSV files with properly quoted/escaped newlines!  If you have \n in your data fields, multicorecsv will not work for you.

On x64, when reading long lines of data, multicorecsv beats encoding/csv by ~10x with 16+ CPUs and is about equal on single core tasks
```
BenchmarkRead              10000            777165 ns/op
BenchmarkRead-2            20000            403737 ns/op
BenchmarkRead-4            30000            205364 ns/op
BenchmarkRead-8           100000            106925 ns/op
BenchmarkRead-16          100000             75292 ns/op
BenchmarkRead-24          100000             68552 ns/op
BenchmarkEncodingCSV       10000            755472 ns/op
BenchmarkEncodingCSV-2     10000            752154 ns/op
BenchmarkEncodingCSV-4     10000            770172 ns/op
BenchmarkEncodingCSV-8     10000            765852 ns/op
BenchmarkEncodingCSV-16    10000            776697 ns/op
BenchmarkEncodingCSV-24    10000            803459 ns/op
```
- multicorecsv splits up the data by line, then gives out lines for different cores to parse before putting it back in proper line order for the reader
