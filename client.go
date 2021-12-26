package rangetripper

import (
	"net/http"
	"time"
)

// DefaultClient is what RangeTripper will use to actually make the individual GET requests.
// Change the values to change the outcome. Don't set the DefaultClient's Client.Transport
// to a RangeTripper, or :mindblown:. DefaultClient can be a lowly http.Client if you prefer
var DefaultClient Client = NewRetryClient(10, 2*time.Second, 60*time.Second)

// Client is an interface that could refer to an http.Client or a rangetripper.RetryClient
type Client interface {
	Do(*http.Request) (*http.Response, error)
}
