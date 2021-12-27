// Package rangetripper provides a performant http.RoundTripper that handles byte-range downloads if
// the resulting HTTP server claims to support them in a HEAD request for the file. RangeTripper will
// download 1/Nth of the file asynchronously with each of the ``parallelDownloads`` specified in a New.
// N+1 actual downloaders are most likely as the +1 covers any gap from non-even division of content-length.
package rangetripper

import (
	"github.com/cognusion/go-sequence"
	"github.com/cognusion/go-timings"

	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Static errors to return
const (
	ContentLengthNumericError  = rtError("Content-Length value cannot be converted to a number")
	ContentLengthMismatchError = rtError("Downloaded file size does not match content-length")
)

var (
	seq = sequence.New(0)
)

// RTError is an error type
type rtError string

// Error returns the stringified version of RTError
func (e rtError) Error() string {
	return string(e)
}

// RangeTripper is an http.RoundTripper to be used in an http.Client.
// This should not be used in its default state, instead by its New functions.
// A single RangeTripper *must* only be used for one request.
type RangeTripper struct {
	TimingsOut *log.Logger
	DebugOut   *log.Logger

	client  Client
	workers int
	toFile  string
	outFile *os.File
	wg      sync.WaitGroup
}

// New simply returns a RangeTripper or an error. Logged messages are discarded.
func New(parallelDownloads int, outputFilePath string) (*RangeTripper, error) {
	return NewWithLoggers(parallelDownloads, outputFilePath, nil, nil)
}

// NewWithLoggers returns a RangeTripper or an error. Logged messages are sent to the specified Logger, or discarded if nil.
func NewWithLoggers(parallelDownloads int, outputFilePath string, timingLogger, debugLogger *log.Logger) (*RangeTripper, error) {
	// Validate file to write to, early
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return nil, err
	}

	// sanity
	if parallelDownloads < 1 {
		parallelDownloads = 1
	}

	// Discard if nil
	if timingLogger == nil {
		timingLogger = log.New(ioutil.Discard, "", 0)
	}

	// Discard if nil
	if debugLogger == nil {
		debugLogger = log.New(ioutil.Discard, "", 0)
	}

	return &RangeTripper{
		TimingsOut: timingLogger,
		DebugOut:   debugLogger,
		workers:    parallelDownloads,
		toFile:     outputFilePath,
		outFile:    outFile,
		client:     DefaultClient,
	}, nil
}

// SetClient allows for overriding the Client used to make the requests.
func (rt *RangeTripper) SetClient(client Client) {
	rt.client = client
}

// RoundTrip is called with a formed Request, writing the Body of the response to
// to the specified output file. The response should largely be ignored, but
// errors are important.
func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error) {

	defer rt.outFile.Close()
	defer r.Body.Close()

	var (
		res           *http.Response
		err           error
		contentLength int
		dlid          = seq.NextHashID()
	)

	defer timings.Track(fmt.Sprintf("[%s] RangeTripper Full", dlid), time.Now(), rt.TimingsOut)

	// Error on head? Bail.
	if res, err = rt.head(r.URL.String()); err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// No Content-Length? Just grab it like normal :(
	if len(res.Header["Content-Length"]) < 1 {
		if err = rt.fetch(r.URL.String()); err != nil {
			return nil, err
		}
		return res, nil
	}

	// Non-numeric content-length? Bail.
	if contentLength, err = strconv.Atoi(res.Header["Content-Length"][0]); err != nil {
		return nil, fmt.Errorf("[%s] value of Content-Length header appears non-numeric: '%s': %w", dlid, res.Header["Content-Length"][0], ContentLengthNumericError)
	}

	// Byte ranges accepted? Let's do this
	if v, ok := res.Header["Accept-Ranges"]; ok && v[0] == "bytes" {
		var (
			start     int
			end       int
			chunkSize = int(contentLength / rt.workers)
		)

		rt.DebugOut.Printf("[%s] Ranges supported! Content Length: %d, Downloaders: %d, Chunk Size %d\n", dlid, contentLength, rt.workers, chunkSize)

		for i := 0; i < rt.workers; i++ {
			rt.wg.Add(1)
			end = start + int(chunkSize)
			rt.DebugOut.Printf("\t[%s] Worker from %d to %d\n", dlid, start, end)
			go rt.fetchChunk(int64(start), int64(end), r.URL.String())
			start = end
		}
		if end < contentLength {
			// gap
			rt.wg.Add(1)
			start = end
			end = contentLength
			rt.DebugOut.Printf("\t[%s] Gap worker from %d to %d\n", dlid, start, end)
			go rt.fetchChunk(int64(start), int64(end), r.URL.String())
		}
		rt.wg.Wait() // wrap in a timer?

		rt.DebugOut.Printf("[%s] complete\n", dlid)
		defer timings.Track(fmt.Sprintf("[%s] RangeTripper Assembled", dlid), time.Now(), rt.TimingsOut)
		//Verify file size
		fileStats, err := rt.outFile.Stat()
		if err != nil {
			return nil, err
		}
		if fileSize := fileStats.Size(); fileSize != int64(contentLength) {
			return nil, fmt.Errorf("[%s] actual Size: %d expected Size: %d : %w", dlid, fileSize, contentLength, ContentLengthMismatchError)
		}
		return res, nil
	}
	// else Byte ranges no accepted :(
	rt.DebugOut.Printf("[%s] Range Download unsupported\nBeginning full download...\n", dlid)

	rt.fetch(r.URL.String())

	rt.DebugOut.Printf("[%s] Download Complete\n", dlid)
	return res, nil
}

// Do is a satisfier of the rangetripper.Client interface, and is identical to RoundTrip
func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error) {
	return rt.RoundTrip(r)
}

// head returns the Response or error from a HEAD request for the specified URL
func (rt *RangeTripper) head(url string) (*http.Response, error) {
	var (
		res *http.Response
		req *http.Request
		err error
	)

	if req, err = http.NewRequest("HEAD", url, nil); err != nil {
		return nil, err
	}

	if res, err = rt.client.Do(req); err != nil {
		return nil, err
	}
	return res, nil
}

// fetch is a full-response fetch-and-write func.
// It consumes the response entirely
func (rt *RangeTripper) fetch(url string) error {
	var (
		req *http.Request
		res *http.Response
		err error
	)

	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return err
	}

	if res, err = rt.client.Do(req); err != nil {
		return err
	}
	defer res.Body.Close()

	if _, err = io.Copy(rt.outFile, res.Body); err != nil {
		return fmt.Errorf("error during write: %w", err)
	}

	rt.DebugOut.Printf("Finished Downloading %s\n", url)
	return err
}

// fetchChunk is a range fetch-and-write func.
// It consumes the response entirely, and assumes a WaitGroup has been Added
// to before it is called.
func (rt *RangeTripper) fetchChunk(start, end int64, url string) error {
	var (
		req *http.Request
		res *http.Response
		err error
	)

	defer rt.wg.Done()

	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	if res, err = rt.client.Do(req); err != nil {
		return err
	}
	defer res.Body.Close()

	// Read the chunk into a buffer, and then write it to the outfile at the appropriate offset
	if ra, err := ReadAll(res.Body); err != nil {
		rt.DebugOut.Printf("Error during ReadAll byte %d: %s\n", start, err)
		return err
	} else if _, err := rt.outFile.WriteAt(ra, start); err != nil {
		rt.DebugOut.Printf("Error during writing byte %d: %s\n", start, err)
		return err
	}

	rt.DebugOut.Printf("Finished Downloading %d-%d : %s\n", start, end, url)
	return nil
}
