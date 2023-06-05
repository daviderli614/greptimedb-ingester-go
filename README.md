[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/GreptimeTeam/greptimedb-client-go/blob/main/LICENSE)
[![Build Status](https://github.com/greptimeteam/greptimedb-client-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GreptimeTeam/greptimedb-client-go/blob/main/.github/workflows/ci.yml)
[![codecov](https://codecov.io/gh/GreptimeTeam/greptimedb-client-go/branch/main/graph/badge.svg?token=76KIKITADQ)](https://codecov.io/gh/GreptimeTeam/greptimedb-client-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/GreptimeTeam/greptimedb-client-go.svg)](https://pkg.go.dev/github.com/GreptimeTeam/greptimedb-client-go)
# GreptimeDB Go Client

Provide API for using GreptimeDB client in Go.

## Installation

```sh
go get github.com/GreptimeTeam/greptimedb-client-go
```

## Example

you can visit [Documentation][document] for usage details and documentation,

## Usage

#### Datatype Supported

- int8, int16, int32, int64, int
- uint8, uint16, uint32, uint64, uint
- float32, float64
- bool
- string
- time.Time

#### Timestamp

you can customize timestamp index via calling methods of [Metric][metric_doc]

##### precision

The default timestamp column precision is `Millisecond`, you can set a different precision.
And once the precision is setted, you can not change it any more.

- time.Second
- time.Millisecond
- time.Microsecond
- time.Nanosecond

```go
metric.SetTimePrecision(time.Microsecond)
```

##### alias

The default timestamp column name is `ts`, if you want to use another name, you can change it:

```go
metric.SetTimestampAlias("timestamp")
```

#### Prometheus

How to query with RangePromql and Promql, you can visit [promql_test.go](query_promql_test.go) for details

#### Stream Insert

You can send several insert requests by `Send()` and notify GreptimeDB no more messages by `CloseAndRecv()`

You can visit [stream_client_test.go](stream_client_test.go) for details

## License

This greptimedb-client-go uses the __Apache 2.0 license__ to strike a balance
between open contributions and allowing you to use the software however you want.

<!-- links -->
[document]: https://pkg.go.dev/github.com/GreptimeTeam/greptimedb-client-go
[metric_doc]: https://pkg.go.dev/github.com/GreptimeTeam/greptimedb-client-go#Metric
