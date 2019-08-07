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
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opencensus.io/trace"
)

func TestSpanContextFromRequest(t *testing.T) {
	var (
		format  = &HTTPFormat{}
		traceID = trace.TraceID{0x5a, 0x96, 0x12, 0xa2, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10}
		spanID  = trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
		epoch   = time.Now().Unix()
	)

	binary.BigEndian.PutUint32(traceID[0:4], uint32(epoch))

	t.Run("no header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		_, ok := format.SpanContextFromRequest(req)
		if ok {
			t.Errorf("expected false; got true")
		}
	})

	t.Run("traceID only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		amazonTraceID := convertToAmazonTraceID(traceID)
		req.Header.Set(httpHeader, amazonTraceID)

		sc, ok := format.SpanContextFromRequest(req)
		if !ok {
			t.Errorf("expected true; got false")
		}
		if traceID != sc.TraceID {
			t.Errorf("expected %v; got %v", traceID, sc.TraceID)
		}
		if zeroSpanID != sc.SpanID {
			t.Errorf("expected true; got false")
		}
		if 0 != sc.TraceOptions {
			t.Errorf("expected 1; got %v", sc.TraceOptions)
		}
	})

	t.Run("traceID only with root prefix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		amazonTraceID := convertToAmazonTraceID(traceID)
		req.Header.Set(httpHeader, prefixRoot+amazonTraceID)

		sc, ok := format.SpanContextFromRequest(req)
		if !ok {
			t.Errorf("expected true; got false")
		}
		if traceID != sc.TraceID {
			t.Errorf("expected %v; got %v", traceID, sc.TraceID)
		}
		if zeroSpanID != sc.SpanID {
			t.Errorf("expected true; got false")
		}
		if 0 != sc.TraceOptions {
			t.Errorf("expected 1; got %v", sc.TraceOptions)
		}
	})

	t.Run("traceID with parentSpanID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		amazonTraceID := convertToAmazonTraceID(traceID)
		amazonSpanID := convertToAmazonSpanID(spanID)
		req.Header.Set(httpHeader, prefixRoot+amazonTraceID+";"+prefixParent+amazonSpanID)

		sc, ok := format.SpanContextFromRequest(req)
		if !ok {
			t.Errorf("expected true; got false")
		}
		if traceID != sc.TraceID {
			t.Errorf("expected %v; got %v", traceID, sc.TraceID)
		}
		if spanID != sc.SpanID {
			t.Errorf("expected %v; got %v", spanID, sc.SpanID)
		}
		if 0 != sc.TraceOptions {
			t.Errorf("expected 1; got %v", sc.TraceOptions)
		}
	})

	t.Run("traceID with parentSpanID and sampled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		amazonTraceID := convertToAmazonTraceID(traceID)
		amazonSpanID := convertToAmazonSpanID(spanID)
		req.Header.Set(httpHeader, prefixRoot+amazonTraceID+";"+prefixParent+amazonSpanID+";"+prefixSampled+"1")

		sc, ok := format.SpanContextFromRequest(req)
		if !ok {
			t.Errorf("expected true; got false")
		}
		if traceID != sc.TraceID {
			t.Errorf("expected %v; got %v", traceID, sc.TraceID)
		}
		if spanID != sc.SpanID {
			t.Errorf("expected %v; got %v", spanID, sc.SpanID)
		}
		if 1 != sc.TraceOptions {
			t.Errorf("expected 1; got %v", sc.TraceOptions)
		}
	})

	t.Run("bad traceID", func(t *testing.T) {
		var (
			req = httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		)
		req.Header.Set(httpHeader, "1-bad-junk")

		_, ok := format.SpanContextFromRequest(req)
		if ok {
			t.Errorf("expected false; got true")
		}
	})

	t.Run("bad spanID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/", nil)
		amazonTraceID := convertToAmazonTraceID(traceID)
		req.Header.Set(httpHeader, prefixRoot+amazonTraceID+";"+prefixParent+"junk-span")

		_, ok := format.SpanContextFromRequest(req)
		if ok {
			t.Errorf("expected false; got true")
		}
	})
}

func TestSpanContextToRequest(t *testing.T) {
	var (
		format  = &HTTPFormat{}
		traceID = trace.TraceID{0x5a, 0x96, 0x12, 0xa2, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10}
		spanID  = trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
		req, _  = http.NewRequest(http.MethodGet, "http://localhost/", nil)
		epoch   = time.Now().Unix()
	)

	binary.BigEndian.PutUint32(traceID[0:4], uint32(epoch)) // ensure epoch
	hexEpoch := hex.EncodeToString(traceID[0:4])

	t.Run("trace on", func(t *testing.T) {
		var sc = trace.SpanContext{
			TraceID:      traceID,
			SpanID:       spanID,
			TraceOptions: 1,
		}
		format.SpanContextToRequest(sc, req)
		v := req.Header.Get(httpHeader)
		if expected := fmt.Sprintf("Root=1-%v-05060708090a0b0c0d0e0f10;Parent=0102030405060708;Sampled=1", hexEpoch); expected != v {
			t.Errorf("got %v; expected %v", expected, v)
		}
	})

	t.Run("trace off", func(t *testing.T) {
		var sc = trace.SpanContext{
			TraceID:      traceID,
			SpanID:       spanID,
			TraceOptions: 0,
		}
		format.SpanContextToRequest(sc, req)
		v := req.Header.Get(httpHeader)
		if expected := fmt.Sprintf("Root=1-%v-05060708090a0b0c0d0e0f10;Parent=0102030405060708;Sampled=0", hexEpoch); expected != v {
			t.Errorf("got %v; expected %v", expected, v)
		}
	})
}

func TestParseTraceHeader(t *testing.T) {
	testCases := map[string]struct {
		header   string
		traceID  string
		parentID string
		sampled  trace.TraceOptions
		ok       bool
	}{
		"not-present": {
			header:   "",
			traceID:  "",
			parentID: "",
			sampled:  0,
			ok:       false,
		},
		"root only": {
			header:   "Root=1-5983f5c9-56dcf0bc6d4d214d2dbbe8c6",
			traceID:  "5983f5c956dcf0bc6d4d214d2dbbe8c6",
			parentID: "0000000000000000",
			ok:       true,
			sampled:  0,
		},
		"naked trace id": {
			header:   "1-5983f5c9-56dcf0bc6d4d214d2dbbe8c6",
			traceID:  "5983f5c956dcf0bc6d4d214d2dbbe8c6",
			parentID: "0000000000000000",
			ok:       true,
			sampled:  0,
		},
		"root and parent": {
			header:   "Root=1-5983f5c9-56dcf0bc6d4d214d2dbbe8c6;Parent=0102030405060708",
			traceID:  "5983f5c956dcf0bc6d4d214d2dbbe8c6",
			parentID: "0102030405060708",
			ok:       true,
			sampled:  0,
		},
		"with sampling": {
			header:   "Root=1-5983f5c9-56dcf0bc6d4d214d2dbbe8c6;Parent=0102030405060708;Sampled=1",
			traceID:  "5983f5c956dcf0bc6d4d214d2dbbe8c6",
			parentID: "0102030405060708",
			ok:       true,
			sampled:  1,
		},
		"parent before root": {
			header:   "Parent=0102030405060708;Root=1-5983f5c9-56dcf0bc6d4d214d2dbbe8c6",
			traceID:  "5983f5c956dcf0bc6d4d214d2dbbe8c6",
			parentID: "0102030405060708",
			ok:       true,
			sampled:  0,
		},
		"self is ignored": {
			header:   "Self=blah;Parent=0102030405060708;Root=1-5983f5c9-56dcf0bc6d4d214d2dbbe8c6",
			traceID:  "5983f5c956dcf0bc6d4d214d2dbbe8c6",
			parentID: "0102030405060708",
			ok:       true,
			sampled:  0,
		},
	}

	for label, tc := range testCases {
		t.Run(label, func(t *testing.T) {
			spanContext, ok := ParseTraceHeader(tc.header)
			if got, want := ok, tc.ok; got != want {
				t.Fatalf("got %v; want %v", got, want)
			}
			if ok {
				if got, want := spanContext.TraceID.String(), tc.traceID; got != want {
					t.Fatalf("got %v; want %v", got, want)
				}
				if got, want := spanContext.SpanID.String(), tc.parentID; got != want {
					t.Fatalf("got %v; want %v", got, want)
				}
			}
		})
	}
}
