package rangetripper

import (
	. "github.com/smartystreets/goconvey/convey"

	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func ExampleRangeTripper() {
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

}

func Test_StandardDownload(t *testing.T) {
	tfile, err := ioutil.TempFile("/tmp", "sd")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a server is started that doesn't support ranges, RangeTripper downloads the content correctly", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee`)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.Write(serverBytes) // Simple write
		}))
		// Close the server when test finishes
		defer server.Close()

		// Use Client & URL from our local test server
		//l := log.New(os.Stderr, "[DEBUG] ", 0)
		//rt, err := NewWithLoggers(10, tfile.Name(), l, l)

		rt, err := New(10, tfile.Name())
		So(err, ShouldBeNil)

		req := httptest.NewRequest("GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		fileContents, ferr := ioutil.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_StandardDownloadHTTPClient(t *testing.T) {
	tfile, err := ioutil.TempFile("/tmp", "sdhc")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a server is started that doesn't support ranges, and RangeTripper is configured with http.Client, it still downloads the content correctly", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee`)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.Write(serverBytes) // Simple write
		}))
		// Close the server when test finishes
		defer server.Close()

		// Use Client & URL from our local test server
		//l := log.New(os.Stderr, "[DEBUG] ", 0)
		//rt, err := NewWithLoggers(10, tfile.Name(), l, l)

		rt, err := New(10, tfile.Name())
		rt.SetClient(new(http.Client)) // use a normal http.Client
		So(err, ShouldBeNil)

		req := httptest.NewRequest("GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		fileContents, ferr := ioutil.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_RangeDownload(t *testing.T) {
	tfile, err := ioutil.TempFile("/tmp", "rd")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	tfile2, err := ioutil.TempFile("/tmp", "rd2")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile2.Name())

	Convey("When a server is started that supports ranges, RangeTripper downloads the content correctly", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee`)
		werr := ioutil.WriteFile(tfile2.Name(), serverBytes, 0)
		So(werr, ShouldBeNil)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			http.ServeFile(rw, req, tfile2.Name()) // ServeFile sets Content-Length and Accept-Ranges
		}))
		// Close the server when test finishes
		defer server.Close()

		// Use Client & URL from our local test server
		//l := log.New(os.Stderr, "[DEBUG] ", 0)
		//rt, err := NewWithLoggers(10, tfile.Name(), l, l)

		rt, err := New(10, tfile.Name())
		So(err, ShouldBeNil)

		req := httptest.NewRequest("GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		fileContents, ferr := ioutil.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_StandardDownloadBroken(t *testing.T) {
	tfile, err := ioutil.TempFile("/tmp", "sdb")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a server is started that doesn't support ranges, and times out, retries happen, and then errors out", t, func() {
		//serverBytes := []byte(`OK I have something to say here weeeeee`)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			time.Sleep(1 * time.Second)
		}))
		// Close the server when test finishes
		defer server.Close()

		// Use Client & URL from our local test server
		//l := log.New(os.Stderr, "[DEBUG] ", 0)
		//rt, err := NewWithLoggers(10, tfile.Name(), l, l)

		rt, err := New(10, tfile.Name())
		rt.SetClient(NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond)) // custom RetryClient with short times
		So(err, ShouldBeNil)

		req := httptest.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, rerr := rt.RoundTrip(req)
		stop := time.Now()
		So(rerr, ShouldNotBeNil)
		So(stop, ShouldHappenWithin, ((3*2+1+1)*10)*time.Millisecond, start)

	})

}

func Test_StandardDownloadBrokenExp(t *testing.T) {
	tfile, err := ioutil.TempFile("/tmp", "sdbe")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a server is started that doesn't support ranges, and times out, retries happen exponentially, and then errors out", t, func() {
		//serverBytes := []byte(`OK I have something to say here weeeeee`)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			time.Sleep(1 * time.Second)
		}))
		// Close the server when test finishes
		defer server.Close()

		// Use Client & URL from our local test server
		//l := log.New(os.Stderr, "[DEBUG] ", 0)
		//rt, err := NewWithLoggers(10, tfile.Name(), l, l)

		rt, err := New(10, tfile.Name())
		rt.SetClient(NewRetryClientWithExponentialBackoff(3, 10*time.Millisecond, 10*time.Millisecond)) // custom RetryClient with short times
		So(err, ShouldBeNil)

		req := httptest.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, rerr := rt.RoundTrip(req)
		stop := time.Now()
		So(rerr, ShouldNotBeNil)
		So(stop, ShouldHappenWithin, time.Duration(int64(math.Pow(10, 3)))*time.Millisecond, start)

	})

}

func Test_StandardDownload500s(t *testing.T) {
	tfile, err := ioutil.TempFile("/tmp", "sd5s")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a server is started that doesn't support ranges, and throws 500s, retries happen, and then errors out", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee`)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write(serverBytes)
		}))
		// Close the server when test finishes
		defer server.Close()

		// Use Client & URL from our local test server
		//l := log.New(os.Stderr, "[DEBUG] ", 0)
		//rt, err := NewWithLoggers(10, tfile.Name(), l, l)

		rt, err := New(10, tfile.Name())
		rt.SetClient(NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond)) // custom RetryClient with short times
		So(err, ShouldBeNil)

		req := httptest.NewRequest("GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldNotBeNil)

	})

}
