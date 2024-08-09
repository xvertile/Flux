package roundtrip

import (
	"net/http"
	"net/url"
	"time"
)

type CustomTransport struct {
	Transport http.RoundTripper
}

func (c *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Close = true
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("Connection", "close")
	return c.Transport.RoundTrip(req)
}
func NewHTTPClient(proxyURL string) (*http.Client, error) {
	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
	}

	client := &http.Client{
		Transport: &CustomTransport{Transport: transport},
		Timeout:   20 * time.Second,
	}

	return client, nil
}
