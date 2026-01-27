package persistence

import (
	"encoding/binary"
	"sync"

	"github.com/maxonlinux/meta-terminal-go/pkg/types"
)

var (
	orderBufPool   = sync.Pool{New: func() interface{} { return &bytesBuffer{data: make([]byte, 0, 128)} }}
	fundingBufPool = sync.Pool{New: func() interface{} { return &bytesBuffer{data: make([]byte, 0, 128)} }}
)

type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) reset() {
	b.data = b.data[:0]
}

func EncodeOrder(o *types.Order) []byte {
	buf := orderBufPool.Get().(*bytesBuffer)
	buf.reset()

	encFixed64(buf, uint64(o.ID))
	encFixed64(buf, uint64(o.UserID))

	encString(buf, o.Symbol)
	enc8(buf, byte(o.Category))
	enc8(buf, byte(o.Side))
	enc8(buf, byte(o.Type))
	enc8(buf, byte(o.TIF))
	enc8(buf, byte(o.Status))

	encFixed(buf, o.Price)
	encFixed(buf, o.Quantity)
	encFixed(buf, o.Filled)
	encFixed(buf, o.TriggerPrice)

	enc8(buf, boolToByte(o.ReduceOnly))
	enc8(buf, boolToByte(o.CloseOnTrigger))
	enc8(buf, byte(o.StopOrderType))
	enc8(buf, boolToByte(o.IsConditional))

	encFixed64(buf, o.CreatedAt)
	encFixed64(buf, o.UpdatedAt)

	result := make([]byte, len(buf.data))
	copy(result, buf.data)
	orderBufPool.Put(buf)
	return result
}

func DecodeOrder(data []byte) (*types.Order, error) {
	o := &types.Order{}
	off := 0

	o.ID = types.OrderID(decFixed64(data, &off))
	o.UserID = types.UserID(decFixed64(data, &off))

	o.Symbol = decString(data, &off)

	o.Category = int8(data[off])
	o.Side = int8(data[off+1])
	o.Type = int8(data[off+2])
	o.TIF = int8(data[off+3])
	o.Status = int8(data[off+4])
	off += 5

	o.Price = decFixed(data, &off)
	o.Quantity = decFixed(data, &off)
	o.Filled = decFixed(data, &off)
	o.TriggerPrice = decFixed(data, &off)

	o.ReduceOnly = data[off] == 1
	o.CloseOnTrigger = data[off+1] == 1
	o.StopOrderType = int8(data[off+2])
	o.IsConditional = data[off+3] == 1
	off += 4

	o.CreatedAt = decFixed64(data, &off)
	o.UpdatedAt = decFixed64(data, &off)

	return o, nil
}

func EncodeFunding(r *types.FundingRequest) []byte {
	buf := fundingBufPool.Get().(*bytesBuffer)
	buf.reset()

	encFixed64(buf, uint64(r.ID))
	encFixed64(buf, uint64(r.UserID))
	encString(buf, string(r.Type))
	encString(buf, string(r.Status))
	encString(buf, r.Asset)
	encFixed(buf, r.Amount)
	encString(buf, r.Destination)
	encString(buf, string(r.CreatedBy))
	encString(buf, r.Message)
	encFixed64(buf, r.CreatedAt)
	encFixed64(buf, r.UpdatedAt)

	result := make([]byte, len(buf.data))
	copy(result, buf.data)
	fundingBufPool.Put(buf)
	return result
}

func DecodeFunding(data []byte) (*types.FundingRequest, error) {
	r := &types.FundingRequest{}
	off := 0

	r.ID = types.FundingID(decFixed64(data, &off))
	r.UserID = types.UserID(decFixed64(data, &off))
	r.Type = types.FundingType(decString(data, &off))
	r.Status = types.FundingStatus(decString(data, &off))
	r.Asset = decString(data, &off)
	r.Amount = decFixed(data, &off)
	r.Destination = decString(data, &off)
	r.CreatedBy = types.FundingCreatedBy(decString(data, &off))
	r.Message = decString(data, &off)
	r.CreatedAt = decFixed64(data, &off)
	r.UpdatedAt = decFixed64(data, &off)

	return r, nil
}

func encFixed64(buf *bytesBuffer, v uint64) {
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], v)
	buf.data = append(buf.data, tmp[:]...)
}

func encFixed(buf *bytesBuffer, f types.Price) {
	data, _ := f.MarshalBinary()
	encString(buf, string(data))
}

func encString(buf *bytesBuffer, s string) {
	encVarint(buf, int64(len(s)))
	buf.data = append(buf.data, s...)
}

func encVarint(buf *bytesBuffer, v int64) {
	uv := uint64(v<<1) ^ uint64(v>>63)
	for uv >= 0x80 {
		buf.data = append(buf.data, byte(uv)|0x80)
		uv >>= 7
	}
	buf.data = append(buf.data, byte(uv))
}

func enc8(buf *bytesBuffer, b byte) {
	buf.data = append(buf.data, b)
}

func decFixed64(data []byte, off *int) uint64 {
	v := binary.BigEndian.Uint64(data[*off : *off+8])
	*off += 8
	return v
}

func decFixed(data []byte, off *int) types.Price {
	length := decVarint(data, off)
	start := *off
	*off += int(length)
	var f types.Price
	if err := f.UnmarshalBinary(data[start:*off]); err != nil {
		return types.Price{}
	}
	return f
}

func decString(data []byte, off *int) string {
	length := decVarint(data, off)
	start := *off
	*off += int(length)
	return string(data[start:*off])
}

func decVarint(data []byte, off *int) int64 {
	var result int64
	var shift uint
	for {
		b := data[*off]
		*off++
		result |= int64(b&0x7F) << shift
		if b < 0x80 {
			break
		}
		shift += 7
	}
	return int64(uint64(result)>>1) ^ -(result & 1)
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}
