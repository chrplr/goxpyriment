package internal

import (
	"bufio"
	"fmt"
	"io"
)

type Reader struct {
	size   int
	buffer []byte
	rd     io.Reader
}

func NewReader(r io.Reader, size byte) *Reader {
	return &Reader{
		size:   int(size),
		buffer: make([]byte, size, size),
		rd:     bufio.NewReaderSize(r, int(size)*256),
	}
}

func (r *Reader) Read() ([]byte, error) {
	if n, err := r.rd.Read(r.buffer); err != nil {
		return nil, err // including unexpected io.EOF
	} else if n != r.size {
		return nil, fmt.Errorf("corrupted block: size %d unexpected", n)
	}
	return r.buffer, nil
}
