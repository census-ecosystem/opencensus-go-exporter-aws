aws
---

[![GoDoc][godoc-image]][godoc-url]

`aws` package defines an exporter that publishes spans to `AWS X-Ray`.

## Installation

```
go get go.opencensus.io/exporter/aws
```

#### To Do 

- [x] ~~Publish spans in a separate goroutine~~
- [x] ~~Support propagation of http spans~~
- [x] ~~Support remote spans~~
- [x] ~~Verified works with ELB/ALB~~
- [x] ~~Report errors / exceptions~~
- [x] ~~Publish partial segments; currently only completed segments are published to aws~~

[godoc-image]: https://godoc.org/go.opencensus.io/exporter/aws?status.svg
[godoc-url]: https://godoc.org/go.opencensus.io/exporter/aws
