package rangetripper

import (
	"bytes"
	"io"

	"github.com/fortytw2/leaktest"
	. "github.com/smartystreets/goconvey/convey"

	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func ExampleRangeTripper() {
	// Set up a temporary file
	tfile, err := os.CreateTemp("/tmp", "rt")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name()) // clean up after ourselves

	client := new(http.Client) // make a new Client
	rt, _ := New(10)           // make a new RangeTripper (errors ignored for brevity. Don't be dumb)
	client.Transport = rt      // Use the RangeTripper as the Transport

	req, err := http.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", "https://google.com/", nil)
	if err != nil {
		panic(err)
	}

	if _, err := client.Do(req); err != nil {
		panic(err)
	}
	// tfile is the google homepage

}

func Test_StandardDownload(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "sd")
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

		rt, err := New(10)
		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_StandardDownloadHTTPClient(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "sdhc")
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

		rt, err := New(10)
		rt.SetClient(new(http.Client)) // use a normal http.Client
		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_RangeDownloadFile(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "rd")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	tfile2, err := os.CreateTemp("/tmp", "rdx")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile2.Name())

	Convey("When a server is started that supports ranges, RangeTripper downloads the content correctly to a file", t, func(c C) {
		serverBytes := []byte(`OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee`)
		werr := os.WriteFile(tfile2.Name(), serverBytes, 0)
		So(werr, ShouldBeNil)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			http.ServeFile(rw, req, tfile2.Name()) // ServeFile sets Content-Length and Accept-Ranges
		}))
		// Close the server when test finishes
		defer server.Close()

		rt, err := New(10)
		So(err, ShouldBeNil)

		progressChan := make(chan int64)

		req := httptest.NewRequestWithContext(context.WithValue(context.WithValue(context.Background(), progressChanKey, progressChan), outfileKey, tfile.Name()), "GET", server.URL, nil)

		// Check the progress
		done := make(chan interface{})
		go func(x C, p <-chan int64) {

			contentLength := <-p // first item is the contentLength
			var count int64
			for {
				select {
				case <-done:
					//x.Printf("\nSo %d ShouldEqual %d\n", count, contentLength)
					x.So(count, ShouldEqual, contentLength)
					return
				case b := <-p:
					count += b
				}
			}

		}(c, progressChan)

		_, rerr := rt.RoundTrip(req) // Run the request
		close(done)                  // Close the done chan

		So(rerr, ShouldBeNil)
		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_RangeDownloadBuffer(t *testing.T) {
	defer leaktest.Check(t)()

	tfile2, err := os.CreateTemp("/tmp", "rdx")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile2.Name())

	Convey("When a server is started that supports ranges, RangeTripper downloads the content correctly to a Buffer", t, func(c C) {
		serverBytes := []byte(`OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee`)
		werr := os.WriteFile(tfile2.Name(), serverBytes, 0)
		So(werr, ShouldBeNil)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			http.ServeFile(rw, req, tfile2.Name()) // ServeFile sets Content-Length and Accept-Ranges
		}))
		// Close the server when test finishes
		defer server.Close()

		rt, err := New(10)
		So(err, ShouldBeNil)

		progressChan := make(chan int64)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), progressChanKey, progressChan), "GET", server.URL, nil)

		// Check the progress
		done := make(chan interface{})
		go func(x C, p <-chan int64) {

			contentLength := <-p // first item is the contentLength
			var count int64
			for {
				select {
				case <-done:
					//x.Printf("\nSo %d ShouldEqual %d\n", count, contentLength)
					x.So(count, ShouldEqual, contentLength)
					return
				case b := <-p:
					count += b
				}
			}

		}(c, progressChan)

		resp, rerr := rt.RoundTrip(req) // Run the request
		close(done)                     // Close the done chan

		So(rerr, ShouldBeNil)
		rBytes, raerr := io.ReadAll(resp.Body)
		So(raerr, ShouldBeNil)

		defer resp.Body.Close()

		So(rBytes, ShouldResemble, serverBytes)

	})

}

func Test_RangeDownloadChunkSize(t *testing.T) {
	defer leaktest.Check(t)()

	Convey("When a server is started that supports ranges, and chunkSize is set, RangeTripper downloads the content correctly", t, func(c C) {
		serverBytes := []byte(`OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee`)

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			sbuff := bytes.NewReader(serverBytes)
			http.ServeContent(rw, req, "thefile", time.Now(), sbuff)
		}))
		// Close the server when test finishes
		defer server.Close()

		for chunkSize := int64(1); chunkSize < 10; chunkSize++ {
			tfile, err := os.CreateTemp("/tmp", "rtchunk")
			if err != nil {
				panic(err)
			}
			name := tfile.Name()
			tfile.Close()
			defer os.Remove(tfile.Name())

			rt, err := New(10)
			//rt, err := NewWithLoggers(10, name, log.New(io.Discard, "", 0), log.New(os.Stderr, "[DEBUG] ", 0))
			So(err, ShouldBeNil)
			rt.SetChunkSize(chunkSize)

			req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, name), "GET", server.URL, nil)

			_, rerr := rt.RoundTrip(req) // Run the request
			So(rerr, ShouldBeNil)

			fileContents, ferr := os.ReadFile(tfile.Name())
			So(ferr, ShouldBeNil)
			So(string(fileContents), ShouldEqual, string(serverBytes))
			So(rt.workers, ShouldEqual, int(int64(len(serverBytes))/chunkSize))
		}
	})

}

func Test_HEAD403(t *testing.T) {
	defer leaktest.Check(t)()

	Convey("When a server returns a 403 for HEAD and GET, it is handled correctly", t, func() {
		tfile, err := os.CreateTemp("/tmp", "sdhc")
		if err != nil {
			panic(err)
		}
		defer os.Remove(tfile.Name())
		defer tfile.Close()

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte(`FORBIDDEN`)) // Simple write
		}))
		// Close the server when test finishes
		defer server.Close()

		rt, err := New(10)
		rt.SetClient(new(http.Client)) // use a normal http.Client
		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldNotBeNil)
	})

	Convey("When a server returns a 403 for HEAD and a 206 for GET, it is handled correctly", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee`)

		tfile, err := os.CreateTemp("/tmp", "sdhc")
		if err != nil {
			panic(err)
		}
		defer os.Remove(tfile.Name())

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodHead {
				rw.WriteHeader(http.StatusForbidden)
				rw.Write([]byte(`FORBIDDEN`)) // Simple write
				return
			}
			// GET, etc
			sbuff := bytes.NewReader(serverBytes)
			http.ServeContent(rw, req, "thefile", time.Now(), sbuff)

		}))
		// Close the server when test finishes
		defer server.Close()

		//rt, err := NewWithLoggers(10, tfile.Name(), log.New(io.Discard, "", 0), log.New(os.Stderr, "[DEBUG] ", 0))
		rt, err := New(10)
		rt.SetClient(new(http.Client)) // use a normal http.Client
		rt.SetChunkSize(10)

		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		tfile.Close()

		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))
	})

	Convey("When a server returns a 403 for HEAD and a 200 for GET, it is handled correctly", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee OK I have something to say here weeeeee`)

		tfile, err := os.CreateTemp("/tmp", "sdhc")
		if err != nil {
			panic(err)
		}
		defer os.Remove(tfile.Name())

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodHead {
				rw.WriteHeader(http.StatusForbidden)
				rw.Write([]byte(`FORBIDDEN`)) // Simple write
				return
			}
			// GET, etc
			rw.Write(serverBytes)

		}))
		// Close the server when test finishes
		defer server.Close()

		//rt, err := NewWithLoggers(10, tfile.Name(), log.New(io.Discard, "", 0), log.New(os.Stderr, "[DEBUG] ", 0))
		rt, err := New(10)
		rt.SetClient(new(http.Client)) // use a normal http.Client
		rt.SetChunkSize(10)

		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		tfile.Close()

		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))
	})

}

func Test_RetryClient(t *testing.T) {
	defer leaktest.Check(t)()

	Convey("When a request works, RetryClient doesn't retry :)", t, func() {

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.Write([]byte("Woooo"))
		}))
		// Close the server when test finishes
		defer server.Close()

		rt := NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond) // custom RetryClient with short times
		req, _ := http.NewRequest("GET", server.URL, nil)

		start := time.Now()
		res, rerr := rt.Do(req)
		stop := time.Now()
		So(rerr, ShouldBeNil)
		So(res.StatusCode, ShouldEqual, http.StatusOK)
		So(stop, ShouldHappenWithin, 2*time.Millisecond, start)

	})

	Convey("When a request times out, RetryClient retries happen, and then errors out", t, func() {

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			time.Sleep(1 * time.Second)
		}))
		// Close the server when test finishes
		defer server.Close()

		rt := NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond) // custom RetryClient with short times
		req, _ := http.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, rerr := rt.Do(req)
		stop := time.Now()
		So(rerr.Error(), ShouldContainSubstring, "context deadline exceeded")
		So(stop, ShouldHappenWithin, ((3*2+1+1)*10)*time.Millisecond, start)

	})

	Convey("When a request returns a 403, RetryClient errors out immediately", t, func() {

		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusForbidden)
		}))
		// Close the server when test finishes
		defer server.Close()

		rt := NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond) // custom RetryClient with short times
		req, _ := http.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, rerr := rt.Do(req)
		stop := time.Now()
		So(rerr, ShouldNotBeNil)
		So(rerr, ShouldEqual, errStatusNope)
		So(stop, ShouldHappenWithin, 4*time.Millisecond, start)

	})

}

func Test_RetryClientExp(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "sdbe")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a request times out, RetryClient retries happen exponentially, and then errors out", t, func() {
		// Start a local HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			time.Sleep(1 * time.Second)
		}))
		// Close the server when test finishes
		defer server.Close()

		rt := NewRetryClientWithExponentialBackoff(3, 10*time.Millisecond, 10*time.Millisecond) // custom RetryClient with short times
		req := httptest.NewRequest("GET", server.URL, nil)

		start := time.Now()
		_, rerr := rt.Do(req)
		stop := time.Now()
		So(rerr, ShouldNotBeNil)
		So(stop, ShouldHappenWithin, time.Duration(int64(math.Pow(10, 3)))*time.Millisecond, start)

	})

}

func Test_StandardDownload500s(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "sdfs")
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

		rt, err := New(10)
		rt.SetClient(NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond)) // custom RetryClient with short times
		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldNotBeNil)

	})

}

func Test_HEADErrorButGETRange(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "sdfs")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tfile.Name())

	Convey("When a server is started that supports ranges but throws a low error on HEAD, retries happen, and it all works", t, func() {
		serverBytes := []byte(`OK I have something to say here weeeeee!!!!`)

		var server *httptest.Server

		// Start a local HTTP server
		server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodHead {
				// we close the connection on HEAD
				server.CloseClientConnections()
				return
			}
			// GET, etc
			sbuff := bytes.NewReader(serverBytes)
			http.ServeContent(rw, req, "thefile", time.Now(), sbuff)
		}))
		// Close the server when test finishes
		defer server.Close()

		rt, err := New(10)
		rt.SetClient(NewRetryClient(3, 10*time.Millisecond, 10*time.Millisecond)) // custom RetryClient with short times
		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		tfile.Close()

		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))

	})

}

func Test_StandardDownloadSecondRequestFails(t *testing.T) {
	defer leaktest.Check(t)()

	tfile, err := os.CreateTemp("/tmp", "sd")
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

		rt, err := New(10)
		So(err, ShouldBeNil)

		req := httptest.NewRequestWithContext(context.WithValue(context.Background(), outfileKey, tfile.Name()), "GET", server.URL, nil)

		_, rerr := rt.RoundTrip(req)
		So(rerr, ShouldBeNil)
		fileContents, ferr := os.ReadFile(tfile.Name())
		So(ferr, ShouldBeNil)
		So(string(fileContents), ShouldEqual, string(serverBytes))
	})
}
