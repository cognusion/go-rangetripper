package rangetripper

import (
	"errors"

	"github.com/eapache/go-resiliency/retrier"

	"fmt"
	"net/http"
	"time"
)

var (
	ErrStatusNope error = errors.New("non-retriable HTTP status received")
)

// RetryClient contains variables and methods to use when making smarter HTTP requests
type RetryClient struct {
	client  *http.Client
	timeout time.Duration
	retrier *retrier.Retrier
}

// NewRetryClient returns a RetryClient that will retry failed requests ``retries`` times, every ``every``,
// and use ``timeout`` as a timeout
func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient {

	b := make(retrier.BlacklistClassifier, 1)
	b[0] = ErrStatusNope

	return &RetryClient{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		retrier: retrier.New(retrier.ConstantBackoff(retries, every), b),
	}
}

// NewRetryClientWithExponentialBackoff returns a RetryClient that will retry failed requests ``retries`` times,
// first after ``initially`` and exponentially longer each time, and use ``timeout`` as a timeout
func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient {
	b := make(retrier.BlacklistClassifier, 1)
	b[0] = ErrStatusNope

	return &RetryClient{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		retrier: retrier.New(retrier.ExponentialBackoff(retries, initially), b),
	}
}

// Do takes a Request, and returns a Response or an error, following the rules of the RetryClient
func (w *RetryClient) Do(req *http.Request) (*http.Response, error) {
	var ret *http.Response

	try := func() error {
		resp, tryErr := w.client.Do(req)
		if tryErr != nil {
			return tryErr
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return ErrStatusNope
		} else if resp.StatusCode >= 300 || resp.StatusCode < 200 {
			return fmt.Errorf("non 2XX HTTP status received: %s", resp.Status)
		}

		ret = resp
		return nil
	}

	if err := w.retrier.Run(try); err != nil {
		return nil, err
	}
	return ret, nil
}
