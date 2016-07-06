package multicorecsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"runtime"
	"sync"
)

type csvLine struct {
	data []byte
	num  int
}

type sliceLine struct {
	data []string
	num  int
}

// Reader contains all the internals required.  Use NewReader(io.Reader).
type Reader struct {
	reader  io.Reader
	linein  chan []csvLine
	lineout chan []sliceLine
	errChan chan error
	// the following are from encoding/csv package and are copied into the underlying csv.Reader
	Comma            rune
	Comment          rune
	FieldsPerRecord  int // we can't implement this without more overhead/synchronization
	LazyQuotes       bool
	TrailingComma    bool
	TrimLeadingSpace bool
	place            int              // how many lines have been returned so far
	queue            map[int][]string // used to buffer lines that come in out of order
	finalError       error
	cancel           chan struct{} // when this is closed, cancel all operations
	readOnce         sync.Once
	closeOnce        sync.Once
	ChunkSize        int // the # of lines to hand to each goroutine -- default 50
}

// NewReader returns a new Reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return NewReaderSized(r, 50)
}

// NewReader returns a new Reader that reads from r with the chunked size
func NewReaderSized(r io.Reader, chunkSize int) *Reader {
	return &Reader{
		reader:    r,
		Comma:     ',',
		linein:    make(chan []csvLine, chunkSize),
		lineout:   make(chan []sliceLine, chunkSize),
		errChan:   make(chan error),
		queue:     make(map[int][]string),
		cancel:    make(chan struct{}),
		ChunkSize: chunkSize,
	}
}

// Close will clean up any goroutines that aren't finished.
// It will also close the underlying Reader if it implements io.ReadCloser
func (mcr *Reader) Close() error {
	var insideError error
	mcr.closeOnce.Do(func() {
		close(mcr.cancel)
		if c, ok := mcr.reader.(io.Closer); ok {
			insideError = c.Close()
		}
	})
	return insideError
}

// ReadAll reads all the remaining records from r.
// Each record is a slice of fields.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read until EOF, it does not treat end of file as an error to be
// reported.
func (mcr *Reader) ReadAll() ([][]string, error) {
	var all [][]string
	out, errChan := mcr.Stream()
	for line := range out {
		all = append(all, line)
	}
	return all, <-errChan
}

// Stream returns a chan of []string representing a row in the CSV file.
// Lines are sent on the channel in order they were in the source file.
// The caller must receive all rows and receive the error from the error chan,
// otherwise the caller must call Close to clean up any goroutines.
func (mcr *Reader) Stream() (chan []string, chan error) {
	out := make(chan []string)
	errChan := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errChan)
		for {
			line, err := mcr.Read()
			if len(line) > 0 {
				out <- line
			}
			if err == nil {
				continue
			}
			if err == io.EOF {
				return
			}
			errChan <- err
			return
		}
	}()
	return out, errChan
}

// Read reads one record from r.  The record is a slice of strings with each
// string representing one field.  In the background, the internal io.Reader
// will be read from ahead of the caller utilizing Read() to pull every row
func (mcr *Reader) Read() ([]string, error) {
	if mcr.finalError != nil {
		return nil, mcr.finalError
	}
	mcr.start()
	for {
		line, ok := mcr.queue[mcr.place]
		if !ok {
			break // next value isn't in the queue, move on
		}
		delete(mcr.queue, mcr.place)
		mcr.place++
		if len(line) == 0 {
			continue
		}
		return line, nil
	}
	found := false
	var foundVal []string
	for lines := range mcr.lineout {
		for _, line := range lines {
			if line.num == mcr.place {
				found = true
				foundVal = line.data
			} else {
				mcr.queue[line.num] = line.data
			}
		}
		if found {
			mcr.place++
			return foundVal, nil
		} // else, keep going, didn't find what we were looking for yet!
	}
	mcr.finalError = <-mcr.errChan
	return nil, mcr.finalError
}

func (mcr *Reader) startReading() error {
	defer close(mcr.linein)
	linenum := 0
	bytesreader := bufio.NewReader(mcr.reader)
NextChunk:
	for {
		toBeParsed := make([]csvLine, 0, mcr.ChunkSize)
		for {
			line, err := bytesreader.ReadBytes('\n')
			if len(line) > 0 {
				if line[0] == '\r' {
					continue // we don't care about 'blank' lines from Windows style
				}
				toBeParsed = append(toBeParsed, csvLine{
					data: line,
					num:  linenum,
				})
				linenum++
			}
			if err == nil || err == io.EOF {
				if len(toBeParsed) == mcr.ChunkSize || err == io.EOF {
					select {
					case mcr.linein <- toBeParsed:
						if err == io.EOF {
							return nil
						}
						continue NextChunk
					case <-mcr.cancel:
						return nil
					}
				}
				continue
			}
			return err // err is not nil and is not io.EOF
		}
	}
}

func (mcr *Reader) parseCSVLines() error {
	var buf bytes.Buffer
	r := csv.NewReader(&buf)
	r.Comma = mcr.Comma
	r.Comment = mcr.Comment
	r.LazyQuotes = mcr.LazyQuotes
	r.TrailingComma = mcr.TrailingComma
	r.TrimLeadingSpace = mcr.TrimLeadingSpace
	for toBeParsed := range mcr.linein {
		parsed := make([]sliceLine, 0, len(toBeParsed))
		for _, b := range toBeParsed {
			buf.Reset()
			_, _ = buf.Write(b.data)
			char, _, err := buf.ReadRune()
			if err != nil {
				_ = mcr.Close()
				return err
			}
			if char == '\n' || char == mcr.Comment {
				parsed = append(parsed, sliceLine{
					data: nil,
					num:  b.num,
				})
				continue
			}
			_ = buf.UnreadRune()
			line, err := r.Read()
			if err != nil {
				pe, ok := err.(*csv.ParseError)
				if ok {
					pe.Line = b.num + 1
				}
				_ = mcr.Close()
				return err
			}
			parsed = append(parsed, sliceLine{
				data: line,
				num:  b.num,
			})
		}
		select {
		case mcr.lineout <- parsed:
		case <-mcr.cancel:
			return nil
		}
	}
	return nil
}

func (mcr *Reader) waitForDone(err1, err2 chan error) {
	foundError := <-err1
	for i := 0; i < runtime.NumCPU(); i++ {
		err := <-err2
		if err != nil && err != io.EOF && foundError == nil {
			foundError = err
		}
	}
	if foundError == nil {
		foundError = io.EOF
	}
	close(mcr.lineout)
	mcr.errChan <- foundError
}

func (mcr *Reader) start() {
	mcr.readOnce.Do(func() {
		err1 := make(chan error, 1)
		err2 := make(chan error)
		go func() {
			err1 <- mcr.startReading()
		}()
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				err2 <- mcr.parseCSVLines()
			}()
		}
		go mcr.waitForDone(err1, err2)
	})
}
