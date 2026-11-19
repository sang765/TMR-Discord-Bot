package sources

import (
	"io"
	"net/http"
)

// newRequest builds a GET/POST request with our UA.
func newRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	return req, nil
}

// doRequest performs req, returns body (max 4MB) and status code.
func doRequest(req *http.Request) (string, int, error) {
	resp, err := HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(b), resp.StatusCode, nil
}
