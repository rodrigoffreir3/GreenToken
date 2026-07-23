module greentoken/cmd/greentoken

go 1.24

require greentoken/agent v0.0.0

require github.com/NVIDIA/go-nvml v0.12.4-1 // indirect

replace (
	greentoken/agent => ../../agent
	greentoken/collector => ../../collector
	greentoken/pb => ../../pb
)
