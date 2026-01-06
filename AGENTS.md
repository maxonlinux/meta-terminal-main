# Trading Engine - Zero-Allocation Platform for $10 Server

Very useful information here you MUST read: https://ramendraparmar.substack.com/p/system-design-question-10-stock-trading

## Linter
golangci-lint run

## MCP
you can use Context7 MCP server as much as you need
also, you have github MCP for code snippets and best practices

## Overview

High-performance trading engine optimized for minimal resource usage:
- **Target: $10/month server** (2 vCPU, 4GB RAM, 50GB SSD)
- **1000+ markets** (SPOT + LINEAR)
- **Zero-allocation design** - no GC pressure in hot paths
- **Atomic consistency** - no partial updates on crash
- **WAL + Snapshot architecture** - durability with minimal overhead

## Ключевые принципы

1. **ВСЕ ордера требуют counterparty** - для исполнения ордера в книге должен быть встречный ордер. Без counterparty ордер не исполнится (будет canceled для IOC/FOK или останется в книге для GTC/POST_ONLY).

2. **Conditional (STOP) ордера** - Превращаются в новые ордера. Существующий ордер меняет статус UNTRIGGERED → TRIGGERED и создается клон этого ордера с triggerPrice = null.

3. **Lock/Unlock** - только для RESTING ордеров (LIMIT, не исполнившиеся сразу), только остаток (qty - filled).

4. **LINEAR маржа** - (price\*qty) / leverage

5. **FOK** - частичное исполнение НЕВОЗМОЖНО. Если нет полного объёма - полная отмена без исполнения.

---

## 1. Константы

```go
// Категории
CATEGORY_SPOT = 0
CATEGORY_LINEAR = 1

// Типы ордеров
ORDER_TYPE_LIMIT = 0
ORDER_TYPE_MARKET = 1

// Стороны
ORDER_SIDE_BUY = 0
ORDER_SIDE_SELL = 1

// TIF
TIF_GTC = 0        // Good Till Cancel
TIF_IOC = 1        // Immediate Or Cancel
TIF_FOK = 2        // Fill Or Kill
TIF_POST_ONLY = 3  // Только создание

// Статусы
ORDER_STATUS_NEW = 0
ORDER_STATUS_PARTIALLY_FILLED = 1
ORDER_STATUS_FILLED = 2
ORDER_STATUS_CANCELED = 3
ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
ORDER_STATUS_UNTRIGGERED = 5      // STOP ждёт триггера
ORDER_STATUS_TRIGGERED = 6        // STOP сработал
ORDER_STATUS_DEACTIVATED = 7

// Bucket
BUCKET_AVAILABLE = 0
BUCKET_LOCKED = 1
BUCKET_MARGIN = 2
```

---

## 2. Категории рынков

### 2.1 SPOT (CATEGORY_SPOT = 0)

- Нет позиций
- Нет reduceOnly
- Есть closeOnTrigger для STOP
- Переводы между пользователями (реальные активы)
- Балансы: AVAILABLE, LOCKED

### 2.2 LINEAR (CATEGORY_LINEAR = 1)

- Есть позиции (LONG/SHORT)
- Есть reduceOnly
- Есть closeOnTrigger
- Есть маржа и плечо
- Балансы: AVAILABLE, LOCKED, MARGIN

---

## 3. Типы ордеров

### 3.1 LIMIT (ORDER_TYPE_LIMIT = 0)

- Требует price
- Блокирует баланс ТОЛЬКО если ушёл в RESTING
- Остаток в книге

### 3.2 MARKET (ORDER_TYPE_MARKET = 1)

- price = null
- Нет блокировки
- Исполняется немедленно

---

## 4. TIF - Время действия

### 4.1 GTC (TIF_GTC = 0) - Good Till Cancel

- Лежит в книге до отмены или исполнения
- Частичное исполнение допускается

### 4.2 IOC (TIF_IOC = 1) - Immediate Or Cancel

- Исполняется немедленно
- Частичное допускается (остаток отменяется) -> статус PARTIALLY_FILLED_CANCELED
- НЕ попадает в книгу

### 4.3 FOK (TIF_FOK = 2) - Fill Or Kill

- **Должен исполниться ПОЛНОСТЬЮ или НЕ исполниться ВООБЩЕ**
- Частичное исполнение НЕВОЗМОЖНО
- Если нет полного объёма → полная отмена БЕЗ исполнения
- НЕ попадает в книгу

### 4.4 POST_ONLY (TIF_POST_ONLY = 3)

- Создаётся только если НЕ исполняется сразу
- Если пересекает спред → отклоняем
- Попадает в книгу только если НЕ исполняется

---

## 5. Статусы ордеров

- Любой новый ордер который попал в книгу - NEW
- Любой ордер с частичным исполнением (кроме FOK) - PARTIALLY_FILLED
- Если исполнен полностью - FILLED
- Если отменен - CANCELED
- Если отменен но частично заполнен - PARTIALLY_FILLED_CANCELED

| Статус                    | Описание                  | Конечный? |
| ------------------------- | ------------------------- | --------- |
| NEW                       | В книге, ждёт контрагента | Нет       |
| PARTIALLY_FILLED          | Частично исполнен         | Нет       |
| FILLED                    | Полностью исполнен        | Да        |
| CANCELED                  | Отменён                   | Да        |
| PARTIALLY_FILLED_CANCELED | Частично + отмена остатка | Да        |
| UNTRIGGERED               | STOP ждёт triggerPrice    | Нет       |
| TRIGGERED                 | STOP сработал             | Нет       |
| DEACTIVATED               | Деактивирован             | Да        |

**REJECTED НЕ существует как статус!**

---

## 6. Флаги

### 6.1 reduceOnly

- SPOT: НЕ поддерживается (REJECT)
- LINEAR: Только уменьшает позицию
- Проверяется при создании: `qty <= |position.size|`
- При каждом уменьшении размера позиции ВСЕ ордера которые относятся к этой позиции должны быть обрезаны или отменены, чтобы их сумма не превышала размер позиции. Это должно происходить атомарно и автоматически

### 6.2 closeOnTrigger

- **Только для LINEAR** (SPOT: игнорируется, нет позиций)
- Для STOP и TP/SL и для ликвидации (внутренний, создается с origin = SYSTEM)
- При срабатывании закрывает позицию через MARKET IOC ордер (требует counterparty!)
- Может иметь qty = 0 (означает при срабатывании закрыть весь объём)

### 6.3 stopOrderType

- 0 = обычный ордер
- 4 = STOP
- 5 = TP
- 6 = SL
- 7 = LIQUIDATION
- Другие = специализированные стопы

---

## 8. Матрица комбинаций

### 8.1 SPOT

| Тип    | TIF       | Lock         | Книга      | reduceOnly | closeOnTrigger |
| ------ | --------- | ------------ | ---------- | ---------- | -------------- |
| LIMIT  | GTC       | ДА (остаток) | ДА         | ✗          | ✗              |
| LIMIT  | IOC       | НЕТ          | НЕТ        | ✗          | ✗              |
| LIMIT  | FOK       | НЕТ          | НЕТ        | ✗          | ✗              |
| LIMIT  | POST_ONLY | ДА           | ДА/REJECT  | ✗          | ✗              |
| MARKET | -         | НЕТ          | НЕТ        | ✗          | ✗              |
| STOP   | -         | ДА           | НЕТ (UNTR) | ✗          | ✗              |

### 8.2 LINEAR

| Тип    | TIF       | Lock | Книга      | reduceOnly | closeOnTrigger | Margin |
| ------ | --------- | ---- | ---------- | ---------- | -------------- | ------ |
| LIMIT  | GTC       | ДА   | ДА         | ✓          | ✗              | 1%     |
| LIMIT  | IOC       | НЕТ  | НЕТ        | ✗          | ✗              | 1%     |
| LIMIT  | FOK       | НЕТ  | НЕТ        | ✗          | ✗              | 1%     |
| LIMIT  | POST_ONLY | ДА   | ДА         | ✗          | ✗              | 1%     |
| MARKET | -         | НЕТ  | НЕТ        | ✗          | ✗              | 1%     |
| STOP   | -         | ДА   | НЕТ (UNTR) | ✓          | ✓              | 1%     |

---

## 9. Валидация

### 9.1 Pre-Trade (ДО создания)

- валидация ввода по схеме
- SPOT: closeOnTrigger игнорируется (нет позиций)
- LINEAR: reduceOnly требует существующую позицию

**Отклонённые ордера НЕ создаются, НЕ сохраняются в память!**

---

## 10. Жизненный цикл

### 10.1 LIMIT + GTC (SPOT)

```
placeOrder(input)
  ↓
validateOrder() → ok или REJECT (ордер НЕ создан!)
  ↓
createOrder() → orderIdx
  ↓
matchOrder()
  ↓
Если fills.length === 0:
  → lockBalance(remaining)
  → status = NEW (RESTING)
  → в книгу
  ↓
Ожидание контрагента
  ↓
При исполнении:
  → unlockBalance(filled)
  → переводы SPOT
  → status = FILLED/PARTIAL
```

### 10.2 MARKET

```
placeOrder(input)
  ↓
validateOrder()
  ↓
createOrder()
  ↓
matchOrder() ← немедленно
  ↓
Исполнение
  ↓
переводы SPOT / позиции LINEAR
  ↓
status = FILLED/PARTIAL
```

### 10.3 IOC

```
matchIoc():
  fills, remaining = book.match(orderIdx)

  if (fills.length === 0) {
    cancelOrder()
    status = CANCELLED
  } else if (remaining > 0) {
    cancelOrder()  // Отменяем остаток
    status = PARTIAL  // Частичное допускается!
  } else {
    status = FILLED
  }
```

### 10.4 FOK (КРИТИЧНО!)

```
matchFok():
  fills remaining = book.match(orderIdx)  // Может частично исполниться!

  if (remaining > 0) {
    // ЧАСТИЧНОЕ ИСПОЛНЕНИЕ НЕ ДОПУСКАЕТСЯ!
    // ОТМЕНЯЕМ ВСЁ, включая fills!
    cancelOrder()
// fills ТЕРЯЮТСЯ!
    status = CANCELLED
  } else {
    // Полное исполнение
    status = FILLED
  }
```

### 10.5 POST_ONLY

```
matchPostOnly():
  wouldCross = проверка на пересечение спреда

  if (wouldCross) {
    cancelOrder()
// Не создан в памяти!
  } else {
    book.addOrder()
    status = NEW (RESTING)
  }
```

### 10.6 STOP (Conditional)

```
placeOrder(input с triggerPrice):
  createOrder() → orderIdx
  setStatus(UNTRIGGERED)
  triggerMonitor.addOrder(orderIdx)  // В heap

onPriceChange(symbol, price):
  triggered = triggerMonitor.check(symbolIdx, price)
  triggerOrders(triggered, price)

triggerOrders():
  for (orderIdx in triggered) {
    setStatus(TRIGGERED)
    onOrderTriggered(orderIdx)
  }

onOrderTriggered(orderIdx):
  order = getOrder(orderIdx)

  if (order.closeOnTrigger) {
    closePosition(userIdx, symbolIdx)
    cancelOrder(orderIdx)  // Удаляем stop-ордер
  } else {
    // создаём новый ордер!
  }
```

---

## 11. Conditional (STOP) ордера

### 11.1 Ключевые моменты

1. STOP ордер создаёт новый ордер
2. Существующий ордер меняет статус UNTRIGGERED → TRIGGERED

### 11.2 TriggerMonitor

```typescript
class TriggerMonitor {
  // BUY стопы: min-heap, срабатывают когда цена поднимается ВЫШЕ
  private buyHeap: Int32Array;

  // SELL стопы: max-heap, срабатывают когда цена падает НИЖЕ
  private sellHeap: Int32Array;

  check(symbolIdx, currentPrice) {
    const triggered = [];

    // SELL: цена <= triggerPrice
    while (sellHeap[0] satisfies condition) {
      triggered.push(heapPop(sellHeap));
    }

    // BUY: цена >= triggerPrice
    while (buyHeap[0] satisfies condition) {
      triggered.push(heapPop(buyHeap));
    }

    return triggered;
  }
}
```

---

## 12. LINEAR и маржа

### 12.0 Правило позиций (КРИТИЧНО!)

**Position.size ВСЕГДА положительный!** Side определяет направление:

```go
// ПРАВИЛЬНО:
position.Size = 10   // всегда положительный
position.Side = 0    // 0 = LONG, 1 = SHORT

position.Size = 5    // размер позиции
position.Side = 1    // SHORT

// НЕПРАВИЛЬНО:
position.Size = -10  // ЗАПРЕЩЕНО!
position.Side = 0
```

### 12.1 Плечо (привязано к позиции, не к пользователю!)

- **По умолчанию**: leverage = 2 для новой позиции
- **При изменении**: устанавливается в позицию, пересчитывается маржа
- **Пустая позиция**: size = 0, side = null (-1), leverage = установленное значение
- **При закрытии**: size = 0, side = null (-1), leverage = сохраняется

```go
position.leverage = 2  // По умолчанию при создании
position.leverage = 10 // После изменения через EditLeverage
```

### 12.2 Поля позиции (хранятся в базе)

```
Position {
    UserID, Symbol, Size, Side, EntryPrice, Leverage
    
    // Риск-параметры (пересчитываются на каждый fill)
    InitialMargin    // IM = size * entryPrice / leverage
    MaintenanceMargin // MM = IM / 10
    LiquidationPrice // Цена ликвидации
    
    RealizedPnl      // Накопленный realized PnL
    Version          // Для optimistic locking
}
```

### 12.3 Расчёт рисков

```go
func CalculatePositionRisk(pos *Position) {
    if pos.Size == 0 {
        pos.InitialMargin = 0
        pos.MaintenanceMargin = 0
        pos.LiquidationPrice = 0
        return
    }
    
    pos.InitialMargin = pos.Size * pos.EntryPrice / pos.Leverage
    pos.MaintenanceMargin = pos.InitialMargin / 10
    
    // Ликвидация происходит когда |UnrealizedPnL| > (IM - MM)
    // Рассчитываем цену ликвидации для отображения пользователю
    buffer := pos.InitialMargin - pos.MaintenanceMargin
    
    if pos.Side == LONG {
        // LONG ликвидируется когда цена падает
        // PnL = (price - entryPrice) * size
        // Ликвидация когда PnL < -buffer
        // price - entryPrice < -buffer / size
        // price < entryPrice - buffer / size
        pos.LiquidationPrice = pos.EntryPrice - buffer / pos.Size
    } else {
        // SHORT ликвидируется когда цена растёт
        // PnL = (entryPrice - price) * size
        // Ликвидация когда PnL < -buffer
        // entryPrice - price < -buffer / size
        // price > entryPrice + buffer / size
        pos.LiquidationPrice = pos.EntryPrice + buffer / pos.Size
    }
}
```

**Вызывается:**
- При создании новой позиции
- При каждом fill (увеличение/уменьшение позиции)
- При изменении leverage

### 12.4 UpdatePosition - обновление позиции

Вызывается при каждом fill. Пересчитывает все параметры позиции.

```go
func UpdatePosition(s *state.State, userID, symbol, filledQty, price, side, leverage) (*Position, rpnl) {
    pos := getOrCreatePosition(userID, symbol, leverage)
    
    var realizedPnl int64
    
    if pos.Size == 0 {
        // Открытие позиции
        pos.Size = filledQty
        pos.Side = side
        pos.EntryPrice = price
    } else if pos.Side == side {
        // Увеличение позиции ( усреднение)
        newSize := pos.Size + filledQty
        pos.EntryPrice = (pos.EntryPrice*pos.Size + price*filledQty) / newSize
        pos.Size = newSize
    } else {
        // Уменьшение/переворот позиции
        if filledQty >= pos.Size {
            // Полное закрытие + переворот
            closedSize := pos.Size
            realizedPnl = calculateRpnl(pos, closedSize, price)
            pos.Size = filledQty - pos.Size
            pos.Side = side
            pos.EntryPrice = price
        } else {
            // Частичное закрытие
            realizedPnl = calculateRpnl(pos, filledQty, price)
            pos.Size -= filledQty
        }
    }
    
    // Пересчитываем риски
    CalculatePositionRisk(pos)
    
    if pos.Size == 0 {
        pos.Side = -1
        pos.EntryPrice = 0
        pos.InitialMargin = 0
        pos.MaintenanceMargin = 0
        pos.LiquidationPrice = 0
    }
    
    pos.Version++
    return pos, realizedPnl
}
```

```go
    // Применяем realized PnL к балансу (НЕ В ДОМЕНЕ ПОЗИЦИИ А В ДРУГОМ МЕСТЕ)
    if realizedPnl != 0 {
        bal := balance.GetOrCreate(s, userID, "USDT")
        bal.Available += realizedPnl
        pos.RealizedPnl += realizedPnl
    }
```

### 12.5 РPnL vs UPnL

**RPNL (Realized PnL)** - реализованная прибыль/убыток:
- Считается при уменьшении позиции
- **Сохраняется** в позиции (накопленный)
- **Применяется** к Available балансу немедленно
- Формула: `(entryPrice - closePrice) × closedQty`

**UPnL (Unrealized PnL)** - нереализованная прибыль/убыток:
- Считается динамически по текущей цене
- **НЕ сохраняется** в позиции
- **НЕ применяется** к балансу пока позиция открыта
- Формула: `(currentPrice - entryPrice) × size`

### 12.6 Проверка ликвидации

```go
func CheckLiquidation(pos *Position, currentPrice Price) bool {
    if pos.Size == 0 {
        return false
    }
    
    upnl := (currentPrice - pos.EntryPrice) * pos.Size
    buffer := pos.InitialMargin - pos.MaintenanceMargin
    
    return upnl < -buffer || upnl > buffer
}
```

**Вызывается:**
- На каждый тик цены (price feed)
- При изменении leverage (EditLeverage)

### 12.7 EditLeverage - изменение плеча

```
1. Проверка: leverage должен быть 1-100
2. Расчёт: oldIM = size * entryPrice / oldLeverage
3. Расчёт: newIM = size * entryPrice / newLeverage
4. Корректировка баланса:
   - если newIM > oldIM: available -= (newIM - oldIM)
   - если newIM < oldIM: available += (oldIM - newIM)
5. Проверка на ликвидацию при новом leverage
6. Установка: position.leverage = newLeverage
7. Пересчёт рисков: CalculatePositionRisk(pos)
```

### 13.1 SPOT: FOK отмена (fills теряются!)

```
Исходное:
  UserA: 100000 USDT
  UserB: 0.3 BTC

Действие: UserA → BUY 1 BTC @ 50000 FOK
  book.match() исполняет 0.3 @ 50000
  remaining = 0.7 > 0

  // FOK ЧАСТИЧНОЕ НЕ ДОПУСКАЕТСЯ!
  cancelOrder()
  fills = []  // fills ОТМЕНЯЮТСЯ!
  status = CANCELLED

Результат:
  UserA: 100000 USDT, 0 BTC (НИЧЕГО!)
  UserB: 0.3 BTC (НИЧЕГО не потерял!)
```

### 13.2 SPOT: IOC частичное (допускается)

```
Действие: UserA → BUY 1 BTC @ 50000 IOC
  book.match() исполняет 0.3 @ 50000
  remaining = 0.7 > 0

  // IOC ЧАСТИЧНОЕ ДОПУСКАЕТСЯ!
  cancelOrder()
  fills сохраняются
  status = PARTIAL

Результат:
  UserA: 85000 USDT, 0.3 BTC
  UserB: 98500 USDT, 0 BTC
```

### 13.3 STOP с closeOnTrigger

```
Позиция: LONG +1 BTC @ 50000

STOP SELL @ 49000 closeOnTrigger:
  При срабатывании:
  → closePosition() - размещает MARKET IOC ордер
  → Для исполнения НУЖЕН counterparty (BID в книге)!
  → realizedPnl = -1000 USDT
  → cancelOrder(stop)
  → Позиция = 0

STOP SELL @ 49000 (без closeOnTrigger):
  При срабатывании:
  → matchOrder(orderIdx) как SELL LIMIT
  → Создаётся ордер в книге
  → При исполнении: Position = 0
```

### 13.4 LINEAR: reduceOnly

```
Позиция: LONG +1 BTC @ 50000

SELL 0.5 BTC reduceOnly:
  ✓ Принят (0.5 <= 1)
  → Position = +0.5 BTC

SELL 1.5 BTC reduceOnly:
  ✗ REJECT (1.5 > 1)

BUY 0.5 BTC reduceOnly:
  ✗ REJECT (BUY увеличивает позицию)
```

---

## 14. Итоговые правила

**ВСЕ ордера требуют counterparty** - без встречного ордера в книге исполнение невозможно.

1. **REJECTED** - не статус. Ордер не создаётся.

2. **STOP** - превращается в новый ордер. Существующий меняет статус.

3. **Lock** - только для RESTING LIMIT, только остаток.

4. **LINEAR** - маржа, не полная стоимость. transferToMargin.

5. **FOK** - частичное НЕВОЗМОЖНО. Отмена = fills теряются.

6. **IOC** - частичное допускается. Отмена = fills сохраняются.

7. **POST_ONLY** - отклоняется если пересекает спред.

8. **reduceOnly** - только LINEAR, только уменьшает позицию.

9. **closeOnTrigger** - закрывает позицию через MARKET IOC ордер (требует counterparty).

СХЕМА ордера который приходит от клиента: {
userId: string;
qty: Decimal;
price: Decimal | null;
side: OrderSide;
type: OrderType;
tif: TIF;
triggerPrice: string | null;
closeOnTrigger: boolean;
stopOrderType: StopOrderType | null;
reduceOnly: boolean;
};

если есть triggerPrice = ордер Conditional
если есть closeOnTrigger = ордер CloseOnTrigger (обязательно имеет stopOrderType)
если есть reduceOnly = ордер ReduceOnly (только LINEAR)
если type LIMIT то обязательно должна быть указана цена price (для MARKET всегда игнорируем и приравниваем к null)

---

## 15. Балансы и Маржа

### 15.1 Структура баланса

```
UserBalance {
    UserID    UserID
    Asset     string    // "USDT", "BTC", etc.
    Available int64     // Свободные средства
    Locked    int64     // Заблокировано под активные LIMIT ордера
    Margin    int64     // Используется как маржа для позиций
    Version   int64     // Optimistic locking
}
```

### 15.2 LockForOrder - блокировка под ордер

При создании LIMIT ордера часть Available переводится в Locked (и Margin для LINEAR).

```go
func LockForOrder(s *state.State, category, userID, order, leverage) {
    if order.Type == MARKET {
        return  // MARKET не блокирует
    }
    
    toLock = (order.Qty - order.Filled) * order.Price
    
    if category == SPOT {
        bal.Locked += toLock
    } else {
        // IM = qty * price / leverage
        margin = toLock / leverage
        bal.Margin += margin
        bal.Locked += margin
    }
}
```

### 15.3 UnlockForOrder - разблокировка

При отмене/исполнении ордера Locked возвращается в Available.

```go
func UnlockForOrder(s *state.State, category, userID, order, leverage) {
    toUnlock = (order.Qty - order.Filled) * order.Price
    
    if category == SPOT {
        bal.Locked -= toUnlock
    } else {
        margin = toUnlock / leverage
        bal.Locked -= margin
    }
}
```

### 15.4 ExecuteLinearTrade - применение RPNL и маржи

При fill ордера вызывается UpdatePosition (пересчитывает риски) и применяется RPNL к Available.

```go
func ExecuteLinearTrade(s, taker, maker, price, qty, leverage) {
    margin = qty * price / leverage
    
    // UpdatePosition пересчитывает IM, MM, LiquidationPrice
    _, takerRpnl := position.UpdatePosition(s, taker.UserID, taker.Symbol, qty, price, taker.Side, leverage)
    _, makerRpnl := position.UpdatePosition(s, maker.UserID, maker.Symbol, qty, price, maker.Side, leverage)
    
    tBal := balance.GetOrCreate(s, taker.UserID, "USDT")
    if taker.Side == BUY {
        tBal.Margin += margin      // Открываем LONG - увеличиваем маржу
        tBal.Available += takerRpnl // Применяем RPNL
    } else {
        tBal.Margin -= margin      // Открываем SHORT - уменьшаем маржу
        tBal.Available += takerRpnl
    }
    
    // Аналогично для maker...
}
```

### 15.5 EditLeverage - изменение плеча

При изменении плеча пересчитывается IM и корректируется Available.

```go
func EditLeverage(userID, symbol, leverage) error {
    pos = getPosition(userID, symbol)
    
    oldIM = pos.InitialMargin
    pos.Leverage = leverage
    position.CalculatePositionRisk(pos)  // Пересчитывает IM, MM, LiquidationPrice
    newIM = pos.InitialMargin
    
    marginDiff = newIM - oldIM
    bal = getBalance(userID, "USDT")
    
    if marginDiff > 0 {
        bal.Available -= marginDiff  // Нужно больше маржи
    } else if marginDiff < 0 {
        bal.Available += -marginDiff // Освобождается маржа
    }
    
    bal.Margin = pos.InitialMargin
}
```

### 15.6 Единая формула маржи

**ВСЕГДА используется одна формула:**

```
IM = qty × price / leverage
MM = IM / 10
```

Используется в:
- `position.CalculatePositionRisk()` - для позиции
- `balance.LockForOrder()` - при создании ордера
- `balance.UnlockForOrder()` - при отмене ордера
- `trade.ExecuteLinearTrade()` - при fill ордера
- `engine.EditLeverage()` - при изменении плеча

---

## 16. 生产环境需要什么

### 16.1 缺失的功能

| 功能 | 优先级 | 说明 | 状态 |
|------|--------|------|------|
| **Price Feed** | 高 | 自动检查每个价格 tick 的清算条件 | ✅ 已实现 |
| **Liquidation Trigger** | 高 | 当条件满足时调用 `ClosePosition()` | ✅ 已实现 |
| **API Validation Schema** | 中 | 验证输入的订单参数 | 待实现 |
| **Multi-asset Support** | 中 | 目前 SPOT 总是 BTC/USDT | 待实现 |
| **Monitoring & Alerts** | 中 | 指标、日志、告警 | 待实现 |

### 16.2 Price Feed Integration (已实现)

```go
// internal/pricefeed/pricefeed.go
type PriceFeed struct {
    state *state.State
    eng   Engine

    prices map[types.SymbolID]types.Price
}

func (pf *PriceFeed) UpdatePrice(symbol types.SymbolID, price types.Price) {
    pf.prices[symbol] = price
    pf.checkLiquidation(symbol, price)
}

func (pf *PriceFeed) checkLiquidation(symbol types.SymbolID, currentPrice types.Price) {
    for userID, us := range pf.state.Users {
        pos := us.Positions[symbol]
        if pos == nil || pos.Size == 0 {
            continue
        }

        if pf.shouldLiquidate(pos, currentPrice) {
            pf.eng.ClosePosition(userID, symbol)
        }
    }
}
```

### 16.3 Multi-asset Support (待实现)

**当前问题**: 交易逻辑硬编码使用 "USDT" 和 "BTC"。

**需要修改的文件**:
1. `internal/trade/trade.go` - `ExecuteSpotTrade()` 和 `ExecuteLinearTrade()`
2. `internal/balance/balance.go` - `LockForOrder()`, `UnlockForOrder()`, `AdjustLocked()`
3. `internal/engine/engine.go` - `ClosePosition()`, `EditLeverage()`

**解决方案**:
1. 将 `SymbolRegistry` 添加到 `State` 结构
2. 修改 `ExecuteSpotTrade(s, taker, maker, price, qty, symbol)` 接受 symbol 参数
3. 修改 `ExecuteLinearTrade(s, taker, maker, price, qty, leverage, symbol)` 接受 symbol 参数
4. 通过 `symbol.QuoteAsset` 和 `symbol.BaseAsset` 获取正确的资产

```go
// 修改后的 ExecuteLinearTrade
func ExecuteLinearTrade(s *state.State, taker, maker *types.Order, price types.Price, qty types.Quantity, leverage int8, symbol *state.Symbol) {
    margin := position.CalculateMargin(qty, price, leverage)

    quoteAsset := symbol.QuoteAsset // "USDT", "BUSD", etc.

    tBal := balance.GetOrCreate(s, taker.UserID, quoteAsset)
    // ... 其余逻辑
}
```

### 16.4 监控指标建议

| 指标 | 说明 |
|------|------|
| `orders_per_second` | 订单处理速率 |
| `fills_per_second` | 成交处理速率 |
| `liquidation_count` | 清算次数 |
| `position_count` | 开放仓位数量 |
| `margin_utilization` | 保证金使用率 |
| `orderbook_depth` | 订单簿深度 |
| `latency_p50` | P50 延迟 |
| `latency_p99` | P99 延迟 |

### 16.4 下一步

1. **Price Feed Service** - 订阅交易所 WebSocket，推送价格到 engine
2. **Liquidation Worker** - 后台任务检查所有开放仓位的清算条件
3. **Schema Validation** - 添加 JSON Schema 验证订单参数
4. **Multi-asset** - 支持更多交易对
5. **Admin API** - 管理员操作接口

---

## 当前状态

**Engine 核心功能已完成:**
- ✅ 订单 (PlaceOrder, CancelOrder, AmendOrder)
- ✅ 成交 (Matching Engine)
- ✅ 仓位 (UpdatePosition, CalculatePositionRisk)
- ✅ 保证金 (IM, MM, Liquidation)
- ✅ 止盈止损 (STOP, TP, SL)
- ✅ 持久化 (WAL + Snapshot)
- ✅ HTTP API
- ✅ Price Feed Service (自动清算检查)
- ✅ Liquidation Trigger

**测试覆盖:**
- ✅ 单元测试: 92+
- ✅ 集成测试: 15
- ✅ 性能测试: 20+ benchmarks
- ✅ Price Feed 测试: 15

**性能:**
- PlaceOrder: ~1.4 μs, 2 allocs
- MatchOrder: ~1-4 μs, 2 allocs
- GetOrder: ~4 ns, 0 allocs
