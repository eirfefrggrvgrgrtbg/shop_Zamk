---
paths:
  - "backend/internal/payouts/**"
  - "backend/internal/selleranalytics/**"
  - "backend/internal/inventory/**"
  - "backend/internal/supplies/**"
  - "backend/internal/fulfillment/**"
  - "backend/internal/returns/**"
  - "backend/internal/orders/**"
  - "backend/internal/payments/**"
  - "backend/internal/products/stock_forecast*.go"
---

# Финансы и склад (контракт FBO — в AGENTS.md; здесь только инварианты)

- Деньги — целые копейки (`int64`); без float, округление явное.
- Записи леджера и финансовые события должны быть идемпотентны: повторный вызов не создаёт вторую запись.
- Физический возврат и финансовое удержание — разные сущности; не смешивать.
- `available = total − reserved`; не менять `total_stock` и `reserved_stock` в обход сервисов склада.
- Изоляция продавцов: каждый запрос фильтруется по `seller_id`, чужие данные недоступны.
- Существующие формулы (комиссия, удержания, return rate, дни покрытия, прогноз) не менять без одобрения владельца; изменение — отдельной задачей с тестами.
- Результаты мутаций логировать и учитывать в наблюдаемости только после коммита; не логировать PII и секреты.
