package multicorecsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"runtime"
	"sync"
)

type MulticoreReader struct {
	reader  io.Reader
	Comma   rune
	linein  chan []byte
	lineout chan []string
	errchan chan error
	started sync.Once
}

func NewReader(r io.Reader) *MulticoreReader {
	return &MulticoreReader{
		reader:  r,
		Comma:   ',',
		linein:  make(chan []byte, runtime.NumCPU()),
		lineout: make(chan []string, runtime.NumCPU()),
		errchan: make(chan error, 1),
	}
}

func (mcr *MulticoreReader) ReadAll() ([][]string, error) {
	mcr.start()
	var all [][]string
	for {
		if line, ok := <-mcr.lineout; !ok {
			if !ok {
				return all, <-mcr.errchan
			}
			all = append(all, line)
		}
	}
}

func (mcr *MulticoreReader) Read() ([]string, error) {
	mcr.start()
	line, ok := <-mcr.lineout
	if ok {
		return line, nil
	}
	return line, <-mcr.errchan
}

func (mcr *MulticoreReader) start() {
	mcr.started.Do(func() {
		err1 := make(chan error, 1)
		err2 := make(chan error, runtime.NumCPU())
		go func() {
			defer close(mcr.linein)
			bytesreader := bufio.NewReader(mcr.reader)
			for {
				line, err := bytesreader.ReadBytes('\n')
				if err == nil {
					mcr.linein <- line
					continue
				}
				err1 <- err
				return
			}
		}()
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				var buf bytes.Buffer
				r := csv.NewReader(&buf)
				r.Comma = mcr.Comma
				for b := range mcr.linein {
					buf.Reset()
					buf.Write(b)
					if line, err := r.Read(); err != nil {
						err2 <- err
						return
					} else {
						mcr.lineout <- line
					}
				}
				err2 <- nil
			}()
		}
		go func() {
			defer close(mcr.lineout)
			foundError := <-err1
			for i := 0; i < runtime.NumCPU(); i++ {
				err := <-err2
				if err != io.EOF {
					if foundError == nil {
						foundError = err
					}
				}
			}
			if foundError == nil {
				foundError = io.EOF
			}
			go func() {
				for {
					mcr.errchan <- foundError
				}
			}()
		}()
	})
}
