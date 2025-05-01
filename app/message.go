package app

import (
	"bytes"
	"io"
	"urigallery/peer"

	"github.com/vmihailenco/msgpack/v5"
)

type Request struct {
	*peer.Data

	Type string
}

func Object[T any](r *Request) (T, error) {
	var template T

	b, err := io.ReadAll(r)
	if err != nil {
		return template, err
	}

	err = msgpack.Unmarshal(b, &template)
	if err != nil {
		return template, err
	}
	return template, nil
}

func newRequest(requestType string, dataStream *peer.Data) *Request {
	return &Request{
		Data: dataStream,
		Type: requestType,
	}
}

type Response struct {
	readers          []io.Reader
	hasUnknownLength bool
	length           uint64
}

func newResponse() *Response {
	return &Response{
		readers:          []io.Reader{},
		hasUnknownLength: false,
		length:           0,
	}
}

func (r *Response) AppendWithLength(reader io.Reader, length uint64) *Response {
	r.readers = append(r.readers, reader)
	if !r.hasUnknownLength {
		r.length += length
	}
	return r
}

func (r *Response) AppendWithoutLength(reader io.Reader) *Response {
	r.readers = append(r.readers, reader)
	r.hasUnknownLength = true
	r.length = 0
	return r
}

func (r *Response) Reader(reader io.Reader, length ...uint64) *Response {
	if len(length) > 0 {
		return r.AppendWithLength(reader, length[0])
	} else {
		return r.AppendWithoutLength(reader)
	}
}

func (r *Response) Bytes(b []byte) *Response {
	return r.AppendWithLength(bytes.NewBuffer(b), uint64(len(b)))
}

func (r *Response) Object(object any) *Response {
	b, err := msgpack.Marshal(object)
	if err != nil {
		return r
	}
	return r.AppendWithLength(bytes.NewBuffer(b), uint64(len(b)))
}

func (r *Response) Code(code uint8) *Response {
	return r.Bytes([]byte{code})
}

func (r *Response) Fail() *Response {
	return r.Code(1)
}

func (r *Response) Success() *Response {
	return r.Code(0)
}

func (r *Response) AsData() *peer.Data {
	return peer.NewData(io.MultiReader(r.readers...), !r.hasUnknownLength, r.length)
}
