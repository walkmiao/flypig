module github.com/wendy512/iec104/examples/plugin-ha-template

go 1.24.0

require (
	github.com/hashicorp/go-plugin v1.6.3
	github.com/wendy512/go-iecp5 v1.2.5
	github.com/wendy512/iec104 v0.0.0
	google.golang.org/grpc v1.58.3
)

replace github.com/wendy512/iec104 => ../../

require (
	github.com/fatih/color v1.7.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/hashicorp/go-hclog v0.14.1 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
