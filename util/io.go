package util

import (
	"io"
)

type closeOnEOFReader struct {
	io.ReadCloser
}

func (r *closeOnEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(p)
	if err != nil {
		r.Close()
	}
	return n, err
}

func CloseOnEOFReader(reader io.ReadCloser) io.ReadCloser {
	return &closeOnEOFReader{ReadCloser: reader}
}
