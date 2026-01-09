# AGENTS.md Compliance Report

## ✅ ПОЛНОЕ СООТВЕТСТВИЕ

| Требование | Статус | Файл |
|------------|--------|------|
| godotenv конфигурация | ✅ | `cmd/all/main.go`, `cmd/gateway/main.go` |
| OMS Shard = 1 symbol | ✅ | `services/oms/service.go:53` |
| orderbooks[category] (SPOT=0, LINEAR=1) | ✅ | `services/oms/service.go:53` |
| OrderStore map[UserID]map[OrderID]*Order | ✅ | `services/oms/service.go:54` |
| Все константы | ✅ | `internal/constants/constants.go` |
| Object Pooling | ✅ | `internal/pool/pool.go` |
| TriggerMonitor (min/max heaps) | ✅ | `internal/triggers/monitor.go` |
| TIF логика (GTC, IOC, FOK, POST_ONLY) | ✅ | `services/oms/service.go` |
| Performance targets | ✅ | Все达标 |

---

## ❌ НЕ РЕАЛИЗОВАНО

| Компонент | Описание | Priority |
|-----------|----------|----------|
| Market Interface | `GetValidator()`, `GetClearing()`, `GetOrderBookState()` | High |
| Validator Interface | `Validate(input *OrderInput) error` | High |
| Clearing Interface | `Reserve()`, `Release()`, `ExecuteTrade()` | High |
| handleConditional() | Триггер → создание ордера без TriggerPrice | High |
| handleCloseOnTrigger() | Создание reduceOnly ордера | High |
| OnPriceTick() | Интеграция price tick → trigger check | High |
| OrderInput.TriggerPrice | Поле для условных ордеров | High |
| OrderInput.CloseOnTrigger | Поле для close-on-trigger | High |
| OrderInput.ReduceOnly | Поле для reduce-only | High |
| Registry | instruments, lastPrices | Medium |
| Symbol Registry HTTP Loader | Загрузка инструментов | Low |

---

## 🔴 КРИТИЧЕСКИЕ ПРОТИВОРЕЧИЯ (МОГУТ СЛОМАТЬ СИСТЕМУ)

### 1. Balance Flow НЕ ИНТЕГРИРОВАН

**AGENTS.md требует:**
```
PlaceOrder → Reserve() → AddOrder() → ExecuteTrade() → Release()
```

**Текущая реализация:**
```go
// services/oms/service.go - PlaceOrder НЕ вызывает Reserve/Release
// services/portfolio/service.go - НЕ вызывается из OMS
```

**Опасность:** 
- Ордера размещаются БЕЗ проверки баланса
- Балансы не резервируются
- Linear ордера без margin check

### 2. OrderInput НЕ СООТВЕТСТВУЕТ AGENTS.md

**Текущий OrderInput (services/oms/service.go:24-33):**
```go
type OrderInput struct {
    UserID   uint64
    Symbol   string
    Category int8
    Side     int8
    Type     int8
    TIF      int8
    Qty      int64
    Price    int64
}
```

**AGENTS.md требует:**
```go
type OrderInput struct {
    UserID          UserID
    Symbol          string
    Category        int8
    Side            int8
    Type            int8
    TIF             int8
    Quantity        Quantity
    Price           Price
    TriggerPrice    Price      // ❌ НЕТ
    ReduceOnly      bool       // ❌ НЕТ
    CloseOnTrigger  bool       // ❌ НЕТ
}
```

**Опасность:**
- Невозможно разместить conditional order (stop-loss, take-profit)
- Невозможно разместить reduce-only order
- СloseOnTrigger не работает

### 3. Order Status НЕ ИСПОЛЬЗУЕТСЯ ПОЛНОСТЬЮ

**Текущий код (services/oms/service.go:146):**
```go
status := s.getOrderStatus(order, matches)
```

**AGENTS.md требует статусов:**
- `ORDER_STATUS_UNTRIGGERED = 5` - для conditional orders
- `ORDER_STATUS_TRIGGERED = 6` - после срабатывания триггера

**Опасность:**
- Conditional orders не помечаются как UNTRIGGERED
- Невозможно отследить состояние conditional order

### 4. TriggerMonitor НЕ ИНТЕГРИРОВАН В OMS

**Есть (internal/triggers/monitor.go):**
```go
type Monitor struct {
    buyTriggers  *BuyHeap
    sellTriggers *SellHeap
    orders       map[types.OrderID]*types.Order
}
```

**Но НЕ используется в OMS PlaceOrder:**
```go
// services/oms/service.go - PlaceOrder
// НЕ проверяет TriggerPrice
// НЕ добавляет в TriggerMonitor
```

**Опасность:**
- TriggerMonitor есть, но бесполезен
- Conditional orders не работают

---

## ⚠️ ПОТЕНЦИАЛЬНО ОПАСНЫЕ ВАРИАНТЫ

### 1. MARKET ордера без IOC/FOK

```go
// services/oms/service.go:116-118
if input.Type == constants.ORDER_TYPE_LIMIT {
    limitPrice = types.Price(input.Price)
}
// НЕТ проверки: MARKET must be IOC/FOK
```

**Можно:** Разместить MARKET order с GTC (противоречит AGENTS.md)

### 2. POST_ONLY ордера могут мачтиться

```go
// services/oms/service.go:120-125
if input.TIF == constants.TIF_POST_ONLY {
    if ob.WouldCross(input.Side, limitPrice) {
        pool.PutOrder(order)
        return nil, nil  // Возвращает nil без ошибки!
    }
}
// Правильно: должен возвращать ошибку или специальный статус
```

**Можно:** POST_ONLY без ошибки выглядит как успешный ордер

### 3. SPOT + LINEAR в одном ордере

**AGENTS.md:**
```
SPOT: reject reduceOnly, closeOnTrigger, trigger orders
```

**Текущий код:** НЕТ валидации

**Можно:** Разместить reduceOnly SPOT order (противоречит бизнес-логике)

### 4. Ликвидации не проверяются

```go
// services/marketdata/marketdata.go:119-136
func (s *Service) publishPriceSnapshots(ctx context.Context) {
    // Только публикует snapshot
    // НЕ проверяет ликвидации
}
```

**Можно:** Позиции уйдут в минус без ликвидации

---

## РЕКОМЕНДАЦИИ (в порядке приоритета)

### P0 - Критические (должно быть сделано)

1. **Добавить поля в OrderInput:**
   - `TriggerPrice int64`
   - `CloseOnTrigger bool`  
   - `ReduceOnly bool`

2. **Интегрировать TriggerMonitor в OMS:**
   - PlaceOrder: если TriggerPrice > 0 → UNTRIGGERED → TriggerMonitor
   - OnPriceTick: вызов TriggerMonitor.Check()

3. **Интегрировать Portfolio:**
   - PlaceOrder → Reserve() для LIMIT/GTC/POST_ONLY
   - ExecuteTrade → Reserve → Deduct

### P1 - Важные

4. **Добавить валидацию:**
   - MARKET must be IOC/FOK
   - SPOT reject reduceOnly/closeOnTrigger
   - SPOT reject conditional orders

5. **handleConditional() и handleCloseOnTrigger()**

6. **Market/Validator/Clearing interfaces**

### P2 - Желательные

7. **Registry** (instruments, lastPrices)
8. **Symbol Registry HTTP Loader**
9. **Liquidation check**

---

## ВЫВОД

**Система РАБОТАЕТ для базовых операций:**
- ✅ PlaceOrder (без баланса)
- ✅ MatchOrder
- ✅ CancelOrder
- ✅ BestBidAsk

**НО НЕ СООТВЕТСТВУЕТ AGENTS.md для:**
- ❌ Conditional orders (stop-loss, take-profit)
- ❌ Balance flow (reserve/release)
- ❌ Linear markets (margin, positions)
- ❌ Liquidation
- ❌ Full order lifecycle

**Рекомендация:** Не использовать для production пока не будут исправлены критические противоречия.
