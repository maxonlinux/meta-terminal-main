package snapshot

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func Encode(w io.Writer, snap *Snapshot) error {
	bw := bufio.NewWriterSize(w, 64*1024)
	if err := writeUint64(bw, snap.TakenAt); err != nil {
		return err
	}
	if err := writeUint64(bw, snap.IDGenLastMS); err != nil {
		return err
	}
	if err := writeUint16(bw, snap.IDGenSeq); err != nil {
		return err
	}
	if err := writeUint32(bw, uint32(len(snap.Instruments))); err != nil {
		return err
	}
	for _, inst := range snap.Instruments {
		if err := writeString(bw, inst.Symbol); err != nil {
			return err
		}
		if err := writeString(bw, inst.BaseAsset); err != nil {
			return err
		}
		if err := writeString(bw, inst.QuoteAsset); err != nil {
			return err
		}
		if err := writeInt8(bw, inst.Category); err != nil {
			return err
		}
	}
	if err := writeUint32(bw, uint32(len(snap.Prices))); err != nil {
		return err
	}
	for _, p := range snap.Prices {
		if err := writeString(bw, p.Symbol); err != nil {
			return err
		}
		if err := writeInt64(bw, int64(p.Price)); err != nil {
			return err
		}
	}
	if err := writeUint32(bw, uint32(len(snap.Users))); err != nil {
		return err
	}
	for _, user := range snap.Users {
		if err := writeUint64(bw, uint64(user.UserID)); err != nil {
			return err
		}
		if err := writeUint32(bw, uint32(len(user.Balances))); err != nil {
			return err
		}
		for _, bal := range user.Balances {
			if err := writeString(bw, bal.Asset); err != nil {
				return err
			}
			for i := 0; i < 3; i++ {
				if err := writeInt64(bw, bal.Buckets[i]); err != nil {
					return err
				}
			}
		}
		if err := writeUint32(bw, uint32(len(user.Positions))); err != nil {
			return err
		}
		for _, pos := range user.Positions {
			if err := writeString(bw, pos.Symbol); err != nil {
				return err
			}
			if err := writeInt64(bw, int64(pos.Size)); err != nil {
				return err
			}
			if err := writeInt8(bw, pos.Side); err != nil {
				return err
			}
			if err := writeInt64(bw, int64(pos.EntryPrice)); err != nil {
				return err
			}
			if err := writeInt8(bw, pos.Leverage); err != nil {
				return err
			}
			if err := writeInt64(bw, pos.InitialMargin); err != nil {
				return err
			}
			if err := writeInt64(bw, pos.MaintenanceMargin); err != nil {
				return err
			}
			if err := writeInt64(bw, int64(pos.LiquidationPrice)); err != nil {
				return err
			}
			if err := writeInt64(bw, pos.Version); err != nil {
				return err
			}
		}
	}
	if err := writeUint32(bw, uint32(len(snap.Orders))); err != nil {
		return err
	}
	for i := range snap.Orders {
		order := snap.Orders[i]
		if err := writeUint64(bw, uint64(order.ID)); err != nil {
			return err
		}
		if err := writeUint64(bw, uint64(order.UserID)); err != nil {
			return err
		}
		if err := writeString(bw, order.Symbol); err != nil {
			return err
		}
		if err := writeInt8(bw, order.Category); err != nil {
			return err
		}
		if err := writeInt8(bw, order.Side); err != nil {
			return err
		}
		if err := writeInt8(bw, order.Type); err != nil {
			return err
		}
		if err := writeInt8(bw, order.TIF); err != nil {
			return err
		}
		if err := writeInt8(bw, order.Status); err != nil {
			return err
		}
		if err := writeInt64(bw, int64(order.Price)); err != nil {
			return err
		}
		if err := writeInt64(bw, int64(order.Quantity)); err != nil {
			return err
		}
		if err := writeInt64(bw, int64(order.Filled)); err != nil {
			return err
		}
		if err := writeInt64(bw, int64(order.TriggerPrice)); err != nil {
			return err
		}
		if err := writeUint8(bw, boolToByte(order.ReduceOnly)); err != nil {
			return err
		}
		if err := writeUint8(bw, boolToByte(order.CloseOnTrigger)); err != nil {
			return err
		}
		if err := writeInt8(bw, order.StopOrderType); err != nil {
			return err
		}
		if err := writeInt8(bw, order.Leverage); err != nil {
			return err
		}
		if err := writeUint64(bw, order.CreatedAt); err != nil {
			return err
		}
		if err := writeUint64(bw, order.UpdatedAt); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func Decode(r io.Reader) (*Snapshot, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	snap := &Snapshot{}
	var err error
	if snap.TakenAt, err = readUint64(br); err != nil {
		return nil, err
	}
	if snap.IDGenLastMS, err = readUint64(br); err != nil {
		return nil, err
	}
	if snap.IDGenSeq, err = readUint16(br); err != nil {
		return nil, err
	}
	instCount, err := readUint32(br)
	if err != nil {
		return nil, err
	}
	if instCount > 0 {
		snap.Instruments = make([]Instrument, instCount)
	}
	for i := 0; i < int(instCount); i++ {
		sym, err := readString(br)
		if err != nil {
			return nil, err
		}
		base, err := readString(br)
		if err != nil {
			return nil, err
		}
		quote, err := readString(br)
		if err != nil {
			return nil, err
		}
		cat, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		snap.Instruments[i] = Instrument{Symbol: sym, BaseAsset: base, QuoteAsset: quote, Category: cat}
	}
	priceCount, err := readUint32(br)
	if err != nil {
		return nil, err
	}
	if priceCount > 0 {
		snap.Prices = make([]Price, priceCount)
	}
	for i := 0; i < int(priceCount); i++ {
		sym, err := readString(br)
		if err != nil {
			return nil, err
		}
		p, err := readInt64(br)
		if err != nil {
			return nil, err
		}
		snap.Prices[i] = Price{Symbol: sym, Price: types.Price(p)}
	}
	userCount, err := readUint32(br)
	if err != nil {
		return nil, err
	}
	if userCount > 0 {
		snap.Users = make([]User, userCount)
	}
	for i := 0; i < int(userCount); i++ {
		uid, err := readOrderID(br)
		if err != nil {
			return nil, err
		}
		balCount, err := readUint32(br)
		if err != nil {
			return nil, err
		}
		user := User{UserID: types.UserID(uid)}
		if balCount > 0 {
			user.Balances = make([]Balance, balCount)
		}
		for b := 0; b < int(balCount); b++ {
			asset, err := readString(br)
			if err != nil {
				return nil, err
			}
			bal := Balance{Asset: asset}
			for j := 0; j < 3; j++ {
				val, err := readInt64(br)
				if err != nil {
					return nil, err
				}
				bal.Buckets[j] = val
			}
			user.Balances[b] = bal
		}
		posCount, err := readUint32(br)
		if err != nil {
			return nil, err
		}
		if posCount > 0 {
			user.Positions = make([]Position, posCount)
		}
		for p := 0; p < int(posCount); p++ {
			sym, err := readString(br)
			if err != nil {
				return nil, err
			}
			size, err := readInt64(br)
			if err != nil {
				return nil, err
			}
			side, err := readInt8(br)
			if err != nil {
				return nil, err
			}
			entry, err := readInt64(br)
			if err != nil {
				return nil, err
			}
			lev, err := readInt8(br)
			if err != nil {
				return nil, err
			}
			initM, err := readInt64(br)
			if err != nil {
				return nil, err
			}
			maintM, err := readInt64(br)
			if err != nil {
				return nil, err
			}
			liq, err := readInt64(br)
			if err != nil {
				return nil, err
			}
			ver, err := readInt64(br)
			if err != nil {
				return nil, err
			}
			user.Positions[p] = Position{
				Symbol:            sym,
				Size:              types.Quantity(size),
				Side:              side,
				EntryPrice:        types.Price(entry),
				Leverage:          lev,
				InitialMargin:     initM,
				MaintenanceMargin: maintM,
				LiquidationPrice:  types.Price(liq),
				Version:           ver,
			}
		}
		snap.Users[i] = user
	}
	orderCount, err := readUint32(br)
	if err != nil {
		return nil, err
	}
	if orderCount > 0 {
		snap.Orders = make([]types.Order, orderCount)
	}
	for i := 0; i < int(orderCount); i++ {
		id, err := readOrderID(br)
		if err != nil {
			return nil, err
		}
		uid, err := readOrderID(br)
		if err != nil {
			return nil, err
		}
		sym, err := readString(br)
		if err != nil {
			return nil, err
		}
		cat, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		side, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		typeVal, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		tif, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		status, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		price, err := readInt64(br)
		if err != nil {
			return nil, err
		}
		qty, err := readInt64(br)
		if err != nil {
			return nil, err
		}
		filled, err := readInt64(br)
		if err != nil {
			return nil, err
		}
		trigger, err := readInt64(br)
		if err != nil {
			return nil, err
		}
		reduceOnly, err := readUint8(br)
		if err != nil {
			return nil, err
		}
		closeTrigger, err := readUint8(br)
		if err != nil {
			return nil, err
		}
		stopType, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		lev, err := readInt8(br)
		if err != nil {
			return nil, err
		}
		created, err := readUint64(br)
		if err != nil {
			return nil, err
		}
		updated, err := readUint64(br)
		if err != nil {
			return nil, err
		}
		snap.Orders[i] = types.Order{
			ID:             types.OrderID(id),
			UserID:         types.UserID(uid),
			Symbol:         sym,
			Category:       cat,
			Side:           side,
			Type:           typeVal,
			TIF:            tif,
			Status:         status,
			Price:          types.Price(price),
			Quantity:       types.Quantity(qty),
			Filled:         types.Quantity(filled),
			TriggerPrice:   types.Price(trigger),
			ReduceOnly:     reduceOnly == 1,
			CloseOnTrigger: closeTrigger == 1,
			StopOrderType:  stopType,
			Leverage:       lev,
			CreatedAt:      created,
			UpdatedAt:      updated,
		}
	}
	return snap, nil
}

func writeUint64(w io.Writer, v uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeUint32(w io.Writer, v uint32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeUint16(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeInt64(w io.Writer, v int64) error {
	return writeUint64(w, uint64(v))
}

func writeInt8(w io.Writer, v int8) error {
	return writeUint8(w, uint8(v))
}

func writeUint8(w io.Writer, v uint8) error {
	var buf [1]byte
	buf[0] = v
	_, err := w.Write(buf[:])
	return err
}

func writeString(w io.Writer, s string) error {
	if err := writeUint32(w, uint32(len(s))); err != nil {
		return err
	}
	if len(s) == 0 {
		return nil
	}
	_, err := w.Write([]byte(s))
	return err
}

func readUint64(r io.Reader) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

func readUint32(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func readUint16(r io.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

func readInt64(r io.Reader) (int64, error) {
	v, err := readUint64(r)
	return int64(v), err
}

func readInt8(r io.Reader) (int8, error) {
	v, err := readUint8(r)
	return int8(v), err
}

func readUint8(r io.Reader) (uint8, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

func readString(r io.Reader) (string, error) {
	length, err := readUint32(r)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readOrderID(r io.Reader) (types.OrderID, error) {
	v, err := readUint64(r)
	return types.OrderID(v), err
}

func boolToByte(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}
