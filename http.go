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
	"go.opencensus.io/plugin/ochttp"
)

const (
	// UrlAttribute allows a custom url to be specified
	UrlAttribute = "http.url"
)

// httpRequest – Information about an http request.
type httpRequest struct {
	// Method – The request method. For example, GET.
	Method string `json:"method,omitempty"`

	// URL – The full URL of the request, compiled from the protocol, hostname,
	// and path of the request.
	URL string `json:"url,omitempty"`

	// UserAgent – The user agent string from the requester's client.
	UserAgent string `json:"user_agent,omitempty"`

	// ClientIP – The IP address of the requester. Can be retrieved from the IP
	// packet's Source Address or, for forwarded requests, from an X-Forwarded-For
	// header.
	ClientIP string `json:"client_ip,omitempty"`

	// XForwardedFor – (segments only) boolean indicating that the client_ip was
	// read from an X-Forwarded-For header and is not reliable as it could have
	// been forged.
	XForwardedFor string `json:"x_forwarded_for,omitempty"`

	// Traced – (subsegments only) boolean indicating that the downstream call
	// is to another traced service. If this field is set to true, X-Ray considers
	// the trace to be broken until the downstream service uploads a segment with
	// a parent_id that matches the id of the subsegment that contains this block.
	//
	// TODO - need to understand the impact of this field
	//Traced bool `json:"traced"`
}

// httpResponse - Information about an http response.
type httpResponse struct {
	// Status – number indicating the HTTP status of the response.
	Status int64 `json:"status,omitempty"`

	// ContentLength – number indicating the length of the response body in bytes.
	ContentLength int64 `json:"content_length,omitempty"`
}

type httpReqResp struct {
	Request  httpRequest  `json:"request"`
	Response httpResponse `json:"response"`
}

func makeHttp(attributes map[string]interface{}) (map[string]interface{}, *httpReqResp, string) {
	var (
		host     string
		path     string
		http     httpReqResp
		filtered = map[string]interface{}{}
	)

	for key, value := range attributes {
		switch key {
		case ochttp.HostAttribute:
			host, _ = value.(string)

		case ochttp.PathAttribute:
			path, _ = value.(string)

		case ochttp.MethodAttribute:
			http.Request.Method, _ = value.(string)

		case ochttp.UserAgentAttribute:
			http.Request.UserAgent, _ = value.(string)

		case ochttp.StatusCodeAttribute:
			http.Response.Status, _ = value.(int64)

		case UrlAttribute:
			http.Request.URL, _ = value.(string)

		default:
			filtered[key] = value
		}
	}

	if http.Request.URL == "" {
		http.Request.URL = host + path
	}

	if len(filtered) == len(attributes) {
		return attributes, nil, ""
	}

	return filtered, &http, host
}
