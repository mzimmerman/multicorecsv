[![Build Status](https://travis-ci.org/mzimmerman/multicorecsv.svg)](https://travis-ci.org/mzimmerman/multicorecsv) [![Coverage](http://gocover.io/_badge/github.com/mzimmerman/multicorecsv)](http://gocover.io/github.com/mzimmerman/multicorecsv)

# multicorecsv
A multicore csv reader library in Go

## No newline support
- muticorecsv does not support CSV files with properly quoted/escaped newlines!  If you have \n in your data fields, multicorecsv will not work for you.

On x64, when reading long lines of data, multicorecsv beats encoding/csv by ~2x with 8+ CPUs and is about equal on single core tasks
```
BenchmarkRead             300000             29436 ns/op
BenchmarkRead-2           300000             20065 ns/op
BenchmarkRead-4           500000             18432 ns/op
BenchmarkRead-8           500000             17633 ns/op
BenchmarkRead-16          500000             16856 ns/op
BenchmarkEncodingCSV      300000             27890 ns/op
BenchmarkEncodingCSV-2    300000             27565 ns/op
BenchmarkEncodingCSV-4    300000             29244 ns/op
BenchmarkEncodingCSV-8    300000             29230 ns/op
BenchmarkEncodingCSV-16   300000             29286 ns/op
```
- multicorecsv splits up the data by line, then gives out lines for different cores to parse before putting it back in proper line order for the reader
