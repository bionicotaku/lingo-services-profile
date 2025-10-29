// Package cataloginbox provides Catalog Inbox runners that hydrate profile video projections.
package cataloginbox

import (
	"fmt"

	videov1 "github.com/bionicotaku/lingo-services-catalog/api/video/v1"
	"google.golang.org/protobuf/proto"
)

// decoder 实现 inbox.Decoder 接口，将 Pub/Sub payload 解析为 Catalog 事件。
type decoder struct{}

// newDecoder 构造事件解码器。
func newDecoder() *decoder {
	return &decoder{}
}

// Decode 解析事件载荷。
func (d *decoder) Decode(data []byte) (*videov1.Event, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("catalog inbox: empty payload")
	}
	evt := &videov1.Event{}
	if err := proto.Unmarshal(data, evt); err != nil {
		return nil, fmt.Errorf("catalog inbox: unmarshal event: %w", err)
	}
	return evt, nil
}
