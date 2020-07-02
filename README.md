aws
---
[![Build Status][travis-image]][travis-url] [![GoDoc][godoc-image]][godoc-url]

`aws` package defines an exporter that publishes spans to `AWS X-Ray`.

## Installation

```
go get contrib.go.opencensus.io/exporter/aws
```

## Quick Start

If your app is running inside AWS, the simplest way to use the Amazon exporter is:

```go
exporter, _ := aws.NewExporter()
trace.RegisterExporter(exporter)
```

If you're outside AWS, you'll need to make sure to configure the AWS credentials along
with the aws region where the traces should be published.  The simplest way to do this
is by setting the following environment variables:

* `AWS_ACCESS_KEY_ID` 
* `AWS_SECRET_ACCESS_KEY` 
* `AWS_DEFAULT_REGION` 

## Big Caveat

Currently, Amazon X-Ray TraceID's are incompatible with OpenCensus TraceIDs.  Consequently,
you must construct a valid TraceID and use http propagation.  

Fortunately, if you're running in AWS behind an `ELB` or `ALB`, the load balancer will
provide the necessary `X-Amzn-Trace-Id` header.  You can simply wrap your handler with
`ochttp.Handler` 

```go
h := &ochttp.Handler{
    Propagation: &aws.HTTPFormat{},
    Handler:     handler,
}
```   

The downside is if you're not running AWS e.g. your local desktop, getting traces is a pain 
for now.

## Credentials

The Amazon exporter uses `github.com/aws/aws-sdk-go` and so supports the common methods
for configuring credentials:

* Environment variables, `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
* Shared credentials,  `~/.aws/credentials`
* Role credentials: EC2, ECS, Lambda, etc

## Region

The Amazon exporter requires the aws region where traces will be set to be defined.  The
following methods are provided:

* Environment variable, `AWS_DEFAULT_REGION` or `AWS_REGION`
* Explicitly via the `WithRegion` option e.g. `aws.NewExporter(aws.WithRegion("us-west-2"))`

If neither of these are specified, the Amazon exporter will attempt to query the aws
metadata url, `http://169.254.169.254/latest/meta-data/placement/availability-zone`, and
publish traces to the current region.

[godoc-image]: https://godoc.org/contrib.go.opencensus.io/exporter/aws?status.svg
[godoc-url]: https://godoc.org/contrib.go.opencensus.io/exporter/aws
[travis-image]: https://travis-ci.org/census-ecosystem/opencensus-go-exporter-aws.svg?branch=master
[travis-url]: https://travis-ci.org/census-ecosystem/opencensus-go-exporter-aws