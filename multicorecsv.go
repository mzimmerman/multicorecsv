package multicorecsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"log"
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
	count   int
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

func (mcr *MulticoreReader) Stream() (chan []string, chan error) {
	mcr.start()
	return mcr.lineout, mcr.errchan
}

func (mcr *MulticoreReader) Read() ([]string, error) {
	mcr.start()
	line, ok := <-mcr.lineout
	if ok {
		mcr.count++
		if mcr.count%100000 == 0 {
			log.Printf("Read line %d", mcr.count)
		}
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
				r := csv.NewReader(bufio.NewReaderSize(BytesChanReader{
					bytesChan: mcr.linein,
				}, 1))
				r.Comma = mcr.Comma
				for {
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
			foundError := <-err1 // in good case, this will still be io.EOF
			for i := 0; i < runtime.NumCPU(); i++ {
				err := <-err2
				if err != nil && foundError == io.EOF {
					foundError = err
				}
			}
			go func() {
				for {
					mcr.errchan <- foundError
				}
			}()
		}()
	})
}

type BytesChanReader struct {
	bytesChan chan []byte
	buf       bytes.Buffer
}

func (scr BytesChanReader) Read(dst []byte) (int, error) {
	if scr.buf.Len() > 0 {
		return scr.buf.Read(dst)
	}
	line, ok := <-scr.bytesChan
	if !ok {
		return 0, io.EOF
	}
	n := copy(dst, line)
	if n < len(line) {
		scr.buf.Write(line[n:])
	}
	return n, nil
}
