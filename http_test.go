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

func handle(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}
}

func TestHttp(t *testing.T) {
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
		Handler:     handle("web"),
	}

	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	amazonTraceID := convertToAmazonTraceID(traceID)
	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com/index", strings.NewReader("hello"))

	w := httptest.NewRecorder()
	req.Header.Set(`X-Amzn-Trace-Id`, amazonTraceID)
	req.Header.Set(`User-Agent`, "ua")

	h.ServeHTTP(w, req)

	var content struct {
		Name        string
		Annotations struct {
			Path        string `json:"http.path"`
			RequestSize int    `json:"http.request_size"`
		}
		Http struct {
			Request struct {
				Method    string
				URL       string `json:"url"`
				UserAgent string `json:"user_agent"`
			}
		}
	}

	v := <-api.ch
	if err := json.NewDecoder(strings.NewReader(v)).Decode(&content); err != nil {
		t.Fatalf("unable to decode content, %v", err)
	}

	if expected := "www.example.com"; expected != content.Name {
		t.Errorf("want %v; got %v", expected, content.Name)
	}
	if expected := "/index"; expected != content.Annotations.Path {
		t.Errorf("want %v; got %v", expected, content.Annotations.Path)
	}
	if expected := http.MethodGet; expected != content.Http.Request.Method {
		t.Errorf("want %v; got %v", expected, content.Http.Request.Method)
	}
	if expected := "ua"; expected != content.Http.Request.UserAgent {
		t.Errorf("want %v; got %v", expected, content.Http.Request.UserAgent)
	}
}
