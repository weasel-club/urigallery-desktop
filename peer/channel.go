package peer

import (
	"errors"
	"io"
	"runtime"
	"sync"
	"time"
	"urigallery/util"

	"github.com/bep/debounce"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
	"github.com/pion/webrtc/v4"
)

type MessageHandler func(message *Message)
type CloseHandler func()

type Channel struct {
	ID string

	peerConnection *webrtc.PeerConnection

	dataChannel     *DataChannel
	dataChannelLock sync.Mutex
	dataChannelWait chan struct{}

	messageHandlers []MessageHandler
	closeHandlers   []CloseHandler

	dataWriters map[uint32]io.WriteCloser

	lastHeartbeat     time.Time
	heartbeatLock     sync.Mutex
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	heartbeatTicker   *time.Ticker

	gcDebounce func(func())

	closed     bool
	closedLock sync.Mutex
}

func (c *Channel) heartbeat() {
	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()

	c.lastHeartbeat = time.Now()
}

func (c *Channel) IsClosed() bool {
	c.closedLock.Lock()
	defer c.closedLock.Unlock()
	return c.closed
}

func (c *Channel) IsTimeout() bool {
	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
	return time.Since(c.lastHeartbeat) > c.heartbeatTimeout
}

func (c *Channel) checkHeartbeat() bool {
	if c.IsClosed() {
		return false
	}

	if c.IsTimeout() {
		c.Close()
		return false
	}

	return true
}

func (c *Channel) startCheckHeartbeat() {
	for {
		<-c.heartbeatTicker.C
		if !c.checkHeartbeat() {
			return
		}
	}
}

type DataChannel struct {
	*webrtc.DataChannel
}

func (d *DataChannel) Write(p []byte) (n int, err error) {
	if d.DataChannel == nil {
		return 0, errors.New("not connected")
	}

	if err := d.DataChannel.Send(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *Channel) tryMarkClosed() bool {
	c.closedLock.Lock()
	defer c.closedLock.Unlock()

	if c.closed {
		return false
	}
	c.closed = true
	return true
}

func (c *Channel) Close() {
	if !c.tryMarkClosed() {
		return
	}

	c.peerConnection.Close()

	c.dataChannelLock.Lock()
	defer c.dataChannelLock.Unlock()
	if c.dataChannel != nil {
		c.dataChannel.Close()
	}

	c.heartbeatTicker.Stop()

	for _, handler := range c.closeHandlers {
		handler()
	}

	c.requestGC()
	c.messageHandlers = nil
	c.closeHandlers = nil
	c.peerConnection = nil
	c.dataChannel = nil
	c.dataWriters = nil
	c.dataChannelWait = nil
	c.heartbeatTicker = nil
	c.gcDebounce = nil
}

func (c *Channel) requestGC() {
	c.gcDebounce(func() {
		runtime.GC()
	})
}

func (c *Channel) OnMessage(handler MessageHandler) {
	c.messageHandlers = append(c.messageHandlers, handler)
}

func (c *Channel) OnClose(handler CloseHandler) {
	c.closeHandlers = append(c.closeHandlers, handler)
}

func (c *Channel) handleMessage(message *Message) {
	for _, handler := range c.messageHandlers {
		go func(handler MessageHandler) {
			handler(message)
		}(handler)
	}
	c.requestGC()
}

func (c *Channel) WaitForConnection() <-chan struct{} {
	return c.dataChannelWait
}

func NewChannel(
	id string,
	pc *webrtc.PeerConnection,
) *Channel {
	const (
		heartbeatInterval = 10 * time.Second
		heartbeatTimeout  = 30 * time.Second
	)

	c := &Channel{
		ID:              id,
		peerConnection:  pc,
		dataChannelWait: make(chan struct{}),
		dataWriters:     make(map[uint32]io.WriteCloser),

		heartbeatInterval: heartbeatInterval,
		heartbeatTimeout:  heartbeatTimeout,
		heartbeatTicker:   time.NewTicker(heartbeatInterval),
		lastHeartbeat:     time.Now(),

		gcDebounce: debounce.New(time.Second * 5),
	}

	// Data channel handler
	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		c.dataChannelLock.Lock()
		defer c.dataChannelLock.Unlock()
		if c.dataChannel != nil {
			return
		}

		c.dataChannel = &DataChannel{DataChannel: dataChannel}

		// Request handler
		c.dataChannel.OnMessage(func(dataChannelMessage webrtc.DataChannelMessage) {
			payload, err := decodePayload(dataChannelMessage.Data)
			if err != nil {
				return
			}

			switch payload := payload.(type) {
			case *HeaderPayload:
				reader, writer := nio.Pipe(buffer.New(ChunkSize * 10))
				data := NewData(util.CloseOnEOFReader(reader), payload.HasLength, payload.Length)
				message := NewMessage(payload.ID, data)
				c.dataWriters[payload.ID] = writer
				go c.handleMessage(message)

			case *ChunkPayload:
				writer, ok := c.dataWriters[payload.ID]
				if !ok {
					return
				}
				writer.Write(payload.Data)

			case *EndPayload:
				writer, ok := c.dataWriters[payload.ID]
				if !ok {
					return
				}
				writer.Close()
				delete(c.dataWriters, payload.ID)

			case *HeartbeatPayload:
				c.heartbeat()
			}
		})

		c.dataChannel.OnClose(func() {
			c.Close()
		})

		close(c.dataChannelWait)
	})

	go c.startCheckHeartbeat()

	return c
}
