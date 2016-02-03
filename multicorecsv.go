package multicorecsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"runtime"
	"sync"
)

type linein struct {
	data []byte
	num  int
}

type lineout struct {
	data []string
	num  int
}

type MulticoreReader struct {
	reader  io.Reader
	linein  chan linein
	lineout chan lineout
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
	once             sync.Once
}

func NewReader(r io.Reader) *MulticoreReader {
	return &MulticoreReader{
		reader:  r,
		Comma:   ',',
		linein:  make(chan linein),
		lineout: make(chan lineout),
		errChan: make(chan error),
		queue:   make(map[int][]string),
		cancel:  make(chan struct{}),
	}
}

func (mcr *MulticoreReader) Close() error {
	close(mcr.cancel)
	return nil
}

func (mcr *MulticoreReader) ReadAll() ([][]string, error) {
	var all [][]string
	out, errChan := mcr.Stream()
	for line := range out {
		all = append(all, line)
	}
	return all, <-errChan
}

func (mcr *MulticoreReader) Stream() (chan []string, chan error) {
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

func (mcr *MulticoreReader) Read() ([]string, error) {
	if mcr.finalError != nil {
		return nil, mcr.finalError
	}
	mcr.start()
	line, ok := mcr.queue[mcr.place]
	if ok {
		delete(mcr.queue, mcr.place)
		mcr.place++
		return line, nil
	}
	for {
		//		log.Printf("mcr.Read() reading from lineout")
		line, ok := <-mcr.lineout
		if !ok {
			mcr.finalError = <-mcr.errChan
			return nil, mcr.finalError
		}
		if line.num == mcr.place {
			mcr.place++
			return line.data, nil
		}
		mcr.queue[line.num] = line.data
		// keep going, fetch the next line since this one was out of order
	}
}

func (mcr *MulticoreReader) startReading(err1 chan error) {
	defer close(mcr.linein)
	linenum := 0
	bytesreader := bufio.NewReader(mcr.reader)
	for {
		line, err := bytesreader.ReadBytes('\n')
		if len(line) > 0 {
			if line[0] == '\n' || rune(line[0]) == mcr.Comment {
				continue // we don't want blanks or comments
			}
			select {
			case mcr.linein <- linein{
				data: line,
				num:  linenum,
			}:
				linenum++
			case <-mcr.cancel:
				err1 <- nil
				return
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			err = nil
		}
		err1 <- err
		return
	}
}

func (mcr *MulticoreReader) parseCSVLines(err2 chan error) {
	var buf bytes.Buffer
	r := csv.NewReader(&buf)
	r.Comma = mcr.Comma
	r.Comment = mcr.Comment
	r.LazyQuotes = mcr.LazyQuotes
	r.TrailingComma = mcr.TrailingComma
	r.TrimLeadingSpace = mcr.TrimLeadingSpace
	for b := range mcr.linein {
		buf.Reset()
		buf.Write(b.data)
		if line, err := r.Read(); err != nil {
			pe := err.(*csv.ParseError)
			pe.Line = b.num + 1
			err2 <- err
			return
		} else {
			select {
			case mcr.lineout <- lineout{
				data: line,
				num:  b.num,
			}:
			case <-mcr.cancel:
				err2 <- nil
				return
			}
		}
	}
	err2 <- nil
}

func (mcr *MulticoreReader) waitForDone(err1, err2 chan error) {
	//	log.Printf("mcr.waitForDone waiting on read error")
	foundError := <-err1
	//	log.Printf("mcr.waitForDone waiting on parse errors")
	for i := 0; i < runtime.NumCPU(); i++ {
		err := <-err2
		if err != nil && err != io.EOF && foundError == nil {
			foundError = err
		}
	}
	if foundError == nil {
		foundError = io.EOF
	}
	//	log.Printf("mcr.waitForDone closing lineout")
	close(mcr.lineout)
	mcr.errChan <- foundError
	//	log.Printf("mcr.waitForDone ending")
}

func (mcr *MulticoreReader) start() {
	mcr.once.Do(func() {
		err1 := make(chan error, 1)
		err2 := make(chan error, runtime.NumCPU())
		go mcr.startReading(err1)
		for i := 0; i < runtime.NumCPU(); i++ {
			go mcr.parseCSVLines(err2)
		}
		go mcr.waitForDone(err1, err2)
	})
}
