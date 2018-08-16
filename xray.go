// Copyright 2018, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/aws/aws-sdk-go/service/xray/xrayiface"
	"go.opencensus.io/trace"
)

// OnExport structure passed when a root segment is published
type OnExport struct {
	// TraceID holds the raw aws traceID e.g. 1-581cf771-a006649127e371903a2de979
	TraceID string
}

const (
	// defaultInterval - segments will be published at this frequency
	defaultInterval = time.Second

	// maxBufferSize - maximum number of segments that can be published in a single call
	maxBufferSize = 50
)

type config struct {
	region     string                // aws region; for sending to another region's xray
	output     io.Writer             // output error messages
	api        xrayiface.XRayAPI     // use specific api instance
	onExport   func(export OnExport) // callback on publish
	origin     origin                // origin of span
	service    *service              // contains embedded version info
	interval   time.Duration         // interval spans are published to aws
	bufferSize int                   // bufSize represents max number of spans before forcing publish
	blacklist  []*regexp.Regexp
}

type Option interface {
	apply(c *config)
}

type optionFunc func(c *config)

func (fn optionFunc) apply(c *config) {
	fn(c)
}

// WithRegion - optional aws region to send xray messages to
func WithRegion(region string) Option {
	return optionFunc(func(c *config) {
		c.region = region
	})
}

// WithOutput - optional writer for error messages
func WithOutput(w io.Writer) Option {
	return optionFunc(func(c *config) {
		c.output = w
	})
}

// WithAPI - optional manually constructed api instance
func WithAPI(api xrayiface.XRayAPI) Option {
	return optionFunc(func(c *config) {
		c.api = api
	})
}

// WithOnExport - function to be called when spans are published to AWS; useful
// if you would like the traceID used for AWS
func WithOnExport(fn func(OnExport)) Option {
	return optionFunc(func(c *config) {
		c.onExport = fn
	})
}

// WithOrigin - specifies the aws origin of the span; currently supported are
// OriginEC2, OriginECS, and OriginEB
func WithOrigin(origin origin) Option {
	return optionFunc(func(c *config) {
		c.origin = origin
	})
}

// WithVersion - specifies the version of the application running
func WithVersion(version string) Option {
	return optionFunc(func(c *config) {
		c.service = &service{
			Version: version,
		}
	})
}

// WithInterval - specifies longest time before buffered spans are published; defaults to 1s
func WithInterval(interval time.Duration) Option {
	return optionFunc(func(c *config) {
		c.interval = interval
	})
}

// WithBufferSize - specifies the maximum number of spans to buffer before publishing them; defaults to 100
func WithBufferSize(bufferSize int) Option {
	return optionFunc(func(c *config) {
		c.bufferSize = bufferSize
	})
}

// WithBlacklist filters out the spans those names match one of the given regexps.
func WithBlacklist(blacklist []*regexp.Regexp) Option {
	return optionFunc(func(c *config) {
		c.blacklist = blacklist
	})
}

// Exporter is an implementation of trace.Exporter that uploads spans to AWS XRay
type Exporter struct {
	api      xrayiface.XRayAPI
	onExport func(export OnExport)
	logger   *log.Logger
	service  *service
	origin   string
	wg       sync.WaitGroup // wg holds number of publishers in flight

	ctx    context.Context    // ctx cancels the child goroutine
	cancel context.CancelFunc // cancels the child goroutine; idempotent
	done   chan struct{}      // done returns immediately once the child goroutine has finished

	mutex     sync.Mutex        // mutex protects offset, buffer, and closed
	offset    int               // offset holds position of next SpanData within queue
	buffer    []*trace.SpanData // buffer of spans not yet published
	closed    bool              // indicates Exporter is closed.  any additional spans will be dropped
	blacklist []*regexp.Regexp
}

var (
	// extract region from aws availability zone
	reRegion = regexp.MustCompile(`([^\-]+-[^\-]+-\d+)`)

	// client exposed to simplify testing
	clientDo = http.DefaultClient.Do
)

// lookupRegionFromMetaData attempts to determine region from aws metadata endpoint
func lookupRegionFromMetaData() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/placement/availability-zone", nil)
	req = req.WithContext(ctx)

	resp, err := clientDo(req)
	if err != nil {
		return "", fmt.Errorf("unable lookup region via http://169.254.169.254/latest/meta-data/placement/availability-zone - %v", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read contents from http://169.254.169.254/latest/meta-data/placement/availability-zone - %v", err)
	}

	segments := reRegion.FindSubmatch(data)
	if len(segments) == 0 {
		return "", fmt.Errorf("unable to determine aws region from availability zone, %v", string(data))
	}

	return string(segments[1]), nil
}

func lookupRegion() (string, error) {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		v, err := lookupRegionFromMetaData()
		if err != nil {
			return "", err
		}
		region = v
	}

	return region, nil
}

// makeApi constructs an instance of the aws xray api
func makeApi(region string) (*xray.XRay, error) {
	if region == "" {
		v, err := lookupRegion()
		if err != nil {
			return nil, err
		}
		region = v
	}

	// Region value will instruct the SDK where to make service API requests to. If is
	// not provided in the environment the region must be provided before a service
	// client request is made.
	//
	// https://docs.aws.amazon.com/sdk-for-go/api/aws/session/
	config := aws.Config{Region: aws.String(region)}
	s, err := session.NewSession(&config)
	if err != nil {
		return nil, err
	}
	return xray.New(s), nil
}

// buildConfig from options
func buildConfig(opts ...Option) (config, error) {
	var c config
	for _, opt := range opts {
		opt.apply(&c)
	}

	if c.output == nil {
		c.output = os.Stderr
	}
	if c.onExport == nil {
		c.onExport = func(export OnExport) {}
	}
	if c.interval <= 0 {
		c.interval = defaultInterval
	}
	if c.bufferSize <= 0 || c.bufferSize > maxBufferSize {
		c.bufferSize = maxBufferSize
	}
	if c.api == nil {
		api, err := makeApi(c.region)
		if err != nil {
			return config{}, err
		}
		c.api = api
	}

	return c, nil
}

// NewExporter returns an implementation of trace.Exporter that uploads spans
// to AWS X-Ray
func NewExporter(opts ...Option) (*Exporter, error) {
	c, err := buildConfig(opts...)
	if err != nil {
		return nil, err
	}

	var (
		logger = log.New(c.output, "XRAY ", log.LstdFlags)
		buffer = make([]*trace.SpanData, c.bufferSize)
		done   = make(chan struct{})
	)

	ctx, cancel := context.WithCancel(context.Background())

	exporter := &Exporter{
		api:      c.api,
		onExport: c.onExport,
		service:  c.service,
		logger:   logger,

		ctx:    ctx,
		cancel: cancel,
		done:   done,

		buffer:    buffer,
		origin:    string(c.origin),
		blacklist: c.blacklist,
	}
	go exporter.publishAtInterval(c.interval)

	return exporter, nil
}

func (e *Exporter) isBlacklisted(span *trace.SpanData) bool {
	for _, blacklistEntry := range e.blacklist {
		if blacklistEntry.MatchString(span.Name) {
			return true
		}
	}
	return false
}

func (e *Exporter) makeInput(spans []*trace.SpanData) (xray.PutTraceSegmentsInput, []string) {
	var (
		traceIDs []string
		docs     []*string
		w        = borrow()
	)
	defer release(w)

	for _, span := range spans {
		if e.isBlacklisted(span) {
			continue
		}
		var segment = e.makeSegment(span)
		if segment.ParentID == "" {
			traceIDs = append(traceIDs, segment.TraceID)
		}

		w.Reset()
		if err := w.Encode(segment); err != nil {
			e.logger.Printf("unable to encode segment, %v", err)
			continue
		}

		docs = append(docs, aws.String(w.String()))
	}

	input := xray.PutTraceSegmentsInput{
		TraceSegmentDocuments: docs,
	}
	return input, traceIDs
}

func (e *Exporter) publish(spans []*trace.SpanData) {
	defer e.wg.Done()

	var (
		input, traceIDs = e.makeInput(spans)
	)
	if len(input.TraceSegmentDocuments) == 0 {
		return
	}

	for attempt := 0; attempt < 3; attempt++ {
		_, err := e.api.PutTraceSegments(&input)
		if err == nil {
			for _, traceID := range traceIDs {
				go e.onExport(OnExport{TraceID: traceID})
			}
			return
		}

		e.logger.Printf("attempt %v failed to publish spans, %v\n", attempt, err)
		time.Sleep(500 * time.Millisecond)
	}

	e.logger.Println("all attempts to publish span failed.  giving up.")
}

func (e *Exporter) flush() {
	if e.offset == 0 {
		return
	}

	var spans []*trace.SpanData
	spans = append(spans, e.buffer[0:e.offset]...)

	// reset buffer
	e.offset = 0

	e.wg.Add(1)
	go e.publish(spans)
}

func (e *Exporter) publishAtInterval(interval time.Duration) {
	defer close(e.done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-ticker.C:
			e.mutex.Lock()
			e.flush()
			e.mutex.Unlock()
		}
	}
}

// Close this exporter and publish any spans that may have been buffered
func (e *Exporter) Close() error {
	e.cancel()
	<-e.done // wait for goroutine to shut down

	e.mutex.Lock()
	if !e.closed {
		e.closed = true
		e.flush()
		e.wg.Wait()
	}
	e.mutex.Unlock()

	return nil
}

func (e *Exporter) Flush() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.flush()
}

// ExportSpan exports a span to AWS X-Ray
func (e *Exporter) ExportSpan(s *trace.SpanData) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.closed {
		e.logger.Println("ExportSpan called on a closed Exporter.  SpanData will be dropped.")
		return
	}

	e.buffer[e.offset] = s
	e.offset++

	if e.offset == cap(e.buffer) {
		e.flush()
	}
}

func (e *Exporter) makeSegment(span *trace.SpanData) segment {
	var (
		s = rawSegment(span)
	)

	if isRootSpan := span.ParentSpanID == zeroSpanID; isRootSpan {
		s.Origin = e.origin
		s.Service = e.service

	} else {
		s.Type = "subsegment"
	}

	return s
}
