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

- Только для STOP и TP/SL и для ликвидации (внутренний, создается с origin = SYSTEM)
- При срабатывании закрывает позицию
- Может иметь qty = 0 (означает при срабатывании закрыть весь обьем)

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
| STOP   | -         | ДА           | НЕТ (UNTR) | ✗          | ✓              |

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

### 12.2 Расчёт маржи

**Важно**: При увеличении плеча требуется МЕНЬШЕ маржи (больше кредитного плеча).

```go
// margin = (size * price) / leverage
margin := int64(qty) * int64(price) / int64(leverage)

// Пример: position 100 BTC @ 50000
// leverage 2x: margin = 100 * 50000 / 2 = 2,500,000 USDT
// leverage 10x: margin = 100 * 50000 / 10 = 500,000 USDT
// leverage 50x: margin = 100 * 50000 / 50 = 100,000 USDT
```

### 12.3 Позиции

```go
position.size > 0  // LONG
position.size < 0  // SHORT
position.size = 0  // Нет позиции (side = null = -1)

// realized PnL при частичном закрытии
if (Math.sign(current.size) !== Math.sign(deltaSize)) {
  realizedPnl = calculatePnl(current, closedSize, fillPrice);
}
```

### 12.4 EditLeverage - изменение плеча

```
1. Проверка: leverage должен быть 1-100
2. Расчёт: oldMargin = size * entryPrice / oldLeverage
3. Расчёт: newMargin = size * entryPrice / newLeverage
4. Корректировка баланса:
   - если newMargin > oldMargin: available -= (newMargin - oldMargin)
   - если newMargin < oldMargin: available += (oldMargin - newMargin)
5. Проверка на ликвидацию
6. Установка: position.leverage = newLeverage
```

### 12.5 Ответы EditLeverage

| Сценарий | HTTP код | Ошибка |
|----------|----------|--------|
| Успех | 200 | - |
| Недостаточно баланса | 400 | "insufficient balance for new margin requirement" |
| Будет ликвидация | 400 | "position would be liquidated with new leverage" |
| Невалидное плечо | 400 | "leverage must be between 1 and 100" |

---

## 13. Примеры

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
