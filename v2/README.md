

# rangetripper
`import "github.com/cognusion/go-rangetripper/v2"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

## <a name="pkg-overview">Overview</a>
Package rangetripper provides a performant http.RoundTripper that handles byte-range downloads if
the resulting HTTP server claims to support them in a HEAD request for the file. RangeTripper will
download 1/Nth of the file asynchronously with each of the “fileChunks“ specified in a New.
N+1 actual downloaders are most likely as the +1 covers any gap from non-even division of content-length.




## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [func WithOutfile(parent context.Context, outFilePath string) context.Context](#WithOutfile)
* [func WithProgressChan(parent context.Context, progressChan chan int64) context.Context](#WithProgressChan)
* [type Client](#Client)
* [type RangeTripper](#RangeTripper)
  * [func New(fileChunks int) (*RangeTripper, error)](#New)
  * [func NewWithLoggers(fileChunks int, timingLogger, debugLogger *log.Logger) (*RangeTripper, error)](#NewWithLoggers)
  * [func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error)](#RangeTripper.Do)
  * [func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)](#RangeTripper.RoundTrip)
  * [func (rt *RangeTripper) SetChunkSize(chunkBytes int64)](#RangeTripper.SetChunkSize)
  * [func (rt *RangeTripper) SetClient(client Client)](#RangeTripper.SetClient)
  * [func (rt *RangeTripper) SetMax(max int)](#RangeTripper.SetMax)
* [type RetryClient](#RetryClient)
  * [func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient](#NewRetryClient)
  * [func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient](#NewRetryClientWithExponentialBackoff)
  * [func (w *RetryClient) Do(req *http.Request) (*http.Response, error)](#RetryClient.Do)

#### <a name="pkg-examples">Examples</a>
* [RangeTripper](#example-rangetripper)

#### <a name="pkg-files">Package files</a>
[client.go](https://github.com/cognusion/go-rangetripper/tree/master/v2/client.go) [retryclient.go](https://github.com/cognusion/go-rangetripper/tree/master/v2/retryclient.go) [rt.go](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go)


## <a name="pkg-constants">Constants</a>
``` go
const (
    ContentLengthNumericError  = rtError("Content-Length value cannot be converted to a number")
    ContentLengthMismatchError = rtError("downloaded file size does not match content-length")

    // OutfileKey is used in an http.Request's Context.WithValue to specify a file to write the fetched web object to, instead of a buffer.
    OutfileKey contextIDKey = iota
    // ProgressChanKey is used in an http.Request's Context.WithValue to pass a chan int64 where RoundTrip will push bytes-written progress updates.
    // The first message to this chan will be either the content-length (if known) or 0 if not.
    ProgressChanKey
)
```
Static errors to return




## <a name="WithOutfile">func</a> [WithOutfile](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=16189:16265#L515)
``` go
func WithOutfile(parent context.Context, outFilePath string) context.Context
```
WithOutfile returns a Context with a properly set OutfileKey.



## <a name="WithProgressChan">func</a> [WithProgressChan](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=16405:16491#L520)
``` go
func WithProgressChan(parent context.Context, progressChan chan int64) context.Context
```
WithProgressChan returns a Context with a properly set ProgressChanKey.




## <a name="Client">type</a> [Client](https://github.com/cognusion/go-rangetripper/tree/master/v2/client.go?s=500:568#L14)
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










## <a name="RangeTripper">type</a> [RangeTripper](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=2314:2441#L75)
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



##### Example RangeTripper:
``` go
// Set up a temporary file
tfile, err := os.CreateTemp("/tmp", "rt")
if err != nil {
    panic(err)
}
defer os.Remove(tfile.Name()) // clean up after ourselves

client := new(http.Client) // make a new Client
rt, _ := New(10)           // make a new RangeTripper (errors ignored for brevity. Don't be dumb)
    client.Transport = rt      // Use the RangeTripper as the Transport

    ctx := WithOutfile(context.Background(), tfile.Name())
    req, err := http.NewRequestWithContext(ctx, "GET", "https://google.com/", nil)
    if err != nil {
        panic(err)
    }

    if _, err := client.Do(req); err != nil {
        panic(err)
    }
    // tfile is the google homepage
```





### <a name="New">func</a> [New](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=2623:2670#L87)
``` go
func New(fileChunks int) (*RangeTripper, error)
```
New returns a RangeTripper or an error. Logged messages are discarded.

fileChunks is the number of pieces to divide the dowloaded file into (+/- 1). Overridden by SetMax.


### <a name="NewWithLoggers">func</a> [NewWithLoggers](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=3076:3173#L98)
``` go
func NewWithLoggers(fileChunks int, timingLogger, debugLogger *log.Logger) (*RangeTripper, error)
```
NewWithLoggers returns a RangeTripper or an error. Logged messages are sent to the specified Logger, or discarded if nil.

fileChunks is the number of pieces to divide the dowloaded file into (+/- 1). Overridden by SetMax.

timingLogger is a logger to send timing-related messages to.

debugLogger is a logger to send debug messages to.





### <a name="RangeTripper.Do">func</a> (\*RangeTripper) [Do](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=10514:10581#L328)
``` go
func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error)
```
Do is a satisfier of the rangetripper.Client interface, and is identical to RoundTrip




### <a name="RangeTripper.RoundTrip">func</a> (\*RangeTripper) [RoundTrip](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=4724:4798#L154)
``` go
func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)
```
RoundTrip is called with a formed Request.

The following Context Key/Values impact the RoundTrip:


	OutfileKey: The value of that is assumed to be a file path path that is where the file should be written to.
	ProgressChanKey: The value is assumed to be a chan int64 where RoundTrip will push bytes-written progress updates.
	  The first message to this chan will be either the content-length (if known) or 0 if not.




### <a name="RangeTripper.SetChunkSize">func</a> (\*RangeTripper) [SetChunkSize](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=4161:4215#L139)
``` go
func (rt *RangeTripper) SetChunkSize(chunkBytes int64)
```
SetChunkSize overrides the “fileChunks“ and instead will divide the resulting Content-Length by this to
determine the appropriate chunk count dynamically. “fileChunks“ will still be used to guide the maximum
number of concurrent workers, unless “SetMax()“ is used.




### <a name="RangeTripper.SetClient">func</a> (\*RangeTripper) [SetClient](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=3631:3679#L124)
``` go
func (rt *RangeTripper) SetClient(client Client)
```
SetClient allows for overriding the Client used to make the requests.




### <a name="RangeTripper.SetMax">func</a> (\*RangeTripper) [SetMax](https://github.com/cognusion/go-rangetripper/tree/master/v2/rt.go?s=3785:3824#L129)
``` go
func (rt *RangeTripper) SetMax(max int)
```
SetMax allows for setting the maximum number of concurrently-running workers




## <a name="RetryClient">type</a> [RetryClient](https://github.com/cognusion/go-rangetripper/tree/master/v2/retryclient.go?s=285:383#L18)
``` go
type RetryClient struct {
    // contains filtered or unexported fields
}

```
RetryClient contains variables and methods to use when making smarter HTTP requests







### <a name="NewRetryClient">func</a> [NewRetryClient](https://github.com/cognusion/go-rangetripper/tree/master/v2/retryclient.go?s=529:604#L26)
``` go
func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient
```
NewRetryClient returns a RetryClient that will retry failed requests ``retries`` times, every ``every``,
and use ``timeout`` as a timeout


### <a name="NewRetryClientWithExponentialBackoff">func</a> [NewRetryClientWithExponentialBackoff](https://github.com/cognusion/go-rangetripper/tree/master/v2/retryclient.go?s=1051:1152#L42)
``` go
func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient
```
NewRetryClientWithExponentialBackoff returns a RetryClient that will retry failed requests ``retries`` times,
first after ``initially`` and exponentially longer each time, and use ``timeout`` as a timeout





### <a name="RetryClient.Do">func</a> (\*RetryClient) [Do](https://github.com/cognusion/go-rangetripper/tree/master/v2/retryclient.go?s=1492:1559#L56)
``` go
func (w *RetryClient) Do(req *http.Request) (*http.Response, error)
```
Do takes a Request, and returns a Response or an error, following the rules of the RetryClient








- - -
Generated by [godoc2md](http://github.com/cognusion/godoc2md)
