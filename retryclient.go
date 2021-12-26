package rangetripper

import (
	"github.com/eapache/go-resiliency/retrier"

	"fmt"
	"net/http"
	"time"
)

// RetryClient contains variables and methods to use when making smarter HTTP requests
type RetryClient struct {
	Client  *http.Client
	timeout time.Duration
	Retrier *retrier.Retrier
}

// NewRetryClient returns a RetryClient that will retry failed requests ``retries`` times, every ``every``,
// and use ``timeout`` as a timeout
func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient {

	return &RetryClient{
		Client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		Retrier: retrier.New(retrier.ConstantBackoff(retries, every), nil),
	}
}

// NewRetryClientWithExponentialBackoff returns a RetryClient that will retry failed requests ``retries`` times,
// first after ``initially`` and exponentially longer each time, and use ``timeout`` as a timeout
func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient {
	return &RetryClient{
		Client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		Retrier: retrier.New(retrier.ExponentialBackoff(retries, initially), nil),
	}
}

// Do takes a Request, and returns a Response or an error, following the rules of the RetryClient
func (w *RetryClient) Do(req *http.Request) (*http.Response, error) {
	var ret *http.Response

	try := func() error {
		resp, tryErr := w.Client.Do(req)
		if tryErr != nil {
			return tryErr
		}

		if resp.StatusCode > 299 || resp.StatusCode < 200 {
			return fmt.Errorf("non 2XX HTTP status received: %s", resp.Status)
		}

		ret = resp
		return nil
	}

	if err := w.Retrier.Run(try); err != nil {
		return nil, err
	}
	return ret, nil
}
