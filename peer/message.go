package peer

import (
	"bytes"
	"encoding/binary"
	"io"
)

// 15KB
const ChunkSize = 15 * 1024

type PayloadType byte

const (
	PayloadTypeHeader    PayloadType = 1
	PayloadTypeChunk     PayloadType = 2
	PayloadTypeEnd       PayloadType = 3
	PayloadTypeHeartbeat PayloadType = 4
)

type PayloadHeader struct {
	Type PayloadType
}

type RPCPayloadHeader struct {
	PayloadHeader
	ID uint32
}

func CreatePayloadHeader(t PayloadType) PayloadHeader {
	return PayloadHeader{
		Type: t,
	}
}

func CreateRPCPayloadHeader(t PayloadType, id uint32) RPCPayloadHeader {
	return RPCPayloadHeader{
		PayloadHeader: CreatePayloadHeader(t),
		ID:            id,
	}
}

type Payload interface {
	Write(w io.Writer) error
	Read(r io.Reader) error
}

type HeaderPayload struct {
	RPCPayloadHeader
	HasLength bool
	Length    uint64
}

func (p *HeaderPayload) Write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, p.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, p.ID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, p.HasLength); err != nil {
		return err
	}
	if p.HasLength {
		if err := binary.Write(w, binary.BigEndian, p.Length); err != nil {
			return err
		}
	}
	return nil
}

func (p *HeaderPayload) Read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &p.Type); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.ID); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.HasLength); err != nil {
		return err
	}
	if p.HasLength {
		if err := binary.Read(r, binary.BigEndian, &p.Length); err != nil {
			return err
		}
	}
	return nil
}

type ChunkPayload struct {
	RPCPayloadHeader
	Data []byte
}

func (p *ChunkPayload) Write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, p.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, p.ID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, p.Data); err != nil {
		return err
	}
	return nil
}

func (p *ChunkPayload) Read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &p.Type); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.ID); err != nil {
		return err
	}

	// Read until end
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	p.Data = data
	return nil
}

type EndPayload struct {
	RPCPayloadHeader
}

func (p *EndPayload) Write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, p.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, p.ID); err != nil {
		return err
	}
	return nil
}

func (p *EndPayload) Read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &p.Type); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.ID); err != nil {
		return err
	}
	return nil
}

type HeartbeatPayload struct {
	PayloadHeader
}

func (p *HeartbeatPayload) Write(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, p.Type); err != nil {
		return err
	}
	return nil
}

func (p *HeartbeatPayload) Read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &p.Type); err != nil {
		return err
	}
	return nil
}

func Header(id uint32, hasLength bool, length uint64) *HeaderPayload {
	return &HeaderPayload{
		RPCPayloadHeader: CreateRPCPayloadHeader(PayloadTypeHeader, id),
		HasLength:        hasLength,
		Length:           length,
	}
}
func Chunk(id uint32, data []byte) *ChunkPayload {
	return &ChunkPayload{
		RPCPayloadHeader: CreateRPCPayloadHeader(PayloadTypeChunk, id),
		Data:             data,
	}
}

func End(id uint32) *EndPayload {
	return &EndPayload{
		RPCPayloadHeader: CreateRPCPayloadHeader(PayloadTypeEnd, id),
	}
}

func Heartbeat() *HeartbeatPayload {
	return &HeartbeatPayload{
		PayloadHeader: CreatePayloadHeader(PayloadTypeHeartbeat),
	}
}

func encodePayload(payload Payload) ([]byte, error) {
	buffer := new(bytes.Buffer)
	if err := payload.Write(buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func decodePayload(data []byte) (Payload, error) {
	var payloadType PayloadType
	payloadType = PayloadType(data[0])

	var payload Payload
	switch payloadType {
	case PayloadTypeHeader:
		payload = &HeaderPayload{}
	case PayloadTypeChunk:
		payload = &ChunkPayload{}
	case PayloadTypeEnd:
		payload = &EndPayload{}
	case PayloadTypeHeartbeat:
		payload = &HeartbeatPayload{}
	}

	if err := payload.Read(bytes.NewReader(data)); err != nil {
		return nil, err
	}

	return payload, nil
}

type Message struct {
	id   uint32
	data *Data
}

func NewMessage(id uint32, dataStream *Data) *Message {
	return &Message{
		id:   id,
		data: dataStream,
	}
}

func (m *Message) ID() uint32 {
	return m.id
}

func (m *Message) Data() *Data {
	return m.data
}

func (c *Channel) Send(m *Message) error {
	// Send header
	header := Header(m.id, m.data.hasLength, m.data.length)
	payload, err := encodePayload(header)
	if err != nil {
		return err
	}
	c.dataChannel.Send(payload)

	// Send chunks
	if err = m.data.writeChunks(m.id, c.dataChannel); err != nil {
		return err
	}

	// Send end
	end := End(m.id)
	payload, err = encodePayload(end)
	if err != nil {
		return err
	}
	c.dataChannel.Send(payload)

	return nil
}

type Data struct {
	reader    io.Reader
	length    uint64
	hasLength bool
}

func (d *Data) Read(p []byte) (n int, err error) {
	return d.reader.Read(p)
}

func (d *Data) writeChunks(messageId uint32, w io.Writer) error {
	buffer := make([]byte, ChunkSize)
	for {
		n, err := d.reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if n > 0 {
			payload, err := encodePayload(Chunk(messageId, buffer[:n]))
			if err != nil {
				return err
			}
			if _, err := w.Write(payload); err != nil {
				return err
			}
		}
	}
}

func NewData(
	reader io.Reader,
	hasLength bool,
	length uint64,
) *Data {
	return &Data{
		reader:    reader,
		hasLength: hasLength,
		length:    length,
	}
}
