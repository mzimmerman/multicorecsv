package multicorecsv

import (
	"bytes"
	"encoding/csv"
	"io"
)

type Reader struct {
	reader  *csv.Reader
	done    chan error
	toWrite chan [][]string
	writer  *csv.Writer
	toRead  chan []string
}

func NewReader(rdr io.Reader, size int) *Reader {
	reader := csv.NewReader(rdr)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // disabling this check
	buf := &bytes.Buffer{}
	r := &Reader{
		reader:  reader,
		done:    make(chan error),
		toWrite: make(chan [][]string, size),
		writer:  csv.NewWriter(buf),
		toRead:  make(chan []string, size),
	}
	go func() {
		defer close(r.toRead)
		// queue from the reader, then queue to the writer
		for {
			tw, ok := <-r.toWrite
			if !ok {
				return
			}
			for x := range tw {
				select {
				case r.toRead <- tw[x]:
				case <-r.done:
					return
				}
			}
		}
	}()
	go func() {
		// read and unstutter them!
		defer func() {
			close(r.toWrite)
		}()
		toSend := make([][]string, 0, size)
		for {
			line, err := reader.Read()
			if err == io.EOF {
				select {
				case _, ok := <-r.done:
					_ = ok // we don't use this but I need to receive a value for the compiler
					// done processing
				case r.toWrite <- toSend:
				}
				return
			}
			if err != nil {
				select {
				case _, ok := <-r.done:
					_ = ok // we don't use this but I need to receive a value for the compiler
					// done processing
				case r.done <- err:
					// error was sent, stop reading
				}
				return
			}
			toSend = append(toSend, line)
			if len(toSend) == size {
				select {
				case _, ok := <-r.done:
					_ = ok // we don't use this but I need to receive a value for the compiler
					// done processing
				case r.toWrite <- toSend:
					toSend = make([][]string, 0, size)
					// log.Printf("sent line toWrite")
				}
			}
		}
	}()
	return r
}

// Close cleans up the resources created to read the file multicore style
func (reader *Reader) Close() {
	close(reader.done)
}

// Read returns only valid CSV data as read from the source and removes multilines and stutter
// if there's an error all subsequent calls to Read will fail with the same error
func (reader *Reader) Read() ([]string, error) {
	select {
	case line := <-reader.toRead:
		return line, nil
	case err := <-reader.done:
		return nil, err
	}
}
