package grpcplugin

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const codecName = "json"

type jsonCodec struct{}

func (jsonCodec) Name() string {
	return codecName
}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}
