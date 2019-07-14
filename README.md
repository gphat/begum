# Begum

Begum is a tool that generates metrics similar to what would come out of an
HTTP-based web or microservice. These metrics can be manipulated by making API
calls to add and remove instances, adjust latencies, and error rates.

# Instances

Begum runs as many "instances" as you like. Each has a name — by default
sequentially `A, B, C…` etc and emits metrics.

## JSON

```
{
  "error_pct": 1,
  "latency_min_ms": 100,
  "latency_max_ms": 300,
  "latency_offset_ms": 0
}
```

# API

* `DELETE /instance/X` removes an instance named `X`, no body.
* `POST /instance/X` adds an instance named `X`, expects JSON
* `PUT /instance/X` adjusts the parameters of an instance named `X`, expects JSON

# Metrics Emitted

The following metrics are emitted:

* `request_duration_millis` with tags/labels `code` and `instance` with
quantiles `0.5, 0.9, 0.99`. Also includes `request_duration_millis_count` and
`request_duration_millis_sum`.
* `errors_encountered_total` with tags/labels `code` and `instance`.

# Name

[Begum](https://en.wikipedia.org/wiki/Begum) is a female royal and aristocratic
title from Central and South Asia.
