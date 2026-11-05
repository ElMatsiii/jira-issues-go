module jira-issues-go

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
	github.com/segmentio/kafka-go v0.4.47
	github.com/stretchr/testify v1.9.0
	google.golang.org/grpc v1.65.0
	jira-contracts v0.0.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace jira-contracts => ../jira-contracts
