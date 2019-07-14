#!/usr/bin/env bash

curl -i -H "Content-Type: application/json" \
  -X DELETE \
  http://localhost:8080/instance/B

curl -i -H "Content-Type: application/json" \
  -X POST \
  -d '{"error_pct":1,"latency_min_ms":100,"latency_max_ms":300,"latency_offset_ms":0}' \
  http://localhost:8080/instance/C
