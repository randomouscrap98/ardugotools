package arduboy

import (
	"errors"
	"io"
)

type ReadWriteErrorPass struct {
	rw  io.ReadWriter
	err error
}

type ReadWritePasser interface {
	io.ReadWriter
	ReadPass([]byte) int
	WritePass([]byte) int
	IsPass() error
}

func (rwep *ReadWriteErrorPass) loopError(b []byte, f func([]byte) (int, error)) (int, error) {
	if rwep.err != nil {
		return 0, rwep.err
	}
	writeexpect := len(b)
	writeamount := 0
	slice := b
	for {
		bcount, err := f(slice) //rwep.rw.Write(slice)
		if err != nil {
			rwep.err = err
			return 0, err
		}
		writeamount += bcount
		if writeamount >= writeexpect {
			return writeamount, nil
		}
		slice = slice[bcount:]
		if len(slice) == 0 {
			rwep.err = errors.New("PROGRAM ERROR: ReadWriteErrorPass ran out of slice!")
			return 0, rwep.err
		}
	}
}

// A special write function which will skip if an error is present in the struct,
// and which will write until it has written the entire buffer contents (blocking)
func (rwep *ReadWriteErrorPass) Write(b []byte) (int, error) {
	return rwep.loopError(b, rwep.rw.Write)
}

// A special read function which will skip if an error is present in the struct,
// and which will read until it fills the entire given byte slice (blocking)
func (rwep *ReadWriteErrorPass) Read(b []byte) (int, error) {
	return rwep.loopError(b, rwep.rw.Read)
}

func (rwep *ReadWriteErrorPass) WritePass(b []byte) int {
	val, _ := rwep.Write(b)
	return val
}

func (rwep *ReadWriteErrorPass) ReadPass(b []byte) int {
	val, _ := rwep.Read(b)
	return val
}

func (rwep *ReadWriteErrorPass) IsPass() error {
	return rwep.err
}
