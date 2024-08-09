package request

import (
	"io/ioutil"
	"net/http"
	"time"
)

type Request struct {
	IP         string
	TimeTaken  time.Duration // Using time.Duration for accurate representation of elapsed time
	StatusCode int
}

func FetchIP(client *http.Client, url string) (*Request, error) {
	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	duration := time.Since(start)
	return &Request{
		IP:         string(body),
		TimeTaken:  duration,
		StatusCode: resp.StatusCode,
	}, nil
}
