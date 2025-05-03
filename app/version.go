package app

const (
	Version = 1
)

type GetVersionResponse struct {
	Version int `msgpack:"version"`
}

func GetVersion(request *Request) (*Response, error) {
	return newResponse().Success().Object(&GetVersionResponse{
		Version: Version,
	}), nil
}
