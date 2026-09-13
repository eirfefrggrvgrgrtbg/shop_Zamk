export type StaffCapabilityGroupKey =
  | 'staff'
  | 'system'
  | 'catalog'
  | 'warehouse'
  | 'orders'
  | 'returns'
  | 'finance'
  | 'sellers'
  | 'auctions'
  | 'support'
  | 'analytics';

export interface StaffCapabilityGroup {
  key: StaffCapabilityGroupKey;
  title: string;
  description?: string;
}

export interface StaffCapabilityDefinition {
  key: string;
  title: string;
  group: StaffCapabilityGroupKey;
  description?: string;
  critical?: boolean;
}

/**
 * Deterministic presentation order for capability groups in Admin UI.
 */
export const STAFF_CAPABILITY_GROUPS: StaffCapabilityGroup[] = [
  {
    key: 'staff',
    title: 'Сотрудники и роли',
    description: 'Управление доступом персонала и шаблонами ролей',
  },
  {
    key: 'system',
    title: 'Система и безопасность',
    description: 'Глобальные настройки, журнал аудита и параметры платформы',
  },
  {
    key: 'catalog',
    title: 'Каталог товаров',
    description: 'Модерация товаров, управление категориями и брендами',
  },
  {
    key: 'warehouse',
    title: 'Склад и логистика',
    description: 'Приёмка поставок, сборка, упаковка и складские остатки',
  },
  {
    key: 'orders',
    title: 'Заказы и отправления',
    description: 'Просмотр заказов покупателей и статусов доставки',
  },
  {
    key: 'returns',
    title: 'Возвраты и компенсации',
    description: 'Обработка клиентских возвратов и денежных компенсаций',
  },
  {
    key: 'finance',
    title: 'Финансы и выплаты',
    description: 'Платежи покупателей, комиссии и утверждение выплат продавцам',
  },
  {
    key: 'sellers',
    title: 'Продавцы',
    description: 'Онбординг, верификация, коммуникация и статус магазинов',
  },
  {
    key: 'auctions',
    title: 'Аукционы',
    description: 'Создание, проведение торгов и настройки аукционных лотов',
  },
  {
    key: 'support',
    title: 'Обращения и отзывы',
    description: 'Тикеты службы поддержки, рассмотрение жалоб и модерация отзывов',
  },
  {
    key: 'analytics',
    title: 'Аналитика и отчёты',
    description: 'Сводные дашборды, операционные отчёты и экспорт данных',
  },
];

/**
 * Canonical presentation configuration for all 86 ZAMK backend capabilities.
 */
export const STAFF_CAPABILITIES: StaffCapabilityDefinition[] = [
  // 1. Staff & Roles (7)
  {
    key: 'staff.read',
    title: 'Просмотр сотрудников',
    group: 'staff',
    description: 'Просмотр списка сотрудников и их базовой информации',
  },
  {
    key: 'staff.create',
    title: 'Создание сотрудников',
    group: 'staff',
    description: 'Создание новых учетных записей сотрудников и первичная выдача доступов',
  },
  {
    key: 'staff.update',
    title: 'Изменение сотрудников',
    group: 'staff',
    description: 'Изменение данных сотрудников, сброс паролей и смена шаблона роли',
  },
  {
    key: 'staff.block',
    title: 'Блокировка сотрудников',
    group: 'staff',
    description: 'Блокировка и архивирование учетных записей сотрудников',
  },
  {
    key: 'staff.permissions.manage',
    title: 'Управление правами сотрудников',
    group: 'staff',
    critical: true,
    description: 'Позволяет напрямую назначать и отзывать индивидуальные права сотрудников',
  },
  {
    key: 'roles.read',
    title: 'Просмотр шаблонов ролей',
    group: 'staff',
    description: 'Просмотр системных шаблонов ролей и входящих в них прав',
  },
  {
    key: 'roles.manage',
    title: 'Управление шаблонами ролей',
    group: 'staff',
    critical: true,
    description: 'Создание и редактирование шаблонов ролей',
  },

  // 2. System & Security (7)
  {
    key: 'users.read',
    title: 'Просмотр пользователей',
    group: 'system',
    description: 'Просмотр аккаунтов клиентов и общей базы пользователей',
  },
  {
    key: 'audit.read',
    title: 'Просмотр журнала аудита',
    group: 'system',
    description: 'Доступ к просмотру действий сотрудников и системных событий',
  },
  {
    key: 'security.read',
    title: 'Просмотр настроек безопасности',
    group: 'system',
    description: 'Просмотр событий безопасности и активных сессий',
  },
  {
    key: 'settings.read',
    title: 'Просмотр настроек системы',
    group: 'system',
    description: 'Просмотр общих системных параметров платформы',
  },
  {
    key: 'settings.manage',
    title: 'Управление настройками системы',
    group: 'system',
    critical: true,
    description: 'Изменение глобальных конфигураций и параметров платформы',
  },
  {
    key: 'storefront.manage',
    title: 'Управление витриной',
    group: 'system',
    description: 'Настройка баннеров, промо-блоков и главной страницы витрины',
  },
  {
    key: 'testing.manage',
    title: 'Управление тестовой лабораторией',
    group: 'system',
    critical: true,
    description: 'Генерация и очистка тестовых сценариев и данных',
  },

  // 3. Catalog (15)
  {
    key: 'products.read',
    title: 'Просмотр товаров',
    group: 'catalog',
    description: 'Просмотр товаров каталога и их характеристик',
  },
  {
    key: 'products.moderate',
    title: 'Модерация товаров',
    group: 'catalog',
    description: 'Взятие товаров на проверку модератором',
  },
  {
    key: 'products.approve',
    title: 'Одобрение товаров',
    group: 'catalog',
    description: 'Подтверждение прохождения товаром модерации',
  },
  {
    key: 'products.reject',
    title: 'Отклонение товаров',
    group: 'catalog',
    description: 'Отклонение товаров с указанием причины',
  },
  {
    key: 'products.publish',
    title: 'Публикация товаров',
    group: 'catalog',
    description: 'Публикация одобренных товаров в общий каталог',
  },
  {
    key: 'products.hide',
    title: 'Скрытие товаров',
    group: 'catalog',
    description: 'Скрытие товаров с витрины маркетплейса',
  },
  {
    key: 'products.block',
    title: 'Блокировка товаров',
    group: 'catalog',
    description: 'Блокировка товаров за нарушения правил',
  },
  {
    key: 'categories.read',
    title: 'Просмотр категорий',
    group: 'catalog',
    description: 'Просмотр дерева категорий каталога',
  },
  {
    key: 'categories.create',
    title: 'Создание категорий',
    group: 'catalog',
    description: 'Добавление новых категорий в каталог',
  },
  {
    key: 'categories.update',
    title: 'Изменение категорий',
    group: 'catalog',
    description: 'Редактирование категорий и размерных сеток',
  },
  {
    key: 'categories.delete',
    title: 'Удаление категорий',
    group: 'catalog',
    description: 'Удаление категорий из каталога',
  },
  {
    key: 'brands.read',
    title: 'Просмотр брендов',
    group: 'catalog',
    description: 'Просмотр справочника брендов',
  },
  {
    key: 'brands.create',
    title: 'Создание брендов',
    group: 'catalog',
    description: 'Добавление новых брендов',
  },
  {
    key: 'brands.update',
    title: 'Изменение брендов',
    group: 'catalog',
    description: 'Редактирование информации о брендах',
  },
  {
    key: 'brands.delete',
    title: 'Удаление брендов',
    group: 'catalog',
    description: 'Удаление брендов из справочника',
  },

  // 4. Warehouse & Inventory (10)
  {
    key: 'warehouse.receiving',
    title: 'Приёмка поставок',
    group: 'warehouse',
    description: 'Первичная приёмка поставок продавцов на складе',
  },
  {
    key: 'warehouse.picking',
    title: 'Сборка заказов',
    group: 'warehouse',
    description: 'Сборка единиц товаров по заказам',
  },
  {
    key: 'warehouse.packing',
    title: 'Упаковка заказов',
    group: 'warehouse',
    description: 'Контроль и упаковка собранных заказов',
  },
  {
    key: 'warehouse.dispatch',
    title: 'Передача в доставку',
    group: 'warehouse',
    description: 'Отгрузка упакованных отправлений курьерам и службам доставки',
  },
  {
    key: 'warehouse.returns',
    title: 'Складская обработка возвратов',
    group: 'warehouse',
    description: 'Физическая приёмка и инспекция возвращенных товаров',
  },
  {
    key: 'inventory.read',
    title: 'Просмотр остатков и ZMU',
    group: 'warehouse',
    description: 'Просмотр складских остатков и серийных номеров единиц товаров',
  },
  {
    key: 'inventory.receipt',
    title: 'Оприходование остатков',
    group: 'warehouse',
    description: 'Внесение единиц товаров на складской баланс',
  },
  {
    key: 'inventory.adjust',
    title: 'Корректировка остатков',
    group: 'warehouse',
    description: 'Инвентаризация и корректировка количества товаров',
  },
  {
    key: 'inventory.write_off',
    title: 'Списание товаров',
    group: 'warehouse',
    description: 'Списание поврежденных и утерянных складских единиц',
  },
  {
    key: 'inventory.movements.read',
    title: 'История складских перемещений',
    group: 'warehouse',
    description: 'Просмотр истории движения остатков и аудита перемещений',
  },

  // 5. Orders & Shipments (5)
  {
    key: 'orders.read',
    title: 'Просмотр заказов',
    group: 'orders',
    description: 'Просмотр списка и деталей заказов покупателей',
  },
  {
    key: 'orders.update_status',
    title: 'Изменение статусов заказов',
    group: 'orders',
    description: 'Ручное изменение состояний заказов',
  },
  {
    key: 'shipments.read',
    title: 'Просмотр отправлений',
    group: 'orders',
    description: 'Просмотр отправлений и трек-номеров',
  },
  {
    key: 'shipments.create',
    title: 'Создание отправлений',
    group: 'orders',
    description: 'Формирование посылок и назначение логистических связок',
  },
  {
    key: 'shipments.update_status',
    title: 'Изменение статусов отправлений',
    group: 'orders',
    description: 'Обновление этапов доставки отправлений',
  },

  // 6. Returns & Refunds (4)
  {
    key: 'returns.read',
    title: 'Просмотр возвратов',
    group: 'returns',
    description: 'Просмотр заявок покупателей на возврат товаров',
  },
  {
    key: 'returns.update_status',
    title: 'Изменение статусов возвратов',
    group: 'returns',
    description: 'Утверждение, отклонение и продвижение заявок возвратов',
  },
  {
    key: 'refunds.read',
    title: 'Просмотр возвратов средств',
    group: 'returns',
    description: 'Просмотр расчетов и истории возвратов денег',
  },
  {
    key: 'refunds.create',
    title: 'Создание возвратов средств',
    group: 'returns',
    description: 'Инициация денежного возврата покупателю',
  },

  // 7. Finance & Payouts (8)
  {
    key: 'payments.read',
    title: 'Просмотр платежей',
    group: 'finance',
    description: 'Просмотр реестра транзакций и статусов оплаты заказов',
  },
  {
    key: 'payouts.read',
    title: 'Просмотр выплат',
    group: 'finance',
    description: 'Просмотр сформированных выплат продавцам',
  },
  {
    key: 'payouts.create',
    title: 'Формирование выплат',
    group: 'finance',
    description: 'Создание новых пакетов выплат продавцам',
  },
  {
    key: 'payouts.approve',
    title: 'Подтверждение выплат',
    group: 'finance',
    critical: true,
    description: 'Авторизация и финальное утверждение выплат продавцам',
  },
  {
    key: 'payouts.reject',
    title: 'Отклонение выплат',
    group: 'finance',
    description: 'Отклонение сформированных выплат продавцам',
  },
  {
    key: 'payouts.update',
    title: 'Редактирование выплат',
    group: 'finance',
    description: 'Изменение реквизитов и сумм выплат',
  },
  {
    key: 'payouts.mark_paid',
    title: 'Отметка об оплате выплат',
    group: 'finance',
    description: 'Фиксация успешного проведения банковского перевода',
  },
  {
    key: 'commission.manage',
    title: 'Управление комиссиями',
    group: 'finance',
    critical: true,
    description: 'Установка базовых и индивидуальных ставок комиссии продавцов',
  },

  // 8. Sellers (6)
  {
    key: 'sellers.read',
    title: 'Просмотр продавцов',
    group: 'sellers',
    description: 'Просмотр карточек продавцов, профилей и контактов',
  },
  {
    key: 'sellers.create_access',
    title: 'Создание доступов продавцов',
    group: 'sellers',
    description: 'Регистрация и выдача доступа в личный кабинет продавца',
  },
  {
    key: 'sellers.update_status',
    title: 'Изменение статусов продавцов',
    group: 'sellers',
    description: 'Активация, блокировка и приостановка магазинов',
  },
  {
    key: 'sellers.verify',
    title: 'Верификация продавцов',
    group: 'sellers',
    description: 'Проверка юридических документов и утверждение онбординга',
  },
  {
    key: 'sellers.warn',
    title: 'Вынесение предупреждений',
    group: 'sellers',
    description: 'Фиксация нарушений и выставление предупреждений продавцам',
  },
  {
    key: 'sellers.message',
    title: 'Отправка сообщений продавцам',
    group: 'sellers',
    description: 'Официальная коммуникация и отправка системных уведомлений продавцам',
  },

  // 9. Auctions (10)
  {
    key: 'auctions.read',
    title: 'Просмотр аукционов',
    group: 'auctions',
    description: 'Просмотр списка аукционов, лотов и ставок',
  },
  {
    key: 'auctions.create',
    title: 'Создание аукционов',
    group: 'auctions',
    description: 'Заведение новых аукционных лотов',
  },
  {
    key: 'auctions.update',
    title: 'Редактирование аукционов',
    group: 'auctions',
    description: 'Изменение параметров и условий проведения торгов',
  },
  {
    key: 'auctions.publish',
    title: 'Публикация аукционов',
    group: 'auctions',
    description: 'Открытие торгов для покупателей',
  },
  {
    key: 'auctions.pause',
    title: 'Приостановка аукционов',
    group: 'auctions',
    description: 'Временная заморозка торгов',
  },
  {
    key: 'auctions.resume',
    title: 'Возобновление аукционов',
    group: 'auctions',
    description: 'Возобновление ранее приостановленных торгов',
  },
  {
    key: 'auctions.cancel',
    title: 'Отмена аукционов',
    group: 'auctions',
    description: 'Аннулирование аукциона и возвращение лота',
  },
  {
    key: 'auctions.finalize',
    title: 'Завершение торгов',
    group: 'auctions',
    description: 'Подведение итогов торгов и определение победителя',
  },
  {
    key: 'auctions.move_to_direct_sale',
    title: 'Перевод в прямую продажу',
    group: 'auctions',
    description: 'Перевод неразыгранного лота в обычный каталог',
  },
  {
    key: 'auctions.manage_settings',
    title: 'Настройки модуля аукционов',
    group: 'auctions',
    critical: true,
    description: 'Управление правилами шага ставок, таймеров и лимитов',
  },

  // 10. Support & Reviews (10)
  {
    key: 'support.read',
    title: 'Просмотр обращений поддержки',
    group: 'support',
    description: 'Просмотр тикетов и диалогов с клиентами',
  },
  {
    key: 'support.respond',
    title: 'Ответы в поддержке',
    group: 'support',
    description: 'Отправка сообщений в тикеты покупателей и продавцов',
  },
  {
    key: 'support.close',
    title: 'Закрытие обращений',
    group: 'support',
    description: 'Перевод тикетов поддержки в статус решенных',
  },
  {
    key: 'complaints.read',
    title: 'Просмотр жалоб',
    group: 'support',
    description: 'Просмотр претензий на заказы и продавцов',
  },
  {
    key: 'complaints.resolve',
    title: 'Разрешение жалоб',
    group: 'support',
    description: 'Принятие решений по спорным ситуациям и жалобам',
  },
  {
    key: 'reviews.read',
    title: 'Просмотр отзывов',
    group: 'support',
    description: 'Просмотр отзывов покупателей на товары',
  },
  {
    key: 'reviews.approve',
    title: 'Публикация отзывов',
    group: 'support',
    description: 'Одобрение отзывов для отображения на витрине',
  },
  {
    key: 'reviews.reject',
    title: 'Отклонение отзывов',
    group: 'support',
    description: 'Отклонение отзывов, нарушающих правила',
  },
  {
    key: 'reviews.hide',
    title: 'Скрытие отзывов',
    group: 'support',
    description: 'Временное скрытие отзывов с витрины',
  },
  {
    key: 'reviews.block',
    title: 'Блокировка отзывов',
    group: 'support',
    description: 'Перманентная блокировка отзывов и спама',
  },

  // 11. Analytics & Reports (4)
  {
    key: 'dashboard.read',
    title: 'Просмотр главного дашборда',
    group: 'analytics',
    description: 'Доступ к сводным KPI показателям на главном экране',
  },
  {
    key: 'analytics.read',
    title: 'Просмотр аналитики',
    group: 'analytics',
    description: 'Доступ к графике продаж, конверсий и метрик',
  },
  {
    key: 'reports.read',
    title: 'Просмотр отчётов',
    group: 'analytics',
    description: 'Формирование и просмотр операционных отчетов',
  },
  {
    key: 'exports.excel',
    title: 'Выгрузка в Excel',
    group: 'analytics',
    description: 'Экспорт аналитических и финансовых данных в таблицы',
  },
];

/**
 * Keyed lookup map for O(1) definition access.
 */
export const STAFF_CAPABILITY_MAP: Record<string, StaffCapabilityDefinition> =
  STAFF_CAPABILITIES.reduce<Record<string, StaffCapabilityDefinition>>((acc, cap) => {
    acc[cap.key] = cap;
    return acc;
  }, {});

/**
 * Retrieve definition for a specific capability key.
 */
export function getCapabilityDefinition(key: string): StaffCapabilityDefinition | undefined {
  return STAFF_CAPABILITY_MAP[key];
}

/**
 * Retrieve all capabilities belonging to a specific group.
 */
export function getCapabilitiesByGroup(group: StaffCapabilityGroupKey): StaffCapabilityDefinition[] {
  return STAFF_CAPABILITIES.filter((cap) => cap.group === group);
}

/**
 * List all 86 capability keys.
 */
export function getAllCapabilityKeys(): string[] {
  return STAFF_CAPABILITIES.map((cap) => cap.key);
}
