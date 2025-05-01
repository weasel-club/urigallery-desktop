package app

import (
	"encoding/binary"
	"fmt"
	"io"
	"runtime"
	"urigallery/peer"
)

type errorResponse struct {
	Error string `msgpack:"error"`
}

func createError(err error) *errorResponse {
	return &errorResponse{
		Error: err.Error(),
	}
}

type App struct {
	handlers  map[string]Handler
	semaphore chan struct{}
}

type Handler func(r *Request) (*Response, error)

func NewApp() *App {
	maxRequests := runtime.NumCPU()

	return &App{
		handlers:  make(map[string]Handler),
		semaphore: make(chan struct{}, maxRequests),
	}
}

func (a *App) AddHandler(requestType string, handler Handler) {
	a.handlers[requestType] = handler
}

func (a *App) RemoveHandler(requestType string) {
	delete(a.handlers, requestType)
}

func sendError(channel *peer.Channel, m *peer.Message, err error) {
	channel.Send(peer.NewMessage(m.ID(), newResponse().Fail().Object(createError(err)).AsData()))
}

func (a *App) HandleMessage(channel *peer.Channel, m *peer.Message) {
	var typeLength uint8
	if err := binary.Read(m.Data(), binary.BigEndian, &typeLength); err != nil {
		sendError(channel, m, err)
		return
	}

	bytes := make([]byte, typeLength)
	if _, err := io.ReadFull(m.Data(), bytes); err != nil {
		sendError(channel, m, err)
		return
	}

	requestType := string(bytes)
	handler, ok := a.handlers[requestType]
	if !ok {
		sendError(channel, m, fmt.Errorf("unknown request type: %s", requestType))
		return
	}

	var response *Response
	var err error

	func() {
		a.semaphore <- struct{}{}
		defer func() { <-a.semaphore }()
		response, err = handler(newRequest(requestType, m.Data()))
	}()

	if err != nil {
		sendError(channel, m, err)
		return
	}

	if response != nil {
		channel.Send(peer.NewMessage(m.ID(), response.AsData()))
	} else {
		channel.Send(peer.NewMessage(m.ID(), newResponse().Success().AsData()))
	}
}
