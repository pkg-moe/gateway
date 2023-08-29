package grpc_websocket

import (
	//"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/proto"
)

// WebsocketMarshaller is a Marshaller which marshals/unmarshals into/from serialize proto bytes
type WebsocketMarshaller struct{}

// ContentType always returns "application/grpc-websocket".
func (*WebsocketMarshaller) ContentType(_ interface{}) string {
	return "application/grpc-websocket"
}

// Marshal marshals "value" into Proto
func (*WebsocketMarshaller) Marshal(value interface{}) ([]byte, error) {
	result, ok := value.(map[string]interface{})
	if !ok {
		return nil, errors.New("unable to marshal non result field")
	}

	message, ok := result["result"].(proto.Message)
	if !ok {
		return nil, errors.New("unable to marshal non proto field")
	}

	buffer, err := proto.Marshal(message)

	if err != nil {
		return nil, err
	}

	l := make([]byte, 4)
	binary.BigEndian.PutUint32(l, uint32(len(buffer)))

	body := append([]byte{0b00000000}, append(l, buffer...)...)
	body = append(body, []byte{0b11111111, 0b10000000, 0b11111111, 0b10000000, 0b11111111, 0b10000001}...)

	return body, nil
}

// Unmarshal unmarshals proto "data" into "value"
func (*WebsocketMarshaller) Unmarshal(data []byte, value interface{}) error {
	message, ok := value.(proto.Message)
	if !ok {
		return errors.New("unable to unmarshal non proto field")
	}
	return proto.Unmarshal(data, message)
}

// NewDecoder returns a Decoder which reads proto stream from "reader".
func (marshaller *WebsocketMarshaller) NewDecoder(reader io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(value interface{}) error {
		compression := make([]byte, 1)
		if _, err := reader.Read(compression); err != nil {
			return err
		}

		lens := make([]byte, 4)
		if _, err := reader.Read(lens); err != nil {
			return err
		}
		l := binary.BigEndian.Uint32(lens)
		buffer := make([]byte, l)

		if _, err := reader.Read(buffer); err != nil {
			return err
		}

		return marshaller.Unmarshal(buffer, value)
	})
}

// NewEncoder returns an Encoder which writes proto stream into "writer".
func (marshaller *WebsocketMarshaller) NewEncoder(writer io.Writer) runtime.Encoder {
	return runtime.EncoderFunc(func(value interface{}) error {
		buffer, err := marshaller.Marshal(value)
		if err != nil {
			return err
		}

		l := make([]byte, 4)
		binary.BigEndian.PutUint32(l, uint32(len(buffer)))

		body := append([]byte{0x00}, append(l, buffer...)...)

		if _, err := writer.Write(body); err != nil {
			return err
		}

		return nil
	})
}
