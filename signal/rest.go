package signal

import (
	"fmt"

	"resty.dev/v3"
)

const (
	signalServerBaseURL = "https://signal.goorm.me"
)

var restClient *resty.Client
var lastToken string

func getRestClient(token string) *resty.Client {
	if restClient == nil || lastToken != token {
		if restClient != nil {
			restClient.Close()
		}
		restClient = resty.New().SetBaseURL(signalServerBaseURL)
		lastToken = token
	}

	if token != "" {
		restClient.SetHeader("Authorization", "Bearer "+token)
	}

	return restClient
}

type errorResponse struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

type successResponse[T any] struct {
	Type string `json:"type"`
	Data T      `json:"data"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (c *Client) register() (*loginResponse, error) {
	res, err := getRestClient(c.token).R().SetResult(&successResponse[loginResponse]{}).SetError(&errorResponse{}).Post("/auth")
	if err != nil {
		return nil, err
	}

	if res.IsError() {
		return nil, fmt.Errorf("register failed: %s", res.Error().(*errorResponse).Error)
	}

	return &res.Result().(*successResponse[loginResponse]).Data, nil
}

type getAuthResponse struct {
	UID string `json:"uid"`
}

func (c *Client) getAuth() (*getAuthResponse, error) {
	res, err := getRestClient(c.token).R().SetResult(&successResponse[getAuthResponse]{}).SetError(&errorResponse{}).Get("/auth")
	if err != nil {
		return nil, err
	}

	if res.IsError() {
		return nil, fmt.Errorf("get auth failed: %s", res.Error().(*errorResponse).Error)
	}

	return &res.Result().(*successResponse[getAuthResponse]).Data, nil
}

type createOTPResponse struct {
	OTP       string `json:"otp"`
	ExpiresAt int    `json:"expiresAt"`
}

func (c *Client) createOTP() (*createOTPResponse, error) {
	res, err := getRestClient(c.token).R().SetResult(&successResponse[createOTPResponse]{}).SetError(&errorResponse{}).Post("/auth/otp")
	if err != nil {
		return nil, err
	}

	return &res.Result().(*successResponse[createOTPResponse]).Data, nil
}

type SessionEvent any

type SessionEventConnection struct {
	SDP string `json:"sdp"`
	ID  string `json:"id"`
}

func (c *Client) openSession(onEvent func(event SessionEvent)) error {
	es := resty.NewEventSource().SetURL(signalServerBaseURL+"/sessions/connections").
		SetHeader("Authorization", "Bearer "+c.token).
		OnMessage(func(data any) {}, nil).
		AddEventListener("connection", func(data any) {
			go onEvent(data.(*SessionEventConnection))
		}, &SessionEventConnection{})
	err := es.Get()
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) acceptSession(id string, sdp string) error {
	_, err := getRestClient(c.token).R().
		SetBody(map[string]string{"sdp": sdp}).
		SetError(&errorResponse{}).
		Post(fmt.Sprintf("/sessions/connections/%s/accept", id))
	if err != nil {
		return err
	}
	return nil
}
