import { STAFF_CAPABILITIES } from './staffCapabilities';

export type SectionMode = 'CLOSED' | 'VIEW' | 'WORK' | 'CUSTOM';

export interface StaffAccessSectionAction {
  capability: string;
  title: string;
  description?: string;
  criticality?: 'normal' | 'sensitive' | 'critical';
  isCritical?: boolean;
  isAdditional?: boolean;
}

export interface StaffAccessSection {
  key: string;
  title: string;
  description?: string;
  route?: string;
  group: string;
  actions: StaffAccessSectionAction[];
  modes: {
    view?: string[];
    work?: string[];
  };
}

export interface StaffAccessSectionGroup {
  key: string;
  title: string;
}

export const STAFF_ACCESS_SECTION_GROUPS: StaffAccessSectionGroup[] = [
  { key: 'catalog', title: 'КАТАЛОГ' },
  { key: 'sellers', title: 'ПРОДАВЦЫ' },
  { key: 'orders', title: 'ЗАКАЗЫ' },
  { key: 'warehouse_delivery', title: 'СКЛАД И ДОСТАВКА' },
  { key: 'finance', title: 'ФИНАНСЫ' },
  { key: 'management', title: 'УПРАВЛЕНИЕ' },
  { key: 'analytics', title: 'АНАЛИТИКА' },
];

/**
 * 8 Capabilities preserved outside normal editor (backend-only, platform security, testing lab, future support).
 * These must never be lost or silently deleted during draft editing.
 */
export const ADVANCED_ONLY_CAPABILITIES: string[] = [
  'security.read',
  'storefront.manage',
  'testing.manage',
  'support.read',
  'support.respond',
  'support.close',
  'complaints.read',
  'complaints.resolve',
];

/**
 * 22 Canonical real Admin Sections.
 * Presentation-only structure mapping 78 capabilities with explicit VIEW and WORK modes.
 */
export const STAFF_ACCESS_SECTIONS: StaffAccessSection[] = [
  // 1. КАТАЛОГ
  {
    key: 'products',
    title: 'Товары',
    description: 'Каталог товаров маркетплейса, публикация и блокировка',
    route: '/products',
    group: 'catalog',
    modes: {
      view: ['products.read'],
      work: ['products.read', 'products.publish', 'products.hide', 'products.block'],
    },
    actions: [
      {
        capability: 'products.read',
        title: 'Просмотр товаров',
        description: 'Просмотр товаров каталога и их характеристик',
        criticality: 'normal',
      },
      {
        capability: 'products.publish',
        title: 'Публикация товаров',
        description: 'Публикация карточек товаров в общий каталог',
        criticality: 'normal',
      },
      {
        capability: 'products.hide',
        title: 'Скрытие товаров',
        description: 'Временное скрытие товаров с витрины',
        criticality: 'normal',
      },
      {
        capability: 'products.block',
        title: 'Блокировка товаров',
        description: 'Перманентная блокировка товаров за нарушения',
        criticality: 'sensitive',
      },
    ],
  },
  {
    key: 'moderation',
    title: 'Модерация',
    description: 'Очередь модерации товаров и проверка отзывов покупателей',
    route: '/moderation',
    group: 'catalog',
    modes: {
      view: ['products.moderate', 'reviews.read'],
      work: [
        'products.moderate',
        'products.approve',
        'products.reject',
        'reviews.read',
        'reviews.approve',
        'reviews.reject',
        'reviews.hide',
        'reviews.block',
      ],
    },
    actions: [
      {
        capability: 'products.moderate',
        title: 'Взятие на модерацию',
        description: 'Доступ к очереди проверки товаров продавцов',
        criticality: 'normal',
      },
      {
        capability: 'products.approve',
        title: 'Одобрение товаров',
        description: 'Подтверждение прохождения модерации карточкой товара',
        criticality: 'normal',
      },
      {
        capability: 'products.reject',
        title: 'Отклонение товаров',
        description: 'Отклонение товара с указанием причины продавцу',
        criticality: 'normal',
      },
      {
        capability: 'reviews.read',
        title: 'Просмотр отзывов',
        description: 'Просмотр отзывов покупателей на товары',
        criticality: 'normal',
      },
      {
        capability: 'reviews.approve',
        title: 'Публикация отзывов',
        description: 'Одобрение отзывов для отображения на витрине',
        criticality: 'normal',
      },
      {
        capability: 'reviews.reject',
        title: 'Отклонение отзывов',
        description: 'Отклонение отзывов, нарушающих правила платформы',
        criticality: 'normal',
      },
      {
        capability: 'reviews.hide',
        title: 'Скрытие отзывов',
        description: 'Временное скрытие отзывов с витрины',
        criticality: 'normal',
      },
      {
        capability: 'reviews.block',
        title: 'Блокировка отзывов',
        description: 'Перманентная блокировка спама и нарушающих отзывов',
        criticality: 'sensitive',
      },
    ],
  },
  {
    key: 'catalog',
    title: 'Категории и бренды',
    description: 'Управление деревом категорий, брендов и размерных сеток',
    route: '/catalog',
    group: 'catalog',
    modes: {
      view: ['categories.read', 'brands.read'],
      work: [
        'categories.read',
        'categories.create',
        'categories.update',
        'categories.delete',
        'brands.read',
        'brands.create',
        'brands.update',
        'brands.delete',
      ],
    },
    actions: [
      {
        capability: 'categories.read',
        title: 'Просмотр категорий',
        description: 'Просмотр дерева категорий каталога',
        criticality: 'normal',
      },
      {
        capability: 'categories.create',
        title: 'Создание категорий',
        description: 'Добавление новых категорий в дерево каталога',
        criticality: 'normal',
      },
      {
        capability: 'categories.update',
        title: 'Изменение категорий',
        description: 'Редактирование категорий и размерных сеток',
        criticality: 'normal',
      },
      {
        capability: 'categories.delete',
        title: 'Удаление категорий',
        description: 'Удаление категорий из каталога',
        criticality: 'normal',
      },
      {
        capability: 'brands.read',
        title: 'Просмотр брендов',
        description: 'Просмотр справочника брендов',
        criticality: 'normal',
      },
      {
        capability: 'brands.create',
        title: 'Создание брендов',
        description: 'Добавление новых брендов в систему',
        criticality: 'normal',
      },
      {
        capability: 'brands.update',
        title: 'Изменение брендов',
        description: 'Редактирование информации о брендах',
        criticality: 'normal',
      },
      {
        capability: 'brands.delete',
        title: 'Удаление брендов',
        description: 'Удаление брендов из справочника',
        criticality: 'normal',
      },
    ],
  },

  // 2. ПРОДАВЦЫ
  {
    key: 'sellers',
    title: 'Продавцы',
    description: 'Управление партнерами, верификация юридических лиц и коммуникация',
    route: '/sellers',
    group: 'sellers',
    modes: {
      view: ['sellers.read'],
      work: [
        'sellers.read',
        'sellers.create_access',
        'sellers.update_status',
        'sellers.verify',
        'sellers.warn',
        'sellers.message',
      ],
    },
    actions: [
      {
        capability: 'sellers.read',
        title: 'Просмотр продавцов',
        description: 'Просмотр списка продавцов, карточек и контактных данных',
        criticality: 'normal',
      },
      {
        capability: 'sellers.create_access',
        title: 'Создание доступов',
        description: 'Выдача первичного доступа в кабинет продавца',
        criticality: 'normal',
      },
      {
        capability: 'sellers.update_status',
        title: 'Изменение статусов продавцов',
        description: 'Активация, блокировка и приостановка магазинов',
        criticality: 'normal',
      },
      {
        capability: 'sellers.verify',
        title: 'Верификация документов',
        description: 'Проверка юридических данных и онбординг продавцов',
        criticality: 'normal',
      },
      {
        capability: 'sellers.warn',
        title: 'Вынесение предупреждений',
        description: 'Фиксация нарушений и отправка штрафных предупреждений',
        criticality: 'normal',
      },
      {
        capability: 'sellers.message',
        title: 'Отправка сообщений',
        description: 'Официальная системная коммуникация с продавцами',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'auctions',
    title: 'Аукционы',
    description: 'Управление торгами, лотами и правилами аукционной площадки',
    route: '/auctions',
    group: 'sellers',
    modes: {
      view: ['auctions.read'],
      work: [
        'auctions.read',
        'auctions.create',
        'auctions.update',
        'auctions.publish',
        'auctions.pause',
        'auctions.resume',
        'auctions.cancel',
        'auctions.finalize',
        'auctions.move_to_direct_sale',
      ],
    },
    actions: [
      {
        capability: 'auctions.read',
        title: 'Просмотр аукционов',
        description: 'Просмотр списка аукционов, лотов и истории ставок',
        criticality: 'normal',
      },
      {
        capability: 'auctions.create',
        title: 'Создание аукционов',
        description: 'Создание новых лотов и назначение стартовых условий',
        criticality: 'normal',
      },
      {
        capability: 'auctions.update',
        title: 'Редактирование аукционов',
        description: 'Изменение параметров аукциона до начала торгов',
        criticality: 'normal',
      },
      {
        capability: 'auctions.publish',
        title: 'Публикация аукционов',
        description: 'Открытие аукциона для участия покупателей',
        criticality: 'normal',
      },
      {
        capability: 'auctions.pause',
        title: 'Приостановка аукционов',
        description: 'Временная пауза активных торгов',
        criticality: 'normal',
      },
      {
        capability: 'auctions.resume',
        title: 'Возобновление аукционов',
        description: 'Снятие с паузы и продолжение торгов',
        criticality: 'normal',
      },
      {
        capability: 'auctions.cancel',
        title: 'Отмена аукционов',
        description: 'Аннулирование аукциона и возврат лота',
        criticality: 'normal',
      },
      {
        capability: 'auctions.finalize',
        title: 'Завершение торгов',
        description: 'Подведение итогов и фиксация победителя',
        criticality: 'normal',
      },
      {
        capability: 'auctions.move_to_direct_sale',
        title: 'Перевод в прямую продажу',
        description: 'Перенос неразыгранного товара в обычный каталог',
        criticality: 'normal',
      },
      {
        capability: 'auctions.manage_settings',
        title: 'Настройки параметров аукционов',
        description: 'Управление шагом ставок, лимитами и таймерами аукционов',
        criticality: 'critical',
        isCritical: true,
        isAdditional: true,
      },
    ],
  },

  // 3. ЗАКАЗЫ
  {
    key: 'orders',
    title: 'Заказы',
    description: 'Реестр заказов клиентов и оперативное изменение статусов',
    route: '/orders',
    group: 'orders',
    modes: {
      view: ['orders.read'],
      work: ['orders.read', 'orders.update_status'],
    },
    actions: [
      {
        capability: 'orders.read',
        title: 'Просмотр заказов',
        description: 'Просмотр списка, состава и статусов заказов покупателей',
        criticality: 'normal',
      },
      {
        capability: 'orders.update_status',
        title: 'Изменение статуса заказа',
        description: 'Ручная смена статусов заказов оператором',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'returns',
    title: 'Возвраты',
    description: 'Клиентские возвраты товаров, складская инспекция и статусы',
    route: '/returns',
    group: 'orders',
    modes: {
      view: ['returns.read'],
      work: ['returns.read', 'returns.update_status', 'warehouse.returns'],
    },
    actions: [
      {
        capability: 'returns.read',
        title: 'Просмотр возвратов',
        description: 'Просмотр реестра возвратов от покупателей',
        criticality: 'normal',
      },
      {
        capability: 'returns.update_status',
        title: 'Изменение статусов возвратов',
        description: 'Утверждение и продвижение клиентских заявок на возврат',
        criticality: 'normal',
      },
      {
        capability: 'warehouse.returns',
        title: 'Складская обработка возвратов',
        description: 'Физическая приемка и инспекция состояния возвращенных товаров',
        criticality: 'normal',
      },
    ],
  },

  // 4. СКЛАД И ДОСТАВКА
  {
    key: 'receiving',
    title: 'Приёмка поставок',
    description: 'Первичная приёмка поставок продавцов на складе',
    route: '/supplies/receiving',
    group: 'warehouse_delivery',
    modes: {
      work: ['inventory.receipt'],
    },
    actions: [
      {
        capability: 'inventory.receipt',
        title: 'Оприходование поставок',
        description: 'Сканирование грузомест и постановка принятых единиц на баланс',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'orders_receiving',
    title: 'Приёмка заказов',
    description: 'Складская приёмка заказов через сканер',
    route: '/orders/receiving',
    group: 'warehouse_delivery',
    modes: {
      work: ['warehouse.receiving'],
    },
    actions: [
      {
        capability: 'warehouse.receiving',
        title: 'Приёмка заказов на складе',
        description: 'Сканирование и подтверждение приёмки заказа на сортировку',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'picking',
    title: 'Сборка заказов',
    description: 'Сборка единиц с полок склада, упаковка и передача курьерам',
    route: '/fulfillment/picking',
    group: 'warehouse_delivery',
    modes: {
      view: ['warehouse.picking'],
      work: ['warehouse.picking', 'warehouse.packing', 'warehouse.dispatch'],
    },
    actions: [
      {
        capability: 'warehouse.picking',
        title: 'Сборка заказов (Pick)',
        description: 'Отбор единиц товаров по складским волнам сборки',
        criticality: 'normal',
      },
      {
        capability: 'warehouse.packing',
        title: 'Упаковка заказов (Pack)',
        description: 'Контроль комплектации и упаковка собранных отправлений',
        criticality: 'normal',
      },
      {
        capability: 'warehouse.dispatch',
        title: 'Передача в доставку (Dispatch)',
        description: 'Отгрузка упакованных коробок курьерам служб доставки',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'inventory',
    title: 'Остатки / Склад',
    description: 'Складской учет, инвентаризация, списания и ZMU единиц',
    route: '/inventory',
    group: 'warehouse_delivery',
    modes: {
      view: ['inventory.read', 'inventory.movements.read'],
      work: [
        'inventory.read',
        'inventory.adjust',
        'inventory.write_off',
        'inventory.movements.read',
      ],
    },
    actions: [
      {
        capability: 'inventory.read',
        title: 'Просмотр остатков и ZMU',
        description: 'Просмотр балансов товаров и статусов индивидуальных ZMU',
        criticality: 'normal',
      },
      {
        capability: 'inventory.movements.read',
        title: 'История складских перемещений',
        description: 'Просмотр журнала физических перемещений остатков',
        criticality: 'normal',
      },
      {
        capability: 'inventory.adjust',
        title: 'Корректировка остатков',
        description: 'Проведение инвентаризации и ручная корректировка балансов',
        criticality: 'normal',
      },
      {
        capability: 'inventory.write_off',
        title: 'Списание товаров',
        description: 'Списание бракованных, утерянных и поврежденных единиц',
        criticality: 'sensitive',
      },
    ],
  },
  {
    key: 'shipments',
    title: 'Доставка / Отгрузки',
    description: 'Управление отправлениями, трек-номерами и логистикой доставки',
    route: '/shipments',
    group: 'warehouse_delivery',
    modes: {
      view: ['shipments.read'],
      work: ['shipments.read', 'shipments.create', 'shipments.update_status'],
    },
    actions: [
      {
        capability: 'shipments.read',
        title: 'Просмотр отправлений',
        description: 'Просмотр списка доставок, курьерских служб и трек-номеров',
        criticality: 'normal',
      },
      {
        capability: 'shipments.create',
        title: 'Формирование отправлений',
        description: 'Создание новых партий отправлений и передача в курьерские службы',
        criticality: 'normal',
      },
      {
        capability: 'shipments.update_status',
        title: 'Обновление статусов доставки',
        description: 'Ручная смена статусов этапов доставки отправлений',
        criticality: 'normal',
      },
    ],
  },

  // 5. ФИНАНСЫ
  {
    key: 'payments',
    title: 'Платежи покупателей',
    description: 'Журнал транзакций оплат заказов и эквайринга',
    route: '/payments',
    group: 'finance',
    modes: {
      view: ['payments.read'],
    },
    actions: [
      {
        capability: 'payments.read',
        title: 'Просмотр реестра платежей',
        description: 'Просмотр транзакций, платежных провайдеров и статусов списаний',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'refunds',
    title: 'Возмещения',
    description: 'Денежные возвраты покупателям по возвращенным заказам',
    route: '/refunds',
    group: 'finance',
    modes: {
      view: ['refunds.read'],
      work: ['refunds.read', 'refunds.create'],
    },
    actions: [
      {
        capability: 'refunds.read',
        title: 'Просмотр возмещений',
        description: 'Просмотр истории денежных возвратов клиентам',
        criticality: 'normal',
      },
      {
        capability: 'refunds.create',
        title: 'Инициация возвратов средств',
        description: 'Создание и проведение возврата денег покупателю на карту',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'payouts',
    title: 'Выплаты продавцам',
    description: 'Формирование реестров выплат, комиссии и утверждение транзакций',
    route: '/payouts',
    group: 'finance',
    modes: {
      view: ['payouts.read'],
      work: [
        'payouts.read',
        'payouts.create',
        'payouts.update',
        'payouts.reject',
        'payouts.mark_paid',
      ],
    },
    actions: [
      {
        capability: 'payouts.read',
        title: 'Просмотр выплат',
        description: 'Просмотр сформированных пакетов выплат продавцам',
        criticality: 'normal',
      },
      {
        capability: 'payouts.create',
        title: 'Формирование пакетов выплат',
        description: 'Расчет балансов и создание новых выплат продавцам',
        criticality: 'normal',
      },
      {
        capability: 'payouts.update',
        title: 'Редактирование выплат',
        description: 'Корректировка сумм и платежных реквизитов выплат',
        criticality: 'normal',
      },
      {
        capability: 'payouts.reject',
        title: 'Отклонение выплат',
        description: 'Отклонение некорректно сформированных выплат',
        criticality: 'normal',
      },
      {
        capability: 'payouts.mark_paid',
        title: 'Отметка об оплате',
        description: 'Фиксация исполнения банковского перевода продавцу',
        criticality: 'normal',
      },
      {
        capability: 'payouts.approve',
        title: 'Подтверждение и авторизация выплат',
        description: 'Финальное утверждение и допуск выплат к проведению',
        criticality: 'critical',
        isCritical: true,
        isAdditional: true,
      },
      {
        capability: 'commission.manage',
        title: 'Управление ставками комиссий',
        description: 'Изменение базовых и индивидуальных процентных ставок комиссии',
        criticality: 'critical',
        isCritical: true,
        isAdditional: true,
      },
    ],
  },

  // 6. УПРАВЛЕНИЕ
  {
    key: 'staff',
    title: 'Сотрудники',
    description: 'Управление учетными записями сотрудников и их правами',
    route: '/staff',
    group: 'management',
    modes: {
      view: ['staff.read'],
      work: ['staff.read', 'staff.create', 'staff.update', 'staff.block'],
    },
    actions: [
      {
        capability: 'staff.read',
        title: 'Просмотр сотрудников',
        description: 'Просмотр списка персонала и информации об аккаунтах',
        criticality: 'normal',
      },
      {
        capability: 'staff.create',
        title: 'Добавление сотрудников',
        description: 'Создание новых профилей сотрудников',
        criticality: 'normal',
      },
      {
        capability: 'staff.update',
        title: 'Редактирование профилей',
        description: 'Изменение контактных данных и сброс паролей',
        criticality: 'normal',
      },
      {
        capability: 'staff.block',
        title: 'Блокировка сотрудников',
        description: 'Блокировка и деактивация доступа сотрудников',
        criticality: 'sensitive',
      },
      {
        capability: 'staff.permissions.manage',
        title: 'Управление индивидуальными правами',
        description: 'Прямая выдача и отзыв индивидуальных прав доступа персонала',
        criticality: 'critical',
        isCritical: true,
        isAdditional: true,
      },
    ],
  },
  {
    key: 'roles',
    title: 'Шаблоны доступа',
    description: 'Системные шаблоны ролей и конфигурация наборов прав',
    route: '/roles',
    group: 'management',
    modes: {
      view: ['roles.read'],
    },
    actions: [
      {
        capability: 'roles.read',
        title: 'Просмотр шаблонов ролей',
        description: 'Просмотр списка системных ролей и входящих в них прав',
        criticality: 'normal',
      },
      {
        capability: 'roles.manage',
        title: 'Управление шаблонами ролей',
        description: 'Создание и редактирование глобальных шаблонов ролей',
        criticality: 'critical',
        isCritical: true,
        isAdditional: true,
      },
    ],
  },
  {
    key: 'audit',
    title: 'Журнал действий',
    description: 'Аудит действий сотрудников и системных событий',
    route: '/audit',
    group: 'management',
    modes: {
      view: ['audit.read'],
    },
    actions: [
      {
        capability: 'audit.read',
        title: 'Просмотр журнала аудита',
        description: 'Доступ к истории всех действий сотрудников и системных событий платформы',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'settings',
    title: 'Настройки',
    description: 'Глобальная конфигурация и параметры работы платформы',
    route: '/settings',
    group: 'management',
    modes: {
      view: ['settings.read'],
    },
    actions: [
      {
        capability: 'settings.read',
        title: 'Просмотр настроек платформы',
        description: 'Просмотр общих системных параметров и конфигураций',
        criticality: 'normal',
      },
      {
        capability: 'settings.manage',
        title: 'Изменение настроек платформы',
        description: 'Изменение глобальных параметров маркетплейса',
        criticality: 'critical',
        isCritical: true,
        isAdditional: true,
      },
    ],
  },
  {
    key: 'users',
    title: 'Покупатели',
    description: 'База зарегистрированных пользователей маркетплейса',
    route: '/users',
    group: 'management',
    modes: {
      view: ['users.read'],
    },
    actions: [
      {
        capability: 'users.read',
        title: 'Просмотр базы покупателей',
        description: 'Просмотр профилей клиентов, истории заказов и статусов аккаунтов',
        criticality: 'normal',
      },
    ],
  },

  // 7. АНАЛИТИКА
  {
    key: 'dashboard',
    title: 'Главная',
    description: 'Сводные KPI, показатели выручки и операционные метрики',
    route: '/dashboard',
    group: 'analytics',
    modes: {
      view: ['dashboard.read', 'analytics.read'],
      work: ['dashboard.read', 'analytics.read'],
    },
    actions: [
      {
        capability: 'dashboard.read',
        title: 'Просмотр дашборда',
        description: 'Доступ к главному экрану и базовым сводкам платформы',
        criticality: 'normal',
      },
      {
        capability: 'analytics.read',
        title: 'Детальная аналитика',
        description: 'Доступ к графикам продаж, конверсий и метрик',
        criticality: 'normal',
      },
    ],
  },
  {
    key: 'reports',
    title: 'Отчёты',
    description: 'Формирование сводных отчетов и выгрузка аналитики в Excel',
    route: '/reports',
    group: 'analytics',
    modes: {
      view: ['reports.read'],
      work: ['reports.read', 'exports.excel'],
    },
    actions: [
      {
        capability: 'reports.read',
        title: 'Просмотр отчётов',
        description: 'Формирование и просмотр операционных отчетов платформы',
        criticality: 'normal',
      },
      {
        capability: 'exports.excel',
        title: 'Выгрузка в Excel',
        description: 'Экспорт сводных аналитических и финансовых данных в таблицы',
        criticality: 'normal',
      },
    ],
  },
];

/**
 * Keyed lookup map for fast O(1) section retrieval.
 */
export const STAFF_ACCESS_SECTION_MAP: Record<string, StaffAccessSection> =
  STAFF_ACCESS_SECTIONS.reduce<Record<string, StaffAccessSection>>((acc, s) => {
    acc[s.key] = s;
    return acc;
  }, {});

export function getAccessSection(key: string): StaffAccessSection | undefined {
  return STAFF_ACCESS_SECTION_MAP[key];
}

/**
 * Human readable label for section access modes.
 */
export const SECTION_MODE_LABELS: Record<SectionMode, string> = {
  CLOSED: 'Закрыт',
  VIEW: 'Просмотр',
  WORK: 'Работа',
  CUSTOM: 'Настроено вручную',
};

/**
 * Determine the active human access mode of a section given the employee draft permissions.
 */
export function getSectionMode(
  section: StaffAccessSection,
  draftPermissions: string[]
): SectionMode {
  const sectionCapKeys = new Set(section.actions.map((a) => a.capability));
  const currentSectionCaps = draftPermissions.filter((cap) => sectionCapKeys.has(cap));

  if (currentSectionCaps.length === 0) {
    return 'CLOSED';
  }

  const currentSet = new Set(currentSectionCaps);

  const matchesWork =
    Boolean(section.modes.work && section.modes.work.length > 0) &&
    section.modes.work!.length === currentSet.size &&
    section.modes.work!.every((c) => currentSet.has(c));

  if (matchesWork) {
    return 'WORK';
  }

  const matchesView =
    Boolean(section.modes.view && section.modes.view.length > 0) &&
    section.modes.view!.length === currentSet.size &&
    section.modes.view!.every((c) => currentSet.has(c));

  if (matchesView) {
    return 'VIEW';
  }

  return 'CUSTOM';
}

/**
 * Apply a predefined access mode to a section, updating ONLY the capabilities
 * owned by this section, preserving other sections and advanced/hidden rights.
 */
export function applySectionMode(
  section: StaffAccessSection,
  mode: 'CLOSED' | 'VIEW' | 'WORK',
  currentDraft: string[]
): string[] {
  const sectionCapKeys = new Set(section.actions.map((a) => a.capability));
  // Preserve capabilities belonging to other sections and unknown/advanced capabilities
  const preservedDraft = currentDraft.filter((cap) => !sectionCapKeys.has(cap));

  if (mode === 'CLOSED') {
    return preservedDraft;
  }

  if (mode === 'VIEW') {
    const viewCaps = section.modes.view || [];
    return Array.from(new Set([...preservedDraft, ...viewCaps]));
  }

  if (mode === 'WORK') {
    const workCaps = section.modes.work || section.modes.view || [];
    return Array.from(new Set([...preservedDraft, ...workCaps]));
  }

  return currentDraft;
}

/**
 * Toggle an individual action capability within the current draft.
 */
export function toggleSectionAction(
  capability: string,
  currentDraft: string[]
): string[] {
  const draftSet = new Set(currentDraft);
  if (draftSet.has(capability)) {
    draftSet.delete(capability);
  } else {
    draftSet.add(capability);
  }
  return Array.from(draftSet);
}

export interface DraftChanges {
  added: string[];
  removed: string[];
  isDirty: boolean;
  criticalAdded: string[];
  criticalRemoved: string[];
}

/**
 * Compute symmetric difference between original and draft permissions,
 * identifying additions, removals, and critical alterations.
 */
export function getDraftChanges(
  originalPermissions: string[],
  draftPermissions: string[]
): DraftChanges {
  const origSet = new Set(originalPermissions);
  const draftSet = new Set(draftPermissions);

  const added = draftPermissions.filter((p) => !origSet.has(p));
  const removed = originalPermissions.filter((p) => !draftSet.has(p));
  const isDirty = added.length > 0 || removed.length > 0;

  const criticalMap = new Set(
    STAFF_CAPABILITIES.filter((c) => c.critical).map((c) => c.key)
  );

  const criticalAdded = added.filter((p) => criticalMap.has(p));
  const criticalRemoved = removed.filter((p) => criticalMap.has(p));

  return {
    added,
    removed,
    isDirty,
    criticalAdded,
    criticalRemoved,
  };
}
