// Package rangetripper provides a performant http.RoundTripper that handles byte-range downloads if
// the resulting HTTP server claims to support them in a HEAD request for the file. RangeTripper will
// download 1/Nth of the file asynchronously with each of the “fileChunks“ specified in a New.
// N+1 actual downloaders are most likely as the +1 covers any gap from non-even division of content-length.
package rangetripper

import (
	"github.com/cognusion/go-sequence"
	"github.com/cognusion/go-timings"
	"github.com/cognusion/semaphore"
	"go.uber.org/atomic"

	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Static errors to return
const (
	ContentLengthNumericError   = rtError("Content-Length value cannot be converted to a number")
	ContentLengthMismatchError  = rtError("downloaded file size does not match content-length")
	SingleRequestExhaustedError = rtError("one request has already been made with this RangeTripper")

	headFakeFailedError = rtError("headfake failed, return previous error")
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

	client     Client
	workers    int
	toFile     string
	outFile    *os.File
	wg         sync.WaitGroup
	sem        semaphore.Semaphore
	progress   chan int64
	used       atomic.Bool
	fetchError atomic.Error
	chunkSize  int64
}

// New simply returns a RangeTripper or an error. Logged messages are discarded.
func New(fileChunks int, outputFilePath string) (*RangeTripper, error) {
	return NewWithLoggers(fileChunks, outputFilePath, nil, nil)
}

// NewWithLoggers returns a RangeTripper or an error. Logged messages are sent to the specified Logger, or discarded if nil.
func NewWithLoggers(fileChunks int, outputFilePath string, timingLogger, debugLogger *log.Logger) (*RangeTripper, error) {
	// Validate file to write to, early
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return nil, err
	}

	// sanity
	if fileChunks < 1 {
		fileChunks = 1
	}

	// Discard if nil
	if timingLogger == nil {
		timingLogger = log.New(io.Discard, "", 0)
	}

	// Discard if nil
	if debugLogger == nil {
		debugLogger = log.New(io.Discard, "", 0)
	}

	return &RangeTripper{
		TimingsOut: timingLogger,
		DebugOut:   debugLogger,
		workers:    fileChunks,
		toFile:     outputFilePath,
		outFile:    outFile,
		client:     DefaultClient,
		sem:        semaphore.NewSemaphore(fileChunks + 1),
	}, nil
}

// SetClient allows for overriding the Client used to make the requests.
func (rt *RangeTripper) SetClient(client Client) {
	rt.client = client
}

// SetMax allows for setting the maximum number of concurrently-running workers
func (rt *RangeTripper) SetMax(max int) {
	if max == 0 {
		return
	} else if max > rt.workers {
		max = rt.workers + 1
	}

	rt.sem = semaphore.NewSemaphore(max)
}

// SetChunkSize overrides the “fileChunks“ and instead will divide the resulting Content-Length by this to
// determine the appropriate chunk count dynamically. “fileChunks“ will still be used to guide the maximum
// number of concurrent workers, unless “SetMax()“ is used.
func (rt *RangeTripper) SetChunkSize(chunkBytes int64) {
	if chunkBytes < 1 {
		chunkBytes = 1
	}

	rt.chunkSize = chunkBytes
}

// WithProgress returns a read-only chan that will first provide the total length of the content (in bytes),
// followed by a stream of completed byte-lengths. CAUTION: It is a generally bad idea to call this and then
// ignore the resulting channel.
func (rt *RangeTripper) WithProgress() <-chan int64 {
	if rt.progress == nil {
		rt.progress = make(chan int64, 100)
	}
	return rt.progress
}

// RoundTrip is called with a formed Request, writing the Body of the Response to
// to the specified output file. The Response should be ignored, but
// errors are important. Both the Request.Body and the RangeTripper.outFile will be
// closed when this function returns.
func (rt *RangeTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// We only allow one execution total, which is gated by the rt.used flag.
	if rt.used.Swap(true) {
		// Swap has atomically set the value to true, but returned the previous
		// value of false.
		return nil, SingleRequestExhaustedError
	}

	defer rt.outFile.Close()
	if r.Body != nil {
		defer r.Body.Close()
	}

	var (
		hres          *http.Response
		err           error
		contentLength int
		dlid          = seq.NextHashID()
	)

	defer timings.Track(fmt.Sprintf("[%s] RangeTripper Full", dlid), time.Now(), rt.TimingsOut)

	// Error on head: Bail?
	if hres, err = rt.head(r.URL.String()); err != nil {
		// Some systems toss odd errors on HEAD requests. Noted against a PHP downloader that takes parameters.
		hresn, errn := rt.tryHeadFake(r.URL.String())
		if errn != nil {
			// headfake didn't work out, return original error
			return nil, err
		} else if hresn.StatusCode == http.StatusOK {
			// 200 means it didn't accept the range, and gave us the whole file, so we are done.
			return hresn, nil
		}
		// POST: headfake worked, and we can GET using ranges
		// silently replace the body
		hres = hresn
	}
	hres.Body.Close()

	if hres.StatusCode == http.StatusForbidden {
		// Forbidden might just be for the HEAD
		hfres, hferr := rt.tryHeadFake(r.URL.String())
		if hferr == headFakeFailedError {
			// we resort to returning the original HEAD403
			return nil, fmt.Errorf("error during HEAD: %d / %s", hres.StatusCode, hres.Status)
		} else if hferr != nil {
			// we resort to returning the original HEAD403 but send the returned error to debug
			rt.DebugOut.Printf("Error during tryHeadFake: %v\n", hferr)
			return nil, fmt.Errorf("error during HEAD: %d / %s", hres.StatusCode, hres.Status)
		} else if hfres.StatusCode == http.StatusOK {
			// 200 means it didn't accept the range, and gave us the whole file
			return hfres, nil
		}
		// POST: headfake worked, and we can GET using ranges
		// silently replace the body
		hres = hfres
	} else if !(hres.StatusCode == http.StatusOK || hres.StatusCode == http.StatusPartialContent) {
		return nil, fmt.Errorf("error during HEAD: %d / %s", hres.StatusCode, hres.Status)
	}
	// POST: Either HEAD or GET RANGE succeeded in determining support for range downloads. Proceed!

	if cl := hres.Header.Get("Content-Length"); cl == "" {
		// No Content-Length? Just grab it like normal :(
		if err = rt.fetch(r.URL.String()); err != nil {
			return nil, err
		}
		return hres, nil
	} else if contentLength, err = strconv.Atoi(cl); err != nil {
		// Non-numeric content-length? Bail.
		return nil, fmt.Errorf("[%s] value of Content-Length header appears non-numeric: '%s': %w", dlid, cl, ContentLengthNumericError)
	}

	// Byte ranges accepted? Let's do this
	if v := hres.Header.Get("Accept-Ranges"); v == "bytes" {
		var (
			start     int
			end       int
			chunkSize = int(contentLength / rt.workers)
		)
		if rt.chunkSize != 0 {
			chunkSize = int(rt.chunkSize)
			rt.workers = int(contentLength / chunkSize)
		}

		if rt.progress != nil {
			rt.progress <- int64(contentLength)
		}

		rt.DebugOut.Printf("[%s] Ranges supported! Content Length: %d, Downloaders: %d, Chunk Size %d\n", dlid, contentLength, rt.workers, chunkSize)

		for i := 0; i < rt.workers; i++ {
			rt.sem.Lock()
			if ferr := rt.fetchError.Load(); ferr != nil {
				// We've had an error, bail
				rt.DebugOut.Printf("\t[%s] Error %v encountered while spawning workers, aborting at %d\n", dlid, ferr, start)
				return nil, ferr
			}

			rt.wg.Add(1)
			end = start + int(chunkSize)
			rt.DebugOut.Printf("\t[%s] Worker from %d to %d\n", dlid, start, end)
			go rt.fetchChunk(int64(start), int64(end), r.URL.String())
			start = end
		}
		if end < contentLength {
			// gap
			rt.sem.Lock()
			rt.wg.Add(1)
			start = end
			end = contentLength
			rt.DebugOut.Printf("\t[%s] Gap worker from %d to %d\n", dlid, start, end)
			go rt.fetchChunk(int64(start), int64(end), r.URL.String())
		}
		rt.wg.Wait() // wrap in a timer?

		if ferr := rt.fetchError.Load(); ferr != nil {
			// We've had an error, bail
			rt.DebugOut.Printf("[%s] Error %v encountered after all workers spawned, aborting\n", dlid, ferr)
			return nil, ferr
		}

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
		return hres, nil
	}
	// else Byte ranges not accepted :(
	rt.DebugOut.Printf("[%s] Range Download unsupported\nBeginning full download...\n", dlid)

	rt.fetch(r.URL.String())

	rt.DebugOut.Printf("[%s] Download Complete\n", dlid)
	return hres, nil
}

// Do is a satisfier of the rangetripper.Client interface, and is identical to RoundTrip
func (rt *RangeTripper) Do(r *http.Request) (*http.Response, error) {
	return rt.RoundTrip(r)
}

// head returns the Response or error from a HEAD request for the specified URL
func (rt *RangeTripper) head(url string) (*http.Response, error) {
	var (
		req *http.Request
		res *http.Response
		err error
	)

	defer timings.Track("head", time.Now(), rt.TimingsOut)

	// Create a simple HEAD request
	if req, err = http.NewRequest("HEAD", url, nil); err != nil {
		return nil, err
	}

	if res, err = http.DefaultClient.Do(req); err != nil {
		return nil, err
	}
	return res, nil
}

// headFake returns the Response or error from a GET request with a small RANGE
func (rt *RangeTripper) headFake(url string) (*http.Response, error) {
	var (
		req   *http.Request
		res   *http.Response
		err   error
		start int64 = 0
		end   int64 = 10
	)

	defer timings.Track("headFake", time.Now(), rt.TimingsOut)

	// Create a simple GET request
	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return nil, err
	}

	// Add the Range header with our details
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	if res, err = http.DefaultClient.Do(req); err != nil {
		return nil, err
	}

	rt.DebugOut.Printf("HEADFAKE %d-%d returned %d, %s %s\n", start, end, res.StatusCode, res.Header.Get("Content-Range"), res.Header.Get("Content-Length"))

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

	if rt.progress != nil {
		defer func() { rt.progress <- end - start }()
	}

	defer rt.sem.Unlock()
	defer rt.wg.Done()
	defer timings.Track(fmt.Sprintf("\tfetchChunk %d - %d", start, end), time.Now(), rt.TimingsOut)

	// SHOULD BE LAST of the compulsory defers, so is the first to exec before there are unlocks, etc.
	// If an error occurs, stuff the value. We know that there will be overwrites, and that is ok
	defer func() {
		if err != nil {
			rt.fetchError.Store(err)
		}
	}()

	// Create a simple GET request
	if req, err = http.NewRequest("GET", url, nil); err != nil {
		return err
	}

	// Add the Range header with our details
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	if res, err = rt.client.Do(req); err != nil {
		return err
	}
	defer res.Body.Close()

	//rt.DebugOut.Printf("Range %d-%d returned %d, %s %s\n", start, end, res.StatusCode, res.Header.Get("Content-Range"), res.Header.Get("Content-Length"))

	// Read the chunk into a buffer, and then write it to the outfile at the appropriate offset
	var ra []byte
	if ra, err = io.ReadAll(res.Body); err != nil {
		rt.DebugOut.Printf("Error during ReadAll byte %d: %s\n", start, err)
		return err
	} else if _, err = rt.outFile.WriteAt(ra, start); err != nil {
		rt.DebugOut.Printf("Error during writing byte %d: %s\n", start, err)
		return err
	}

	rt.DebugOut.Printf("Finished Downloading %d-%d: %s\n", start, end, url)
	return nil
}

// tryHeadFake is an abstraction of logic used previously IFF a HEAD returned 403, so
// it can now be used elsewhere. If the error is `headFakeFailedError`, that means
// there was no error, per se, but neither were the results compelling, so you should
// return any previous error you got from the HEAD.
func (rt *RangeTripper) tryHeadFake(url string) (*http.Response, error) {
	// headFake returns the Response or error from a GET request with a small RANGE
	// IFF the Response is a 206 with Content-Length and Content-Range, used in cases
	// where a HEAD may 403 (e.g. AWS S3) but a GET works fine
	if hfres, hferr := rt.headFake(url); hferr != nil {
		return nil, hferr
	} else if hfres.StatusCode == http.StatusOK {
		// 200 means it didn't accept the range, and gave us the whole file
		defer hfres.Body.Close()
		if _, err := io.Copy(rt.outFile, hfres.Body); err != nil {
			return nil, fmt.Errorf("error during write (hf): %w", err)
		}
		// We done, albeit without ranges
		return hfres, nil
	} else if hfres.StatusCode == http.StatusPartialContent {
		// We routed around the HEAD issue.

		// Grab the size listed at the end of the Content-Range header,
		// and force it into the Content-Length header
		parts := strings.Split(hfres.Header.Get("Content-Range"), "/") // bytes 0-10/159
		rt.DebugOut.Printf("%+v\n", parts)
		if len(parts) == 2 {
			hfres.Header.Set("Content-Length", parts[1])
		}
		if v := hfres.Header.Get("Accept-Ranges"); v != "bytes" {
			hfres.Header.Set("Accept-Ranges", "bytes")
		}
		// Silently replacing the old Response with this one after mangling the CL header
		return hfres, nil
	} else {
		// we should resort to returning the original error
		return nil, headFakeFailedError
	}

}
