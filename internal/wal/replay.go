package wal

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anomalyco/meta-terminal-go/internal/balance"
	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/memory"
	"github.com/anomalyco/meta-terminal-go/internal/orderbook"
	"github.com/anomalyco/meta-terminal-go/internal/position"
	"github.com/anomalyco/meta-terminal-go/internal/state"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

func (w *WAL) Replay(state *state.State, startOffset int64, ob *orderbook.OrderBook) error {
	file, err := os.Open(filepath.Join(w.path, "wal.log"))
	if err != nil {
		return fmt.Errorf("failed to open WAL: %w", err)
	}
	defer func() { _ = file.Close() }()

	reader := NewWALReader(file, w.bufferSize)

	if startOffset > 0 {
		if err := reader.Discard(startOffset); err != nil {
			return fmt.Errorf("failed to seek to offset %d: %w", startOffset, err)
		}
	}

	for {
		op, err := reader.ReadOperation()
		if err != nil {
			if err == ErrEndOfWAL {
				break
			}
			return fmt.Errorf("failed to read operation: %w", err)
		}

		if err := replayOperation(state, op, ob); err != nil {
			return fmt.Errorf("failed to replay op %d: %w", op.Type, err)
		}
	}

	return nil
}

func replayOperation(st *state.State, op *Operation, ob *orderbook.OrderBook) error {
	switch op.Type {
	case OP_PLACE_ORDER:
		return replayPlaceOrder(st, op)
	case OP_CANCEL_ORDER:
		return replayCancelOrder(st, op)
	case OP_AMEND_ORDER:
		return replayAmendOrder(st, op)
	case OP_FILL:
		return replayFill(st, op)
	case OP_POSITION_UPDATE:
		return replayPositionUpdate(st, op)
	case OP_BALANCE_UPDATE:
		return replayBalanceUpdate(st, op)
	}
	return nil
}

func replayPlaceOrder(st *state.State, op *Operation) error {
	order := memory.GetOrderPool().Get()
	order.ID = types.OrderID(op.OrderID)
	order.UserID = types.UserID(op.UserID)
	order.Symbol = types.SymbolID(op.Symbol)
	order.Status = constants.ORDER_STATUS_NEW
	order.Filled = 0

	if st.NextOrderID <= types.OrderID(op.OrderID) {
		st.NextOrderID = types.OrderID(op.OrderID) + 1
	}

	if op.Data != nil && len(op.Data) >= 24 {
		order.Side = int8(op.Data[0])
		order.Type = int8(op.Data[1])
		order.TIF = int8(op.Data[2])
		order.Quantity = types.Quantity(binary.BigEndian.Uint64(op.Data[3:11]))
		order.Price = types.Price(binary.BigEndian.Uint64(op.Data[11:19]))
		order.TriggerPrice = types.Price(binary.BigEndian.Uint64(op.Data[19:27]))
	}

	return nil
}

func replayCancelOrder(st *state.State, op *Operation) error {
	return nil
}

func replayAmendOrder(st *state.State, op *Operation) error {
	return nil
}

func replayFill(st *state.State, op *Operation) error {
	if op.Data == nil || len(op.Data) < 56 {
		return fmt.Errorf("invalid fill data")
	}

	fill := &FillOp{
		TakerOrderID: int64(binary.BigEndian.Uint64(op.Data[0:8])),
		MakerOrderID: int64(binary.BigEndian.Uint64(op.Data[8:16])),
		Price:        int64(binary.BigEndian.Uint64(op.Data[16:24])),
		Quantity:     int64(binary.BigEndian.Uint64(op.Data[24:32])),
		Symbol:       int32(binary.BigEndian.Uint32(op.Data[32:36])),
		TakerUserID:  int64(binary.BigEndian.Uint64(op.Data[36:44])),
		MakerUserID:  int64(binary.BigEndian.Uint64(op.Data[44:52])),
	}

	usTaker := st.GetUserState(types.UserID(fill.TakerUserID))
	oldTakerMargin := int64(0)
	if posTaker := usTaker.Positions[types.SymbolID(fill.Symbol)]; posTaker != nil {
		oldTakerMargin = posTaker.InitialMargin
	}
	_, takerPnl := position.UpdatePosition(st, types.UserID(fill.TakerUserID), types.SymbolID(fill.Symbol), types.Quantity(fill.Quantity), types.Price(fill.Price), 0, 2)

	usMaker := st.GetUserState(types.UserID(fill.MakerUserID))
	oldMakerMargin := int64(0)
	if posMaker := usMaker.Positions[types.SymbolID(fill.Symbol)]; posMaker != nil {
		oldMakerMargin = posMaker.InitialMargin
	}
	_, makerPnl := position.UpdatePosition(st, types.UserID(fill.MakerUserID), types.SymbolID(fill.Symbol), types.Quantity(fill.Quantity), types.Price(fill.Price), 1, 2)

	tBal := balance.GetOrCreate(st, types.UserID(fill.TakerUserID), "USDT")
	takerNewMargin := usTaker.Positions[types.SymbolID(fill.Symbol)].InitialMargin
	tBal.Margin += takerNewMargin - oldTakerMargin
	tBal.Available += takerPnl

	mBal := balance.GetOrCreate(st, types.UserID(fill.MakerUserID), "USDT")
	makerNewMargin := usMaker.Positions[types.SymbolID(fill.Symbol)].InitialMargin
	mBal.Margin += makerNewMargin - oldMakerMargin
	mBal.Available += makerPnl

	orderMargin := position.CalculateMargin(types.Quantity(0), types.Price(0), 2)
	filledRatio := types.Quantity(fill.Quantity) / types.Quantity(fill.Quantity)
	unlockAmount := orderMargin * int64(filledRatio)
	mBal.Locked -= unlockAmount

	return nil
}

func replayPositionUpdate(st *state.State, op *Operation) error {
	if op.Data == nil || len(op.Data) < 32 {
		return fmt.Errorf("invalid position update data")
	}

	userID := types.UserID(binary.BigEndian.Uint64(op.Data[0:8]))
	symbol := types.SymbolID(binary.BigEndian.Uint32(op.Data[8:12]))
	size := types.Quantity(binary.BigEndian.Uint64(op.Data[12:20]))
	side := int8(op.Data[20])
	entryPrice := types.Price(binary.BigEndian.Uint64(op.Data[21:29]))
	leverage := int8(op.Data[29])

	us := st.GetUserState(userID)

	if size == 0 {
		delete(us.Positions, symbol)
		return nil
	}

	pos := us.Positions[symbol]
	if pos == nil {
		pos = &types.Position{
			UserID:   userID,
			Symbol:   symbol,
			Size:     size,
			Side:     side,
			Leverage: leverage,
		}
		us.Positions[symbol] = pos
	}

	pos.Size = size
	pos.Side = side
	pos.EntryPrice = entryPrice
	pos.Leverage = leverage
	position.CalculatePositionRisk(pos)

	return nil
}

func replayBalanceUpdate(st *state.State, op *Operation) error {
	if op.Data == nil || len(op.Data) < 32 {
		return fmt.Errorf("invalid balance update data")
	}

	userID := types.UserID(binary.BigEndian.Uint64(op.Data[0:8]))
	available := binary.BigEndian.Uint64(op.Data[8:16])
	locked := binary.BigEndian.Uint64(op.Data[16:24])
	margin := binary.BigEndian.Uint64(op.Data[24:32])
	assetLen := len(op.Data) - 32
	if assetLen <= 0 {
		return fmt.Errorf("invalid asset length")
	}
	asset := string(op.Data[32 : 32+assetLen])

	bal := balance.GetOrCreate(st, userID, asset)
	bal.Available = int64(available)
	bal.Locked = int64(locked)
	bal.Margin = int64(margin)

	return nil
}
