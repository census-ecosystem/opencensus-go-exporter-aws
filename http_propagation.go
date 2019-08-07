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
	"net/http"
	"strings"

	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
)

const (
	httpHeaderMaxSize = 200
	httpHeader        = `X-Amzn-Trace-Id`
	prefixRoot        = "Root="
	prefixParent      = "Parent="
	prefixSampled     = "Sampled="
	separator         = ";" // separator used by x-ray to split parts of X-Amzn-Trace-Id header
)

// HTTPFormat implements propagation.HTTPFormat to propagate
// traces in HTTP headers for for Amazon services: ELB, ALB, Lambda, etc.
type HTTPFormat struct{}

var _ propagation.HTTPFormat = (*HTTPFormat)(nil)

// ParseTraceHeader parses an Amazon trace header to OpenCensus span context.
func ParseTraceHeader(header string) (trace.SpanContext, bool) {
	var (
		traceID      trace.TraceID
		traceIDSet   bool
		spanID       trace.SpanID
		traceOptions trace.TraceOptions
	)

	// Parse the parts of the amazon http trace id regardless of the
	// order the parts appear in.
	//
	// In most cases, the Root= will be the first part.  However, in
	// other cases (like an API Gateway proxy to an HTTP server), the
	// leading part will be Self=
	//
	parts := strings.Split(header, separator)
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, prefixRoot):
			v, err := parseAmazonTraceID(part[len(prefixRoot):])
			if err != nil {
				return trace.SpanContext{}, false
			}
			traceID = v
			traceIDSet = true

		case strings.HasPrefix(part, prefixParent):
			v, err := parseAmazonSpanID(part[len(prefixParent):])
			if err != nil {
				return trace.SpanContext{}, false
			}
			spanID = v

		case strings.HasPrefix(part, prefixSampled):
			if part[len(prefixParent)+1] == '1' {
				traceOptions = 1
			}

		default:
			// possibly a naked trace id.  because we're not sure, we won't bail
			// on failure.
			if v, err := parseAmazonTraceID(part); err == nil {
				traceID = v
				traceIDSet = true
			}
		}
	}
	if !traceIDSet {
		return trace.SpanContext{}, false
	}

	return trace.SpanContext{
		TraceID:      traceID,
		SpanID:       spanID,
		TraceOptions: traceOptions,
	}, true
}

// SpanContextFromRequest extracts an AWS X-Ray Trace span context from incoming requests.
func (f *HTTPFormat) SpanContextFromRequest(req *http.Request) (sc trace.SpanContext, ok bool) {
	h := req.Header.Get(httpHeader)

	// See https://docs.aws.amazon.com/xray/latest/devguide/xray-concepts.html
	// for the header format. Return if the header is empty or missing, or if
	// the header is unreasonably large, to avoid making unnecessary copies of
	// a large string.
	if h == "" || len(h) > httpHeaderMaxSize {
		return trace.SpanContext{}, false
	}
	return ParseTraceHeader(h)
}

// SpanContextToRequest modifies the given request to include a AWS X-Ray trace header.
func (f *HTTPFormat) SpanContextToRequest(sc trace.SpanContext, req *http.Request) {
	req.Header.Set(httpHeader, TraceHeader(sc))
}
