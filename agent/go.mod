module greentoken/agent

go 1.24

require (
    greentoken/pb v0.0.0
    github.com/cilium/ebpf v0.17.3
    github.com/NVIDIA/go-nvml v0.12.4-1
    google.golang.org/grpc v1.71.0
)

replace greentoken/pb => ../pb
