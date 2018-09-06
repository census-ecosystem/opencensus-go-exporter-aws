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