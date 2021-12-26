

# rangetripper
`import "github.com/cognusion/go-rangetripper"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package rangetripper provides a performant http.RoundTripper that handles byte-range downloads if
the resulting HTTP server claims to support them in a HEAD request for the file. RangeTripper will
download 1/Nth of the file asynchronously with each of the ``parallelDownloads`` specified in a New.
N+1 actual downloaders are most likely as the +1 covers any gap from non-even division of content-length.




## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [func ReadAll(r io.Reader) (b []byte, err error)](#ReadAll)
* [type Client](#Client)
* [type RangeTripper](#RangeTripper)
  * [func New(parallelDownloads int, outputFilePath string) (*RangeTripper, error)](#New)
  * [func NewWithLoggers(parallelDownloads int, outputFilePath string, timingLogger, debugLogger *log.Logger) (*RangeTripper, error)](#NewWithLoggers)
  * [func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)](#RangeTripper.RoundTrip)
  * [func (rt *RangeTripper) SetClient(client Client)](#RangeTripper.SetClient)
* [type RetryClient](#RetryClient)
  * [func NewClient(retries int, every, timeout time.Duration) *RetryClient](#NewClient)
  * [func NewClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient](#NewClientWithExponentialBackoff)
  * [func (w *RetryClient) Do(req *http.Request) (*http.Response, error)](#RetryClient.Do)


#### <a name="pkg-files">Package files</a>
[client.go](https://github.com/cognusion/go-rangetripper/tree/master/client.go) [readall.go](https://github.com/cognusion/go-rangetripper/tree/master/readall.go) [rt.go](https://github.com/cognusion/go-rangetripper/tree/master/rt.go)


## <a name="pkg-constants">Constants</a>
``` go
const (
    ContentLengthNumericError  = rtError("Content-Length value cannot be converted to a number")
    ContentLengthMismatchError = rtError("Downloaded file size does not match content-length")
)
```
Static errors to return




## <a name="ReadAll">func</a> [ReadAll](https://github.com/cognusion/go-rangetripper/tree/master/readall.go?s=349:396#L23)
``` go
func ReadAll(r io.Reader) (b []byte, err error)
```
ReadAll is a custom version of io/ioutil.ReadAll() that uses a sync.Pool of bytes.Buffer to rock the reading,
with Zero allocs and 7x better performance




## <a name="Client">type</a> [Client](https://github.com/cognusion/go-rangetripper/tree/master/client.go?s=542:610#L17)
``` go
type Client interface {
    Do(*http.Request) (*http.Response, error)
}
```
Client is an interface that could refer to an http.Client or a rangetripper.RetryClient


``` go
var DefaultClient Client = NewClient(10, 2*time.Second, 60*time.Second)
```
DefaultClient is what RangeTripper will use to actually make the individual GET requests.
Change the values to change the outcome. Don't set the DefaultClient's Client.Transport
to a RangeTripper, or :mindblown:. DefaultClient can be an http.Client if you prefer










## <a name="RangeTripper">type</a> [RangeTripper](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=1223:1387#L43)
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







### <a name="New">func</a> [New](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=1470:1547#L55)
``` go
func New(parallelDownloads int, outputFilePath string) (*RangeTripper, error)
```
New simply returns a RangeTripper or an error. Logged messages are discarded.


### <a name="NewWithLoggers">func</a> [NewWithLoggers](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=1746:1873#L60)
``` go
func NewWithLoggers(parallelDownloads int, outputFilePath string, timingLogger, debugLogger *log.Logger) (*RangeTripper, error)
```
NewWithLoggers returns a RangeTripper or an error. Logged messages are sent to the specified Logger, or discarded if nil.





### <a name="RangeTripper.RoundTrip">func</a> (\*RangeTripper) [RoundTrip](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=2789:2863#L100)
``` go
func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)
```
RoundTrip is called with a formed Request, writing the Body of the response to
to the specified output file. The response should largely be ignored, but
errors are important.




### <a name="RangeTripper.SetClient">func</a> (\*RangeTripper) [SetClient](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=2531:2579#L93)
``` go
func (rt *RangeTripper) SetClient(client Client)
```
SetClient allows for overriding the Client used to make the requests.




## <a name="RetryClient">type</a> [RetryClient](https://github.com/cognusion/go-rangetripper/tree/master/client.go?s=699:797#L22)
``` go
type RetryClient struct {
    Client *http.Client

    Retrier *retrier.Retrier
    // contains filtered or unexported fields
}

```
RetryClient contains variables and methods to use when making smarter HTTP requests







### <a name="NewClient">func</a> [NewClient](https://github.com/cognusion/go-rangetripper/tree/master/client.go?s=857:927#L29)
``` go
func NewClient(retries int, every, timeout time.Duration) *RetryClient
```
NewClient returns a Client with the specified settings


### <a name="NewClientWithExponentialBackoff">func</a> [NewClientWithExponentialBackoff](https://github.com/cognusion/go-rangetripper/tree/master/client.go?s=1179:1275#L41)
``` go
func NewClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient
```
NewClientWithExponentialBackoff returns a Client with the specified settings





### <a name="RetryClient.Do">func</a> (\*RetryClient) [Do](https://github.com/cognusion/go-rangetripper/tree/master/client.go?s=1532:1599#L52)
``` go
func (w *RetryClient) Do(req *http.Request) (*http.Response, error)
```
Do takes a Request, and returns a Response or an error, following the rules








- - -
Generated by [godoc2md](http://godoc.org/github.com/cognusion/godoc2md)
