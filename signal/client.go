package signal

import (
	"errors"
	"log"
	"sync"
	"time"
	"urigallery/peer"

	"github.com/pion/webrtc/v4"
)

type OnChannelHandler func(channel *peer.Channel)

type Client struct {
	uid   string
	token string

	channels     map[string][]*peer.Channel
	channelsLock sync.Mutex
}

func NewClient() *Client {
	return &Client{
		channels: make(map[string][]*peer.Channel),
	}
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) ResetToken() {
	c.token = ""
}

func (c *Client) Login() (string, error) {
	if c.token == "" {
		res, err := c.register()
		if err != nil {
			return "", err
		}
		c.token = res.Token
	}

	res, err := c.getAuth()
	if err != nil {
		c.ResetToken()
		return c.Login()
	}
	c.uid = res.UID

	return c.token, nil
}

func (c *Client) UID() string {
	return c.uid
}

func (c *Client) establishChannel(e *SessionEventConnection) (*peer.Channel, error) {
	// Create peer connection
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	channel := peer.NewChannel(e.ID, pc)

	// Handshake
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  e.SDP,
	}); err != nil {
		log.Printf("failed to set remote description: %s", err)
		return nil, err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("failed to create answer: %s", err)
		return nil, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		log.Printf("failed to set local description: %s", err)
		return nil, err
	}

	// Wait for candidate gathering complete
	<-gatherComplete

	// Response with SDP
	if err := c.acceptSession(e.ID, pc.LocalDescription().SDP); err != nil {
		log.Printf("failed to accept session: %s", err)
		return nil, err
	}

	// Wait for channel to be established or timeout
	timeout := time.After(5 * time.Second)

	select {
	case <-timeout:
		channel.Close()
		return nil, errors.New("connection timed out")
	case <-channel.WaitForConnection():
		return channel, nil
	}
}

func (c *Client) addChannel(e *SessionEventConnection, channel *peer.Channel) {
	c.channelsLock.Lock()
	defer c.channelsLock.Unlock()
	c.channels[e.ID] = append(c.channels[e.ID], channel)
}

func (c *Client) OpenSession(onChannel OnChannelHandler) error {
	return c.openSession(func(event SessionEvent) {
		switch e := event.(type) {
		case *SessionEventConnection:
			channel, err := c.establishChannel(e)
			if err != nil {
				log.Printf("failed to establish channel: %s", err)
				return
			}
			c.addChannel(e, channel)
			onChannel(channel)
		}
	})
}

func (c *Client) CloseSession() {
	c.channelsLock.Lock()
	defer c.channelsLock.Unlock()
	for _, channels := range c.channels {
		for _, channel := range channels {
			channel.Close()
		}
	}
}

func (c *Client) CreateOTP() (string, error) {
	res, err := c.createOTP()
	if err != nil {
		return "", err
	}
	return res.OTP, nil
}
