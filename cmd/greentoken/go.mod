module greentoken/cmd/greentoken

go 1.24

require (
	greentoken/agent v0.0.0
	greentoken/collector v0.0.0
	greentoken/pb v0.0.0
)

replace (
	greentoken/agent => ../../agent
	greentoken/collector => ../../collector
	greentoken/pb => ../../pb
)
