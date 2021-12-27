

# rangetripper
`import "github.com/cognusion/go-rangetripper"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

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
  * [func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error)](#RangeTripper.Do)
  * [func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error)](#RangeTripper.RoundTrip)
  * [func (rt *RangeTripper) SetClient(client Client)](#RangeTripper.SetClient)
* [type RetryClient](#RetryClient)
  * [func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient](#NewRetryClient)
  * [func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient](#NewRetryClientWithExponentialBackoff)
  * [func (w *RetryClient) Do(req *http.Request) (*http.Response, error)](#RetryClient.Do)

#### <a name="pkg-examples">Examples</a>
* [RangeTripper](#example-rangetripper)

#### <a name="pkg-files">Package files</a>
[client.go](https://github.com/cognusion/go-rangetripper/tree/master/client.go) [readall.go](https://github.com/cognusion/go-rangetripper/tree/master/readall.go) [retryclient.go](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go) [rt.go](https://github.com/cognusion/go-rangetripper/tree/master/rt.go)


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



##### Example RangeTripper:
``` go
// Set up a temporary file
tfile, err := ioutil.TempFile("/tmp", "rt")
if err != nil {
    panic(err)
}
defer os.Remove(tfile.Name()) // clean up after ourselves

client := new(http.Client)     // make a new Client
rt, _ := New(10, tfile.Name()) // make a new RangeTripper (errors ignored for brevity. Don't be dumb)
    client.Transport = rt          // Use the RangeTripper as the Transport

    if _, err := client.Get("https://google.com/"); err != nil {
        panic(err)
    }
    // tfile is the google homepage
```





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





### <a name="RangeTripper.Do">func</a> (\*RangeTripper) [Do](https://github.com/cognusion/go-rangetripper/tree/master/rt.go?s=5373:5440#L182)
``` go
func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error)
```
Do is a satisfier of the rangetripper.Client interface, and is identical to RoundTrip




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




## <a name="RetryClient">type</a> [RetryClient](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=193:291#L12)
``` go
type RetryClient struct {
    // contains filtered or unexported fields
}

```
RetryClient contains variables and methods to use when making smarter HTTP requests







### <a name="NewRetryClient">func</a> [NewRetryClient](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=437:512#L20)
``` go
func NewRetryClient(retries int, every, timeout time.Duration) *RetryClient
```
NewRetryClient returns a RetryClient that will retry failed requests ``retries`` times, every ``every``,
and use ``timeout`` as a timeout


### <a name="NewRetryClientWithExponentialBackoff">func</a> [NewRetryClientWithExponentialBackoff](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=895:996#L33)
``` go
func NewRetryClientWithExponentialBackoff(retries int, initially, timeout time.Duration) *RetryClient
```
NewRetryClientWithExponentialBackoff returns a RetryClient that will retry failed requests ``retries`` times,
first after ``initially`` and exponentially longer each time, and use ``timeout`` as a timeout





### <a name="RetryClient.Do">func</a> (\*RetryClient) [Do](https://github.com/cognusion/go-rangetripper/tree/master/retryclient.go?s=1272:1339#L44)
``` go
func (w *RetryClient) Do(req *http.Request) (*http.Response, error)
```
Do takes a Request, and returns a Response or an error, following the rules of the RetryClient








- - -
Generated by [godoc2md](http://godoc.org/github.com/cognusion/godoc2md)
