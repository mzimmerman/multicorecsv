// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multicorecsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"runtime"
	"sync"
)

type csvEncoded struct {
	data *bytes.Buffer
	num  int
}

type linesToWrite struct {
	data [][]string
	num  int
}

// A Writer writes records to a CSV encoded file.
//
// As returned by NewWriter, a Writer writes records terminated by a
// newline and uses ',' as the field delimiter.  The exported fields can be
// changed to customize the details before the first call to Write or WriteAll.
//
// Comma is the field delimiter.
//
// If UseCRLF is true, the Writer ends each record with \r\n instead of \n.
type Writer struct {
	Comma     rune // Field delimiter (set to ',' by NewWriter)
	UseCRLF   bool // True to use \r\n as the line terminator
	ChunkSize int  // the # of lines to hand to each goroutine -- default 50
	w         io.Writer

	lineout    chan csvEncoded
	linein     chan linesToWrite
	place      int        // how many groups of ChunkSize asked to write
	queueIn    [][]string // used to buffer lines requested to write
	finalError error
	//	cancel         chan struct{} // when this is closed, cancel all operations
	closeOnce      sync.Once
	errChan        chan error
	flushOperation chan struct{} // value is sent when Flush operation completes
	bufPool        sync.Pool
	lock           sync.Mutex
}

// NewWriter returns a new Writer that writes to w.  Must call Close when done.
func NewWriter(iow io.Writer) *Writer {
	w := &Writer{
		Comma:   ',',
		w:       iow,
		lineout: make(chan csvEncoded),
		linein:  make(chan linesToWrite),
		queueIn: make([][]string, 0, 50),
		//		cancel:    make(chan struct{}),
		ChunkSize: 50, // sane default
		bufPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
		flushOperation: make(chan struct{}),
		errChan:        make(chan error),
	}
	go func() {
		var wg sync.WaitGroup
		wg.Add(runtime.NumCPU())
		for x := 0; x < runtime.NumCPU(); x++ {
			go w.startEncoding(&wg)
		}
		go w.startWriting()
		go func() {
			w.finalError = <-w.errChan
			//				log.Printf("Received error - %v", w.finalError)
			_ = w.Close()
		}()
		wg.Wait()
		close(w.lineout)
	}()
	return w
}

// Close closes the underlying io.Writer if it's also an io.Closer as well as
// cleaning up all goroutines
func (mcw *Writer) Close() error {
	mcw.closeOnce.Do(func() {
		mcw.Flush()
		close(mcw.linein)
		go func() {
			for {
				if _, ok := <-mcw.lineout; !ok {
					//					log.Printf("Throwing away lineout - %q", lineout)
					// read them all so that the encoders can die
					return
				}
			}
		}()
		if closer, ok := mcw.w.(io.Closer); ok {
			mcw.finalError = closer.Close()
		}
	})
	return mcw.finalError
}

// Writer writes a single CSV record to w along with any necessary quoting.
// A record is a slice of strings with each string being one field.
func (mcw *Writer) Write(record []string) (err error) {
	if len(record) == 0 {
		return nil // done!
	}
	return mcw.write(record)
}
func (mcw *Writer) write(record []string) (err error) {
	mcw.lock.Lock()
	if len(mcw.queueIn) == mcw.ChunkSize || len(record) == 0 { // 0 len == Flush
		//		log.Printf("Sending records for encoding, batch #%d, %q", w.place, w.queueIn)
		mcw.linein <- linesToWrite{
			data: mcw.queueIn,
			num:  mcw.place,
		}
		mcw.place++
		mcw.queueIn = make([][]string, 0, mcw.ChunkSize)
	}
	if len(record) == 0 {
		//		log.Printf("in write(), requesting flush - #%d", w.place)
		mcw.linein <- linesToWrite{
			num: mcw.place,
		}
		mcw.place++
	} else {
		mcw.queueIn = append(mcw.queueIn, record)
		//		log.Printf("in write() queueing record to write - %q", w.queueIn)
	}
	mcw.lock.Unlock()
	return nil
}

func (mcw *Writer) startEncoding(wg *sync.WaitGroup) {
	defer wg.Done()
	for records := range mcw.linein {
		if len(records.data) == 0 {
			mcw.lineout <- csvEncoded{
				num:  records.num,
				data: nil, // sending a flush request
			}
			//			log.Printf("startEncoding() - Sent flush request - #%d", records.num)
			continue
		}
		//		log.Printf("startEncoding() - got batch #%d for encoding - %q", records.num, records.data)
		buf := mcw.bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		writer := csv.NewWriter(buf)
		writer.Comma = mcw.Comma
		writer.UseCRLF = mcw.UseCRLF
		_ = writer.WriteAll(records.data) // can ignore error, writing to a buffer
		mcw.lineout <- csvEncoded{
			num:  records.num,
			data: buf,
		}
		//		log.Printf("Sent %d for writing - %q", records.num, buf.String())
	}
}

func (mcw *Writer) writeInternal(buf *bytes.Buffer, bufferedWriter *bufio.Writer) {
	if buf == nil {
		//		log.Printf("Flushing underlying io.Writer")
		err := bufferedWriter.Flush()
		if err != nil {
			//			log.Printf("writeInternal() caught error 1 - %v - sending", err)
			mcw.errChan <- err
		}
		//		log.Printf("Flushed underlying io.Writer, sending notification")
		mcw.flushOperation <- struct{}{}
		//		log.Printf("Sent flush notification")
		return
	}
	//	log.Printf("Writing underlying data - %q", buf.Bytes())
	_, err := bufferedWriter.Write(buf.Bytes())
	if err != nil {
		//		log.Printf("writeInternal() caught error 2 - %v - sending", err)
		mcw.errChan <- err
	}
	mcw.bufPool.Put(buf)
}

func (mcw *Writer) startWriting() {
	currentPlace := 0
	bufferedWriter := bufio.NewWriter(mcw.w)
	queueOut := make(map[int]*bytes.Buffer)
Top:
	for {
		buf, ok := queueOut[currentPlace]
		if !ok {
			break // next value isn't in the queue, move on
		}
		delete(queueOut, currentPlace)
		mcw.writeInternal(buf, bufferedWriter)
		currentPlace++
	}
	//	log.Printf("looking for lineout #%d", currentPlace)
	for lines := range mcw.lineout {
		//		log.Printf("Got line #%d from lineout", lines.num)
		if lines.num == currentPlace {
			mcw.writeInternal(lines.data, bufferedWriter)
			currentPlace++
		} else {
			queueOut[lines.num] = lines.data
		}
		goto Top
	}
	mcw.finalError = <-mcw.errChan
}

// Flush writes any buffered data to the underlying io.Writer.
// To check if an error occurred during the Flush, call Error.
func (mcw *Writer) Flush() {
	_ = mcw.write(nil)
	<-mcw.flushOperation
}

// Error reports any error that has occurred during a previous Write or Flush.
func (mcw *Writer) Error() error {
	_, err := mcw.w.Write(nil)
	return err
}

// WriteAll writes multiple CSV records to w using Write and then calls Flush.
// Close must still be called after WriteAll to clean up the underlying goroutines
func (mcw *Writer) WriteAll(records [][]string) (err error) {
	for _, record := range records {
		err = mcw.Write(record)
		if err != nil {
			return err
		}
	}
	mcw.Flush()
	return mcw.Error()
}
