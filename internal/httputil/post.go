package httputil

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

func Post(url string, options map[string]string, params map[string]string, requestBody []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	for k, v := range options {
		req.Header.Set(k, v)
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("status code: %d, err msg: %s", resp.StatusCode, body))
	}

	return body, nil
}
