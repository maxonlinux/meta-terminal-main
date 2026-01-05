package orderbook

import (
	"github.com/anomalyco/meta-terminal-go/internal/types"
)

type TreeNode struct {
	Price    types.Price
	Quantity types.Quantity
	Orders   map[types.OrderID]*types.Order
	Left     *TreeNode
	Right    *TreeNode
	Height   int
}

type PriceTree struct {
	root  *TreeNode
	isBid bool
}

func NewPriceTree(isBid bool) *PriceTree {
	return &PriceTree{
		isBid: isBid,
	}
}

func (t *PriceTree) Insert(price types.Price, quantity types.Quantity, order *types.Order) {
	t.root = t.insert(t.root, price, quantity, order)
}

func (t *PriceTree) insert(node *TreeNode, price types.Price, quantity types.Quantity, order *types.Order) *TreeNode {
	if node == nil {
		return &TreeNode{
			Price:    price,
			Quantity: quantity,
			Orders:   map[types.OrderID]*types.Order{order.ID: order},
			Height:   1,
		}
	}

	if price == node.Price {
		node.Quantity += quantity
		node.Orders[order.ID] = order
		return node
	}

	if t.less(price, node.Price) {
		node.Left = t.insert(node.Left, price, quantity, order)
	} else {
		node.Right = t.insert(node.Right, price, quantity, order)
	}

	return t.balance(node)
}

func (t *PriceTree) Remove(price types.Price, quantity types.Quantity) types.Quantity {
	var removed types.Quantity
	t.root, removed = t.remove(t.root, price, quantity)
	return removed
}

func (t *PriceTree) remove(node *TreeNode, price types.Price, quantity types.Quantity) (*TreeNode, types.Quantity) {
	if node == nil {
		return nil, 0
	}

	if price == node.Price {
		if quantity >= node.Quantity {
			return t.deleteNode(node), node.Quantity
		}
		node.Quantity -= quantity
		return node, quantity
	}

	if t.less(price, node.Price) {
		node.Left, _ = t.remove(node.Left, price, quantity)
	} else {
		node.Right, _ = t.remove(node.Right, price, quantity)
	}

	return t.balance(node), 0
}

func (t *PriceTree) deleteNode(node *TreeNode) *TreeNode {
	if node.Left == nil {
		return node.Right
	}
	if node.Right == nil {
		return node.Left
	}

	successor := t.minNode(node.Right)
	node.Price = successor.Price
	node.Quantity = successor.Quantity
	node.Orders = successor.Orders
	node.Right, _ = t.remove(node.Right, successor.Price, 0)

	return t.balance(node)
}

func (t *PriceTree) minNode(node *TreeNode) *TreeNode {
	current := node
	for current.Left != nil {
		current = current.Left
	}
	return current
}

func (t *PriceTree) GetBest() *TreeNode {
	if t.root == nil {
		return nil
	}
	if t.isBid {
		return t.leftmostNode(t.root)
	}
	return t.leftmostNode(t.root)
}

func (t *PriceTree) leftmostNode(node *TreeNode) *TreeNode {
	current := node
	for current.Left != nil {
		current = current.Left
	}
	return current
}

func (t *PriceTree) Find(price types.Price) *TreeNode {
	return t.find(t.root, price)
}

func (t *PriceTree) find(node *TreeNode, price types.Price) *TreeNode {
	if node == nil {
		return nil
	}
	if price == node.Price {
		return node
	}
	if t.less(price, node.Price) {
		return t.find(node.Left, price)
	}
	return t.find(node.Right, price)
}

func (t *PriceTree) balance(node *TreeNode) *TreeNode {
	node.Height = 1 + max(t.height(node.Left), t.height(node.Right))
	balance := t.balanceFactor(node)

	if balance > 1 {
		if t.balanceFactor(node.Left) >= 0 {
			return t.rotateRight(node)
		}
		node.Left = t.rotateLeft(node.Left)
		return t.rotateRight(node)
	}

	if balance < -1 {
		if t.balanceFactor(node.Right) <= 0 {
			return t.rotateLeft(node)
		}
		node.Right = t.rotateRight(node.Right)
		return t.rotateLeft(node)
	}

	return node
}

func (t *PriceTree) rotateRight(y *TreeNode) *TreeNode {
	x := y.Left
	y.Left = x.Right
	x.Right = y
	y.Height = 1 + max(t.height(y.Left), t.height(y.Right))
	x.Height = 1 + max(t.height(x.Left), t.height(x.Right))
	return x
}

func (t *PriceTree) rotateLeft(x *TreeNode) *TreeNode {
	y := x.Right
	x.Right = y.Left
	y.Left = x
	x.Height = 1 + max(t.height(x.Left), t.height(x.Right))
	y.Height = 1 + max(t.height(y.Left), t.height(y.Right))
	return y
}

func (t *PriceTree) balanceFactor(node *TreeNode) int {
	if node == nil {
		return 0
	}
	return t.height(node.Left) - t.height(node.Right)
}

func (t *PriceTree) height(node *TreeNode) int {
	if node == nil {
		return 0
	}
	return node.Height
}

func (t *PriceTree) less(a, b types.Price) bool {
	if t.isBid {
		return a > b
	}
	return a < b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
