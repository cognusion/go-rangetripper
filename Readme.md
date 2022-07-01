

# rangetripper
`import "github.com/cognusion/go-rangetripper"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package rangetripper provides a performant http.RoundTripper that handles byte-range downloads if
the resulting HTTP server claims to support them in a HEAD request for the file. RangeTripper will
download 1/Nth of the file asynchronously with each of the ``fileChunks`` specified in a New.
N+1 actual downloaders are most likely as the +1 covers any gap from non-even division of content-length.




## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type Client](#Client)
* [type RangeTripper](#RangeTripper)
  * [func New(fileChunks int, outputFilePath string) (*RangeTripper, error)](#New)
  * [func NewWithLoggers(fileChunks int, outputFilePath string, timingLogger, debugLogger *log.Logger) (*RangeTripper, error)](#NewWithLoggers)
  * [func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error)](#RangeTripper.Do)
  * [func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)](#RangeTripper.RoundTrip)
  * [func (rt *RangeTripper) SetChunkSize(chunkBytes int64)](#RangeTripper.SetChunkSize)
  * [func (rt *RangeTripper) SetClient(client Client)](#RangeTripper.SetClient)
  * [func (rt *RangeTripper) SetMax(max int)](#RangeTripper.SetMax)
  * [func (rt *RangeTripper) WithProgress() &lt;-chan int64](#RangeTripper.WithProgress)
* [type RetryClient](#RetryClient)
  * [func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient](#NewRetryClient)
  * [func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient](#NewRetryClientWithExponentialBackoff)
  * [func (w *RetryClient) Do(req *http.Request) (*http.Response, error)](#RetryClient.Do)


#### <a name="pkg-files">Package files</a>
[client.go](https://github.com/cognusion/go-rangetripper/tree/master/client.go) [retryclient.go](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go) [rt.go](https://github.com/cognusion/go-rangetripper/tree/master/rt.go)


## <a name="pkg-constants">Constants</a>
``` go
const (
    ContentLengthNumericError   = rtError("Content-Length value cannot be converted to a number")
    ContentLengthMismatchError  = rtError("downloaded file size does not match content-length")
    SingleRequestExhaustedError = rtError("one request has already been made with this RangeTripper")
)
```
Static errors to return





## <a name="Client">type</a> [Client](https://github.com/cognusion/go-rangetripper/tree/master/client.go?s=500:568#L14)
``` go
type Client interface {
    Do(*http.Request) (*http.Response, error)
}
```
Client is an interface that could refer to an http.Client or a rangetripper.RetryClient


``` go
var DefaultClient Client = NewRetryClient(10, 2*time.Second, 60*time.Second)
```
DefaultClient is what RangeTripper will use to actually make the individual GET requests.
Change the values to change the outcome. Don't set the DefaultClient's Client.Transport
to a RangeTripper, or :mindblown:. DefaultClient can be a lowly http.Client if you prefer










## <a name="RangeTripper">type</a> [RangeTripper](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=1387:1704#L48)
``` go
type RangeTripper struct {
    TimingsOut *log.Logger
    DebugOut   *log.Logger
    // contains filtered or unexported fields
}

```
RangeTripper is an http.RoundTripper to be used in an http.Client.
This should not be used in its default state, instead by its New functions.
A single RangeTripper *must* only be used for one request.







### <a name="New">func</a> [New](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=1787:1857#L66)
``` go
func New(fileChunks int, outputFilePath string) (*RangeTripper, error)
```
New simply returns a RangeTripper or an error. Logged messages are discarded.


### <a name="NewWithLoggers">func</a> [NewWithLoggers](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=2049:2169#L71)
``` go
func NewWithLoggers(fileChunks int, outputFilePath string, timingLogger, debugLogger *log.Logger) (*RangeTripper, error)
```
NewWithLoggers returns a RangeTripper or an error. Logged messages are sent to the specified Logger, or discarded if nil.





### <a name="RangeTripper.Do">func</a> (\*RangeTripper) [Do](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=9272:9339#L294)
``` go
func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error)
```
Do is a satisfier of the rangetripper.Client interface, and is identical to RoundTrip




### <a name="RangeTripper.RoundTrip">func</a> (\*RangeTripper) [RoundTrip](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=4254:4328#L145)
``` go
func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)
```
RoundTrip is called with a formed Request, writing the Body of the Response to
to the specified output file. The Response should be ignored, but
errors are important. Both the Request.Body and the RangeTripper.outFile will be
closed when this function returns.




### <a name="RangeTripper.SetChunkSize">func</a> (\*RangeTripper) [SetChunkSize](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=3458:3512#L123)
``` go
func (rt *RangeTripper) SetChunkSize(chunkBytes int64)
```
SetChunkSize overrides the ``fileChunks`` and instead will divide the resulting Content-Length by this to
determine the appropriate chunk count dynamically. ``fileChunks`` will still be used to guide the maximum
number of concurrent workers, unless ``SetMax()`` is used.




### <a name="RangeTripper.SetClient">func</a> (\*RangeTripper) [SetClient](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=2860:2908#L105)
``` go
func (rt *RangeTripper) SetClient(client Client)
```
SetClient allows for overriding the Client used to make the requests.




### <a name="RangeTripper.SetMax">func</a> (\*RangeTripper) [SetMax](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=3014:3053#L110)
``` go
func (rt *RangeTripper) SetMax(max int)
```
SetMax allows for setting the maximum number of concurrently-running workers




### <a name="RangeTripper.WithProgress">func</a> (\*RangeTripper) [WithProgress](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=3838:3889#L134)
``` go
func (rt *RangeTripper) WithProgress() <-chan int64
```
WithProgress returns a read-only chan that will first provide the total length of the content (in bytes),
followed by a stream of completed byte-lengths. CAUTION: It is a generally bad idea to call this and then
ignore the resulting channel.




## <a name="RetryClient">type</a> [RetryClient](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=285:383#L18)
``` go
type RetryClient struct {
    // contains filtered or unexported fields
}

```
RetryClient contains variables and methods to use when making smarter HTTP requests







### <a name="NewRetryClient">func</a> [NewRetryClient](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=529:604#L26)
``` go
func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient
```
NewRetryClient returns a RetryClient that will retry failed requests ``retries`` times, every ``every``,
and use ``timeout`` as a timeout


### <a name="NewRetryClientWithExponentialBackoff">func</a> [NewRetryClientWithExponentialBackoff](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=1051:1152#L42)
``` go
func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient
```
NewRetryClientWithExponentialBackoff returns a RetryClient that will retry failed requests ``retries`` times,
first after ``initially`` and exponentially longer each time, and use ``timeout`` as a timeout





### <a name="RetryClient.Do">func</a> (\*RetryClient) [Do](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=1492:1559#L56)
``` go
func (w *RetryClient) Do(req *http.Request) (*http.Response, error)
```
Do takes a Request, and returns a Response or an error, following the rules of the RetryClient








- - -
Generated by [godoc2md](http://godoc.org/github.com/cognusion/godoc2md)
