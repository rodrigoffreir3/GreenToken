module greentoken/collector

go 1.24

require (
    greentoken/pb v0.0.0
    github.com/prometheus/client_golang v1.22.0
    google.golang.org/grpc v1.71.0
)

replace greentoken/pb => ../pb
