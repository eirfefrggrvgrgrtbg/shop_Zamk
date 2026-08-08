# PROJECT_CONTEXT

- T-Pay mock shell принят;
- production credentials отсутствуют;
- полноценный Checkout browser E2E отложен;
- текущий acceptance использовал подготовленную session/cart;
- frontend validation Checkout будет отдельно проверена при финальной полировке Shop.
- Webhook с неверной подписью получает HTTP 400 без OK и не изменяет финансовые данные. Так как T-Bank считает успешным только ответ OK, повторные доставки такого уведомления возможны и должны контролироваться логированием/rate limiting.
