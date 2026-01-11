package triggers

import (
	"sync"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type TriggerNode struct {
	orderID   types.OrderID
	price     types.Price
	side      int8
	timestamp uint64
	index     int
	level     *priceLevel
}

var nodePool = sync.Pool{
	New: func() interface{} { return &TriggerNode{} },
}

var levelPool = sync.Pool{
	New: func() interface{} { return &priceLevel{} },
}

type priceLevel struct {
	price    types.Price
	orders   []*TriggerNode
	priority uint32
	left     *priceLevel
	right    *priceLevel
	parent   *priceLevel
	prev     *priceLevel
	next     *priceLevel
}

type priceTree struct {
	root   *priceLevel
	levels map[types.Price]*priceLevel
	min    *priceLevel
	max    *priceLevel
}

type Monitor struct {
	mu           sync.Mutex
	buyTree      priceTree
	sellTree     priceTree
	orders       map[types.OrderID]*types.Order
	nodes        map[types.OrderID]*TriggerNode
	prioritySeed uint32
}

func New() *Monitor {
	return NewWithCapacity(0)
}

func NewWithCapacity(capacity int) *Monitor {
	if capacity < 0 {
		capacity = 0
	}
	return &Monitor{
		buyTree:  priceTree{levels: make(map[types.Price]*priceLevel, capacity)},
		sellTree: priceTree{levels: make(map[types.Price]*priceLevel, capacity)},
		orders:   make(map[types.OrderID]*types.Order, capacity),
		nodes:    make(map[types.OrderID]*TriggerNode, capacity),
	}
}

func (m *Monitor) Add(order *types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	triggerSide := effectiveTriggerSide(order)
	node := nodePool.Get().(*TriggerNode)
	*node = TriggerNode{
		orderID:   order.ID,
		price:     order.TriggerPrice,
		side:      triggerSide,
		timestamp: order.CreatedAt,
		index:     -1,
		level:     nil,
	}

	m.orders[order.ID] = order
	m.nodes[order.ID] = node

	if triggerSide == constants.ORDER_SIDE_BUY {
		m.addToTree(&m.buyTree, node)
	} else {
		m.addToTree(&m.sellTree, node)
	}
}

func (m *Monitor) Remove(orderID types.OrderID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	node := m.nodes[orderID]
	if node == nil {
		return false
	}

	delete(m.nodes, orderID)
	delete(m.orders, orderID)
	if node.side == constants.ORDER_SIDE_BUY {
		m.removeFromTree(&m.buyTree, node)
	} else {
		m.removeFromTree(&m.sellTree, node)
	}
	*node = TriggerNode{}
	nodePool.Put(node)
	return true
}

func (m *Monitor) Check(currentPrice types.Price) []*types.Order {
	return m.CheckInto(currentPrice, nil)
}

func (m *Monitor) CheckInto(currentPrice types.Price, out []*types.Order) []*types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()

	triggered := out
	if triggered != nil {
		triggered = triggered[:0]
	}

	triggered = m.collectTriggered(&m.buyTree, currentPrice, true, triggered)
	triggered = m.collectTriggered(&m.sellTree, currentPrice, false, triggered)

	return triggered
}

func (m *Monitor) GetOrder(orderID types.OrderID) *types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders[orderID]
}

func (m *Monitor) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.orders)
}

func (m *Monitor) BuyCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.buyTree.levels)
}

func (m *Monitor) SellCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sellTree.levels)
}

func (m *Monitor) collectTriggered(tree *priceTree, currentPrice types.Price, isBuy bool, out []*types.Order) []*types.Order {
	var level *priceLevel
	if isBuy {
		level = tree.max
	} else {
		level = tree.min
	}

	for level != nil {
		if isBuy {
			if level.price < currentPrice {
				break
			}
		} else {
			if level.price > currentPrice {
				break
			}
		}
		next := level.prev
		if !isBuy {
			next = level.next
		}

		for i := range level.orders {
			node := level.orders[i]
			if node == nil {
				continue
			}
			if order := m.orders[node.orderID]; order != nil {
				out = append(out, order)
			}
			delete(m.orders, node.orderID)
			delete(m.nodes, node.orderID)
			*node = TriggerNode{}
			nodePool.Put(node)
			level.orders[i] = nil
		}
		level.orders = level.orders[:0]
		m.deleteLevel(tree, level)
		level = next
	}
	return out
}

func (m *Monitor) addToTree(tree *priceTree, node *TriggerNode) {
	level := tree.levels[node.price]
	if level == nil {
		levelAny := levelPool.Get()
		level, _ = levelAny.(*priceLevel)
		if level == nil {
			level = &priceLevel{}
		}
		orders := level.orders
		if cap(orders) == 0 {
			orders = make([]*TriggerNode, 0, 4)
		} else {
			orders = orders[:0]
		}
		*level = priceLevel{price: node.price}
		level.orders = orders
		level.priority = m.nextPriority()
		tree.levels[node.price] = level
		m.insertLevel(tree, level)
	}
	node.level = level
	node.index = len(level.orders)
	level.orders = append(level.orders, node)
}

func (m *Monitor) removeFromTree(tree *priceTree, node *TriggerNode) {
	level := node.level
	if level == nil {
		return
	}
	idx := node.index
	last := len(level.orders) - 1
	if idx >= 0 && idx <= last {
		level.orders[idx] = level.orders[last]
		level.orders[idx].index = idx
		level.orders[last] = nil
		level.orders = level.orders[:last]
	}
	if len(level.orders) == 0 {
		m.deleteLevel(tree, level)
	}
	node.level = nil
	node.index = -1
}

func (m *Monitor) insertLevel(tree *priceTree, level *priceLevel) {
	var parent *priceLevel
	cur := tree.root
	var prev *priceLevel
	var next *priceLevel

	for cur != nil {
		parent = cur
		if level.price < cur.price {
			next = cur
			cur = cur.left
		} else {
			prev = cur
			cur = cur.right
		}
	}

	level.parent = parent
	level.prev = prev
	level.next = next
	if prev != nil {
		prev.next = level
	} else {
		tree.min = level
	}
	if next != nil {
		next.prev = level
	} else {
		tree.max = level
	}

	if parent == nil {
		tree.root = level
		return
	}
	if level.price < parent.price {
		parent.left = level
	} else {
		parent.right = level
	}
	m.bubbleUp(tree, level)
}

func (m *Monitor) deleteLevel(tree *priceTree, level *priceLevel) {
	if level == nil {
		return
	}
	delete(tree.levels, level.price)
	if level.prev != nil {
		level.prev.next = level.next
	} else {
		tree.min = level.next
	}
	if level.next != nil {
		level.next.prev = level.prev
	} else {
		tree.max = level.prev
	}

	m.bubbleDown(tree, level)
	parent := level.parent
	if parent == nil {
		tree.root = nil
	} else if parent.left == level {
		parent.left = nil
	} else {
		parent.right = nil
	}
	level.parent = nil
	level.left = nil
	level.right = nil
	level.prev = nil
	level.next = nil
	if cap(level.orders) > 0 {
		level.orders = level.orders[:0]
	}
	level.priority = 0
	level.price = 0
	levelPool.Put(level)
}

func (m *Monitor) bubbleUp(tree *priceTree, node *priceLevel) {
	for node.parent != nil && node.parent.priority > node.priority {
		if node.parent.left == node {
			m.rotateRight(tree, node.parent)
		} else {
			m.rotateLeft(tree, node.parent)
		}
	}
	if node.parent == nil {
		tree.root = node
	}
}

func (m *Monitor) bubbleDown(tree *priceTree, node *priceLevel) {
	for node.left != nil || node.right != nil {
		if node.left == nil {
			m.rotateLeft(tree, node)
		} else if node.right == nil {
			m.rotateRight(tree, node)
		} else if node.left.priority < node.right.priority {
			m.rotateRight(tree, node)
		} else {
			m.rotateLeft(tree, node)
		}
		if node.parent == nil {
			tree.root = node
		}
	}
}

func (m *Monitor) rotateLeft(tree *priceTree, node *priceLevel) {
	right := node.right
	if right == nil {
		return
	}
	node.right = right.left
	if right.left != nil {
		right.left.parent = node
	}
	right.left = node
	right.parent = node.parent
	if node.parent == nil {
		tree.root = right
	} else if node.parent.left == node {
		node.parent.left = right
	} else {
		node.parent.right = right
	}
	node.parent = right
}

func (m *Monitor) rotateRight(tree *priceTree, node *priceLevel) {
	left := node.left
	if left == nil {
		return
	}
	node.left = left.right
	if left.right != nil {
		left.right.parent = node
	}
	left.right = node
	left.parent = node.parent
	if node.parent == nil {
		tree.root = left
	} else if node.parent.left == node {
		node.parent.left = left
	} else {
		node.parent.right = left
	}
	node.parent = left
}

func (m *Monitor) nextPriority() uint32 {
	m.prioritySeed = m.prioritySeed*1664525 + 1013904223
	return m.prioritySeed
}

func effectiveTriggerSide(order *types.Order) int8 {
	if !order.CloseOnTrigger {
		return order.Side
	}
	switch order.StopOrderType {
	case constants.STOP_ORDER_TYPE_STOP, constants.STOP_ORDER_TYPE_STOP_LOSS:
		return flipSide(order.Side)
	default:
		return order.Side
	}
}

func flipSide(side int8) int8 {
	if side == constants.ORDER_SIDE_BUY {
		return constants.ORDER_SIDE_SELL
	}
	return constants.ORDER_SIDE_BUY
}
