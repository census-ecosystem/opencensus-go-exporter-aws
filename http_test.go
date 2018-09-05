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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/aws/aws-sdk-go/service/xray/xrayiface"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
)

type httpTestSegments struct {
	xrayiface.XRayAPI
	ch chan string
}

func (m *httpTestSegments) PutTraceSegments(in *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error) {
	for _, doc := range in.TraceSegmentDocuments {
		m.ch <- *doc
	}
	return nil, nil
}

func handle(statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(statusCode)
		io.WriteString(w, "ok")
	}
}

func TestHttp(t *testing.T) {
	const (
		userAgent = "blah-agent"
		path      = "/index"
	)

	var (
		api         = &httpTestSegments{ch: make(chan string, 1)}
		exporter, _ = NewExporter(WithAPI(api), WithBufferSize(1))
	)

	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.AlwaysSample(),
	})

	var h = &ochttp.Handler{
		Propagation: &HTTPFormat{},
		Handler:     handle(http.StatusNotFound),
	}

	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	amazonTraceID := convertToAmazonTraceID(traceID)
	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com/index", strings.NewReader("hello"))

	w := httptest.NewRecorder()
	req.Header.Set(httpHeader, amazonTraceID)
	req.Header.Set(`User-Agent`, userAgent)

	h.ServeHTTP(w, req)

	var content struct {
		Name string
		Http struct {
			Request struct {
				Method    string
				URL       string `json:"url"`
				UserAgent string `json:"user_agent"`
			}
			Response struct {
				Status int
			}
		}
	}

	v := <-api.ch
	if err := json.Unmarshal([]byte(v), &content); err != nil {
		t.Fatalf("unable to decode content, %v", err)
	}

	if got, want := content.Name, path; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
	if got, want := content.Http.Request.Method, http.MethodGet; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
	if got, want := content.Http.Request.UserAgent, userAgent; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
	if got, want := content.Http.Request.URL, path; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
	if got, want := content.Http.Response.Status, http.StatusNotFound; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
}
