> **Warning**
>
> OpenCensus and OpenTracing have merged to form [OpenTelemetry](https://opentelemetry.io), which serves as the next major version of OpenCensus and OpenTracing.
>
> OpenTelemetry has now reached feature parity with OpenCensus, with tracing and metrics SDKs available in .NET, Golang, Java, NodeJS, and Python. **All OpenCensus Github repositories, except [census-instrumentation/opencensus-python](https://github.com/census-instrumentation/opencensus-python), will be archived on July 31st, 2023**. We encourage users to migrate to OpenTelemetry by this date.
>
> To help you gradually migrate your instrumentation to OpenTelemetry, bridges are available in Java, Go, Python, and JS. [**Read the full blog post to learn more**](https://opentelemetry.io/blog/2023/sunsetting-opencensus/).

aws
---
[![Build Status][travis-image]][travis-url] [![GoDoc][godoc-image]][godoc-url]

`aws` package defines an exporter that publishes spans to `AWS X-Ray`.

## Installation

```
go get contrib.go.opencensus.io/exporter/aws
```

#### To Do 

- [x] ~~Publish spans in a separate goroutine~~
- [x] ~~Support propagation of http spans~~
- [x] ~~Support remote spans~~
- [x] ~~Verified works with ELB/ALB~~
- [x] ~~Report errors / exceptions~~
- [x] ~~Publish partial segments; currently only completed segments are published to aws~~

[godoc-image]: https://godoc.org/contrib.go.opencensus.io/exporter/aws?status.svg
[godoc-url]: https://godoc.org/contrib.go.opencensus.io/exporter/aws
[travis-image]: https://travis-ci.org/census-ecosystem/opencensus-go-exporter-aws.svg?branch=master
[travis-url]: https://travis-ci.org/census-ecosystem/opencensus-go-exporter-aws