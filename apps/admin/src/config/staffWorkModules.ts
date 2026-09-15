export type StaffWorkModuleKey =
  | 'staff_access'
  | 'platform'
  | 'catalog'
  | 'moderation'
  | 'sellers'
  | 'orders_delivery'
  | 'warehouse'
  | 'returns'
  | 'finance'
  | 'support'
  | 'auctions'
  | 'analytics';

export type ActionClass = 'read' | 'write' | 'destructive' | 'security';
export type ActionCriticality = 'normal' | 'sensitive' | 'critical';
export type ActionSurface = 'current_ui' | 'backend_only' | 'future';

export interface StaffWorkModuleAction {
  capability: string;
  title: string;
  description: string;
  class: ActionClass;
  criticality: ActionCriticality;
  surface: ActionSurface;
  dependency?: string[];
}

export interface StaffWorkModuleSection {
  key: string;
  title: string;
  actions: StaffWorkModuleAction[];
}

export interface StaffWorkModule {
  key: StaffWorkModuleKey;
  title: string;
  description: string;
  iconName: string;
  sections: StaffWorkModuleSection[];
}

export type WorkModuleAccessState = 'NO_ACCESS' | 'VIEW' | 'LIMITED' | 'FULL';

export interface WorkModuleSummary {
  moduleKey: StaffWorkModuleKey;
  assignedCount: number;
  totalCount: number;
  state: WorkModuleAccessState;
  criticalAssignedCount: number;
  topAssignedActions: StaffWorkModuleAction[];
}

export interface AccessTemplateDiff {
  added: string[];
  removed: string[];
  matches: boolean;
}

export interface StaffScreenAccessRule {
  key: string;
  title: string;
  route: string;
  visibility: string | string[];
}

/**
 * Proposed explicit screen visibility rules for Admin navigation.
 * Screen-based (not module-based) to guarantee navigation coherence.
 */
export const STAFF_SCREEN_ACCESS_RULES: StaffScreenAccessRule[] = [
  { key: 'dashboard', title: 'Главная', route: '/dashboard', visibility: ['dashboard.read', 'analytics.read'] },
  { key: 'users', title: 'Покупатели', route: '/users', visibility: 'users.read' },
  { key: 'sellers', title: 'Продавцы', route: '/sellers', visibility: 'sellers.read' },
  { key: 'products', title: 'Товары', route: '/products', visibility: 'products.read' },
  { key: 'moderation', title: 'Модерация', route: '/moderation', visibility: ['products.moderate', 'reviews.read', 'sellers.read'] },
  { key: 'catalog', title: 'Категории и бренды', route: '/catalog', visibility: ['categories.read', 'brands.read'] },
  { key: 'orders', title: 'Заказы', route: '/orders', visibility: 'orders.read' },
  { key: 'picking', title: 'Сборка', route: '/fulfillment/picking', visibility: 'warehouse.picking' },
  { key: 'packing', title: 'Упаковка', route: '/fulfillment/packing', visibility: 'warehouse.packing' },
  { key: 'dispatch', title: 'Отгрузка', route: '/fulfillment/dispatch', visibility: 'warehouse.dispatch' },
  { key: 'orders_receiving', title: 'Приемка заказов', route: '/orders/receiving', visibility: 'warehouse.receiving' },
  { key: 'receiving', title: 'Приемка поставок', route: '/supplies/receiving', visibility: 'inventory.receipt' },
  { key: 'returns_receiving', title: 'Приемка возвратов', route: '/returns/receiving', visibility: 'warehouse.returns' },
  { key: 'shipments', title: 'Отправления', route: '/shipments', visibility: 'shipments.read' },
  { key: 'inventory', title: 'Остатки', route: '/inventory', visibility: 'inventory.read' },
  { key: 'free_scan', title: 'Свободный сканер', route: '/warehouse/free-scan', visibility: ['inventory.read', 'inventory.receipt'] },
  { key: 'returns', title: 'Возвраты', route: '/returns', visibility: 'returns.read' },
  { key: 'refunds', title: 'Возмещения', route: '/refunds', visibility: 'refunds.read' },
  { key: 'payments', title: 'Платежи покупателей', route: '/payments', visibility: 'payments.read' },
  { key: 'payouts', title: 'Выплаты продавцам', route: '/payouts', visibility: 'payouts.read' },
  { key: 'auctions', title: 'Аукционы', route: '/auctions', visibility: 'auctions.read' },
  { key: 'reports', title: 'Сводные отчеты', route: '/reports', visibility: 'reports.read' },
  { key: 'roles', title: 'Доступы и роли', route: '/roles', visibility: 'roles.read' },
  { key: 'staff', title: 'Сотрудники', route: '/staff', visibility: 'staff.read' },
  { key: 'audit', title: 'Журнал действий', route: '/audit', visibility: 'audit.read' },
  { key: 'settings', title: 'Настройки', route: '/settings', visibility: 'settings.read' },
];

/**
 * Retrieve explicit screen access rule for a route or key.
 */
export function getStaffScreenAccessRule(routeOrKey: string): StaffScreenAccessRule | undefined {
  return STAFF_SCREEN_ACCESS_RULES.find(
    (rule) => rule.route === routeOrKey || rule.key === routeOrKey
  );
}

/**
 * Retrieve visibility requirement (string or string[]) for a route or key.
 */
export function getStaffScreenVisibility(routeOrKey: string): string | string[] | undefined {
  return getStaffScreenAccessRule(routeOrKey)?.visibility;
}

/**
 * Check if screen visibility rule is satisfied using permission predicates.
 * Handles single capability (string) and multi-capability OR rules (string[]).
 */
export function isScreenRuleVisible(
  visibility: string | string[],
  hasPermission: (permission: string) => boolean,
  hasAnyPermission?: (permissions: string[]) => boolean
): boolean {
  if (Array.isArray(visibility)) {
    if (hasAnyPermission) {
      return hasAnyPermission(visibility);
    }
    return visibility.some((p) => hasPermission(p));
  }
  return hasPermission(visibility);
}

/**
 * Check if screen visibility rule is satisfied using a permission list or set.
 * Set-based for high performance in list loops (e.g. preview modal).
 */
export function isScreenVisibleWithPermissions(
  visibility: string | string[],
  permissions: string[] | Set<string>
): boolean {
  const permSet = permissions instanceof Set ? permissions : new Set(permissions);
  if (Array.isArray(visibility)) {
    return visibility.some((p) => permSet.has(p));
  }
  return permSet.has(visibility);
}

/**
 * 12 Canonical Staff Work Modules.
 * Presentation-only structure mapping all 86 atomic capabilities.
 */
export const STAFF_WORK_MODULES: StaffWorkModule[] = [
  // 1. Staff & Access (7)
  {
    key: 'staff_access',
    title: 'Сотрудники и доступы',
    description: 'Управление доступом персонала, учетными записями и шаблонами ролей',
    iconName: 'Users',
    sections: [
      {
        key: 'staff_members',
        title: 'Сотрудники',
        actions: [
          {
            capability: 'staff.read',
            title: 'Просмотр сотрудников',
            description: 'Просмотр списка сотрудников и их базовой информации',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'staff.create',
            title: 'Добавление сотрудников',
            description: 'Создание новых учетных записей персонала',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'staff.update',
            title: 'Редактирование профилей',
            description: 'Изменение данных учетных записей сотрудников',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'staff.block',
            title: 'Блокировка сотрудников',
            description: 'Блокировка и деактивация учетных записей сотрудников',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'staff.permissions.manage',
            title: 'Управление правами сотрудников',
            description: 'Назначение и отзыв индивидуальных прав доступа персонала',
            class: 'security',
            criticality: 'critical',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'staff_roles',
        title: 'Шаблоны ролей',
        actions: [
          {
            capability: 'roles.read',
            title: 'Просмотр ролей',
            description: 'Просмотр списка системных и пользовательских ролей',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'roles.manage',
            title: 'Управление ролями',
            description: 'Создание, редактирование и настройка прав ролей',
            class: 'security',
            criticality: 'critical',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 2. Platform & System (7)
  {
    key: 'platform',
    title: 'Платформа и система',
    description: 'Системные параметры, аудит действий, покупатели и безопасность платформы',
    iconName: 'Settings',
    sections: [
      {
        key: 'settings',
        title: 'Настройки и конфигурация',
        actions: [
          {
            capability: 'settings.read',
            title: 'Просмотр настроек',
            description: 'Просмотр системных параметров и конфигурации платформы',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'settings.manage',
            title: 'Управление настройками',
            description: 'Изменение системных параметров и конфигурации платформы',
            class: 'security',
            criticality: 'critical',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'audit_security',
        title: 'Аудит и безопасность',
        actions: [
          {
            capability: 'audit.read',
            title: 'Просмотр журнала аудита',
            description: 'Просмотр истории системных событий и действий сотрудников',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'security.read',
            title: 'Просмотр событий безопасности',
            description: 'Мониторинг инцидентов и параметров безопасности платформы',
            class: 'read',
            criticality: 'sensitive',
            surface: 'backend_only',
          },
        ],
      },
      {
        key: 'users',
        title: 'Покупатели',
        actions: [
          {
            capability: 'users.read',
            title: 'Просмотр покупателей',
            description: 'Просмотр списка и профилей зарегистрированных покупателей',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'system_ops',
        title: 'Служебные операции',
        actions: [
          {
            capability: 'storefront.manage',
            title: 'Управление витриной',
            description: 'Настройка баннеров, промо-блоков и главной страницы магазина',
            class: 'write',
            criticality: 'normal',
            surface: 'future',
          },
          {
            capability: 'testing.manage',
            title: 'Управление тестовыми сценариями',
            description: 'Запуск сценариев генерации данных и сброс тестового контура',
            class: 'destructive',
            criticality: 'critical',
            surface: 'backend_only',
          },
        ],
      },
    ],
  },

  // 3. Catalog (9)
  {
    key: 'catalog',
    title: 'Каталог товаров',
    description: 'Рубрикатор категорий, справочник брендов и карточки товаров',
    iconName: 'BookOpen',
    sections: [
      {
        key: 'categories',
        title: 'Категории',
        actions: [
          {
            capability: 'categories.read',
            title: 'Просмотр категорий',
            description: 'Просмотр дерева категорий и товарного рубрикатора',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'categories.create',
            title: 'Создание категорий',
            description: 'Добавление новых категорий в товарный рубрикатор',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'categories.update',
            title: 'Редактирование категорий',
            description: 'Изменение названий, свойств и структуры категорий',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'categories.delete',
            title: 'Удаление категорий',
            description: 'Удаление категорий из товарного рубрикатора',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'brands',
        title: 'Бренды',
        actions: [
          {
            capability: 'brands.read',
            title: 'Просмотр брендов',
            description: 'Просмотр справочника брендов и производителей',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'brands.create',
            title: 'Создание брендов',
            description: 'Добавление новых брендов в систему',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'brands.update',
            title: 'Редактирование брендов',
            description: 'Изменение информации и логотипов брендов',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'brands.delete',
            title: 'Удаление брендов',
            description: 'Удаление брендов из системы',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'products_catalog',
        title: 'Товары',
        actions: [
          {
            capability: 'products.read',
            title: 'Просмотр товаров',
            description: 'Просмотр каталога товаров, карточек и спецификаций',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 4. Moderation (6)
  {
    key: 'moderation',
    title: 'Модерация',
    description: 'Проверка товарных предложений, публикация на витрине и блокировка нарушений',
    iconName: 'ShieldAlert',
    sections: [
      {
        key: 'products_moderation',
        title: 'Модерация товаров',
        actions: [
          {
            capability: 'products.moderate',
            title: 'Модерация товаров',
            description: 'Доступ к очереди проверки товаров продавцов',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'products.approve',
            title: 'Одобрение товаров',
            description: 'Утверждение карточек товаров для допуска к продаже',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'products.reject',
            title: 'Отклонение товаров',
            description: 'Возврат карточек товаров продавцу на доработку',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'products.publish',
            title: 'Принудительная публикация',
            description: 'Принудительный вывод одобренного товара на витрину',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'products.hide',
            title: 'Скрытие товаров',
            description: 'Временное снятие товара с витрины маркетплейса',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'products.block',
            title: 'Блокировка товаров',
            description: 'Перманентная блокировка товаров, нарушающих правила',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 5. Sellers (6)
  {
    key: 'sellers',
    title: 'Продавцы',
    description: 'Онбординг, верификация юридических лиц, коммуникация и статус магазинов',
    iconName: 'Store',
    sections: [
      {
        key: 'sellers_management',
        title: 'Управление продавцами',
        actions: [
          {
            capability: 'sellers.read',
            title: 'Просмотр продавцов',
            description: 'Просмотр карточек продавцов, профилей и контактов',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'sellers.create_access',
            title: 'Создание доступов продавцов',
            description: 'Регистрация и выдача доступа в личный кабинет продавца',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'sellers.update_status',
            title: 'Изменение статусов продавцов',
            description: 'Активация, блокировка и приостановка магазинов',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'sellers.verify',
            title: 'Верификация продавцов',
            description: 'Проверка юридических документов и утверждение онбординга',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'sellers.warn',
            title: 'Вынесение предупреждений',
            description: 'Фиксация нарушений и выставление предупреждений продавцам',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'sellers.message',
            title: 'Отправка сообщений продавцам',
            description: 'Официальная коммуникация и отправка системных уведомлений продавцам',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 6. Orders & Delivery (5)
  {
    key: 'orders_delivery',
    title: 'Заказы и доставки',
    description: 'Обработка заказов покупателей, маршрутизация и статусы отправлений',
    iconName: 'ShoppingCart',
    sections: [
      {
        key: 'orders_ops',
        title: 'Заказы',
        actions: [
          {
            capability: 'orders.read',
            title: 'Просмотр заказов',
            description: 'Просмотр заказов покупателей, состава и статусов оплаты',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'orders.update_status',
            title: 'Изменение статусов заказов',
            description: 'Ручной перевод заказов по этапам жизненного цикла',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'shipments_ops',
        title: 'Отправления и доставка',
        actions: [
          {
            capability: 'shipments.read',
            title: 'Просмотр отправлений',
            description: 'Просмотр списков отправлений и трекинг посылок',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'shipments.create',
            title: 'Создание отправлений',
            description: 'Формирование новых посылок и передача в службы доставки',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'shipments.update_status',
            title: 'Обновление статусов доставки',
            description: 'Актуализация этапов доставки отправлений',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 7. Warehouse & Logistics (9)
  {
    key: 'warehouse',
    title: 'Склад и логистика',
    description: 'Приёмка поставок, сборка, упаковка, отгрузка и управление остатками',
    iconName: 'Boxes',
    sections: [
      {
        key: 'physical_operations',
        title: 'Складские операции',
        actions: [
          {
            capability: 'warehouse.receiving',
            title: 'Приёмка на складе',
            description: 'Приёмка входящих грузовых мест и паллет на складе',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'warehouse.picking',
            title: 'Сборка заказов',
            description: 'Отбор товаров по сборочным листам и сканирование ZMU',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'warehouse.packing',
            title: 'Упаковка заказов',
            description: 'Контроль комплектации и упаковка отобранных товаров',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'warehouse.dispatch',
            title: 'Отгрузка отправлений',
            description: 'Передача упакованных заказов в курьерские службы',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'inventory_control',
        title: 'Складской учёт и остатки',
        actions: [
          {
            capability: 'inventory.read',
            title: 'Просмотр остатков',
            description: 'Просмотр складских остатков, ячеек хранения и квантов',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'inventory.receipt',
            title: 'Приёмка поставок продавцов',
            description: 'Приёмка и сканирование партий поставок от продавцов',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'inventory.adjust',
            title: 'Корректировки и инвентаризация остатков',
            description: 'Проведение инвентаризаций и ручная корректировка количеств в ячейках',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'inventory.write_off',
            title: 'Списание товаров',
            description: 'Списание брака, утерянных или поврежденных товаров',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'inventory.movements.read',
            title: 'Просмотр движений товаров',
            description: 'История перемещений, списаний и оприходований товаров',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 8. Returns (3)
  {
    key: 'returns',
    title: 'Возвраты',
    description: 'Обработка клиентских возвратов и складская приёмка возвратных товаров',
    iconName: 'RotateCcw',
    sections: [
      {
        key: 'returns_ops',
        title: 'Клиентские возвраты',
        actions: [
          {
            capability: 'returns.read',
            title: 'Просмотр возвратов',
            description: 'Просмотр списка возвратов от покупателей и их причин',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'returns.update_status',
            title: 'Изменение статусов возвратов',
            description: 'Принятие решений по возвратам и смена статусов',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'warehouse.returns',
            title: 'Складская приёмка возвратов',
            description: 'Физическая приёмка и осмотр возвращённого товара на складе',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 9. Finance & Payouts (10)
  {
    key: 'finance',
    title: 'Финансы и выплаты',
    description: 'Платежи покупателей, возмещения, комиссии и утверждение выплат продавцам',
    iconName: 'Wallet',
    sections: [
      {
        key: 'payments_and_refunds',
        title: 'Платежи и возмещения',
        actions: [
          {
            capability: 'payments.read',
            title: 'Просмотр платежей',
            description: 'Просмотр входящих транзакций и истории оплат',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'refunds.read',
            title: 'Просмотр возмещений',
            description: 'Просмотр истории возвратов денежных средств покупателям',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'refunds.create',
            title: 'Оформление возмещений',
            description: 'Инициация и проведение денежных возвратов клиентам',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'payouts_management',
        title: 'Выплаты продавцам',
        actions: [
          {
            capability: 'payouts.read',
            title: 'Просмотр выплат',
            description: 'Просмотр истории и реестров выплат продавцам',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'payouts.create',
            title: 'Формирование ведомостей выплат',
            description: 'Создание новых реестров выплат продавцам',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'payouts.update',
            title: 'Процессинг и приостановка выплат',
            description: 'Отправка в процессинг и временное удержание выплат продавцам',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'payouts.approve',
            title: 'Утверждение выплат',
            description: 'Авторизация и финальное одобрение выплат продавцам',
            class: 'security',
            criticality: 'critical',
            surface: 'current_ui',
          },
          {
            capability: 'payouts.reject',
            title: 'Отклонение выплат',
            description: 'Отклонение сформированных заявок на выплату',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'payouts.mark_paid',
            title: 'Отметка об оплате выплат',
            description: 'Фиксация успешного проведения банковского перевода',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'commissions_management',
        title: 'Комиссии',
        actions: [
          {
            capability: 'commission.manage',
            title: 'Управление комиссиями',
            description: 'Установка базовых и индивидуальных ставок комиссии продавцов',
            class: 'security',
            criticality: 'critical',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 10. Support & Reviews (10)
  {
    key: 'support',
    title: 'Обращения и отзывы',
    description: 'Тикеты службы поддержки, рассмотрение жалоб и модерация отзывов',
    iconName: 'MessageSquare',
    sections: [
      {
        key: 'support_tickets',
        title: 'Служба поддержки',
        actions: [
          {
            capability: 'support.read',
            title: 'Просмотр обращений поддержки',
            description: 'Просмотр тикетов и диалогов с клиентами',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'support.respond',
            title: 'Ответы в поддержке',
            description: 'Отправка сообщений в тикеты покупателей и продавцов',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'support.close',
            title: 'Закрытие обращений',
            description: 'Перевод тикетов поддержки в статус решенных',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'complaints_arbitration',
        title: 'Жалобы и претензии',
        actions: [
          {
            capability: 'complaints.read',
            title: 'Просмотр жалоб',
            description: 'Просмотр претензий на заказы и продавцов',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'complaints.resolve',
            title: 'Разрешение жалоб',
            description: 'Принятие решений по спорным ситуациям и жалобам',
            class: 'write',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'reviews_moderation',
        title: 'Отзывы покупателей',
        actions: [
          {
            capability: 'reviews.read',
            title: 'Просмотр отзывов',
            description: 'Просмотр отзывов покупателей на товары',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'reviews.approve',
            title: 'Публикация отзывов',
            description: 'Одобрение отзывов для отображения на витрине',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'reviews.reject',
            title: 'Отклонение отзывов',
            description: 'Отклонение отзывов, нарушающих правила',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'reviews.hide',
            title: 'Скрытие отзывов',
            description: 'Временное скрытие отзывов с витрины',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'reviews.block',
            title: 'Блокировка отзывов',
            description: 'Перманентная блокировка отзывов и спама',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },

  // 11. Auctions (10)
  {
    key: 'auctions',
    title: 'Аукционы',
    description: 'Создание лотов, проведение торгов, завершение и параметры аукционов',
    iconName: 'Gavel',
    sections: [
      {
        key: 'auction_trading',
        title: 'Проведение аукционов',
        actions: [
          {
            capability: 'auctions.read',
            title: 'Просмотр аукционов',
            description: 'Просмотр списка аукционов, лотов и ставок',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.create',
            title: 'Создание аукционов',
            description: 'Заведение новых аукционных лотов',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.update',
            title: 'Редактирование аукционов',
            description: 'Изменение параметров и условий проведения торгов',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.publish',
            title: 'Публикация аукционов',
            description: 'Открытие торгов для покупателей',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.pause',
            title: 'Приостановка аукционов',
            description: 'Временная заморозка торгов',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.resume',
            title: 'Возобновление аукционов',
            description: 'Возобновление ранее приостановленных торгов',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.cancel',
            title: 'Отмена аукционов',
            description: 'Аннулирование аукциона и возвращение лота',
            class: 'destructive',
            criticality: 'sensitive',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.finalize',
            title: 'Завершение торгов',
            description: 'Подведение итогов торгов и определение победителя',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'auctions.move_to_direct_sale',
            title: 'Перевод в прямую продажу',
            description: 'Перевод неразыгранного лота в обычный каталог',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'auction_parameters',
        title: 'Настройки модуля',
        actions: [
          {
            capability: 'auctions.manage_settings',
            title: 'Настройки модуля аукционов',
            description: 'Управление правилами шага ставок, таймеров и лимитов',
            class: 'security',
            criticality: 'critical',
            surface: 'future',
          },
        ],
      },
    ],
  },

  // 12. Analytics & Reports (4)
  {
    key: 'analytics',
    title: 'Аналитика и отчёты',
    description: 'Главный дашборд, графики метрик, операционные отчёты и экспорт',
    iconName: 'BarChart3',
    sections: [
      {
        key: 'dashboards',
        title: 'Аналитика и дашборды',
        actions: [
          {
            capability: 'dashboard.read',
            title: 'Просмотр главного дашборда',
            description: 'Доступ к сводным KPI показателям на главном экране',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'analytics.read',
            title: 'Просмотр аналитики',
            description: 'Доступ к графике продаж, конверсий и метрик',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
      {
        key: 'reports',
        title: 'Отчёты и выгрузки',
        actions: [
          {
            capability: 'reports.read',
            title: 'Просмотр отчётов',
            description: 'Формирование и просмотр операционных отчетов',
            class: 'read',
            criticality: 'normal',
            surface: 'current_ui',
          },
          {
            capability: 'exports.excel',
            title: 'Выгрузка в Excel',
            description: 'Экспорт аналитических и финансовых данных в таблицы',
            class: 'write',
            criticality: 'normal',
            surface: 'current_ui',
          },
        ],
      },
    ],
  },
];

/**
 * Retrieve all action definitions contained in a work module.
 */
export function getWorkModuleActions(module: StaffWorkModule): StaffWorkModuleAction[] {
  return module.sections.flatMap((section) => section.actions);
}

/**
 * Retrieve all action definitions across all 12 work modules.
 */
export function getAllWorkModuleActions(): StaffWorkModuleAction[] {
  return STAFF_WORK_MODULES.flatMap((m) => getWorkModuleActions(m));
}

/**
 * Lookup map from capability key to its owning Work Module.
 */
export const CAPABILITY_TO_WORK_MODULE_MAP: Record<string, StaffWorkModule> =
  STAFF_WORK_MODULES.reduce<Record<string, StaffWorkModule>>((acc, module) => {
    for (const action of getWorkModuleActions(module)) {
      acc[action.capability] = module;
    }
    return acc;
  }, {});

/**
 * Retrieve the work module that owns a specific capability key.
 */
export function getWorkModuleByCapability(capability: string): StaffWorkModule | undefined {
  return CAPABILITY_TO_WORK_MODULE_MAP[capability];
}

/**
 * Pure helper to compute access state for a work module given direct permissions.
 *
 * Deterministic semantics:
 * - NO_ACCESS: 0 module capabilities assigned
 * - FULL: all module capabilities assigned (100%)
 * - VIEW: assigned capabilities are non-mutating read-only actions only
 * - LIMITED: partial access with write/destructive/security actions assigned
 */
export function getWorkModuleAccessState(
  module: StaffWorkModule,
  directPermissions: string[]
): WorkModuleAccessState {
  const actions = getWorkModuleActions(module);
  if (actions.length === 0) return 'NO_ACCESS';

  const assigned = actions.filter((a) => directPermissions.includes(a.capability));
  if (assigned.length === 0) {
    return 'NO_ACCESS';
  }

  if (assigned.length === actions.length) {
    return 'FULL';
  }

  const allAssignedAreRead = assigned.every((a) => a.class === 'read');
  if (allAssignedAreRead) {
    return 'VIEW';
  }

  return 'LIMITED';
}

/**
 * Compute summary statistics for a work module.
 */
export function getWorkModuleSummary(
  module: StaffWorkModule,
  directPermissions: string[]
): WorkModuleSummary {
  const actions = getWorkModuleActions(module);
  const assigned = actions.filter((a) => directPermissions.includes(a.capability));
  const criticalAssigned = assigned.filter((a) => a.criticality === 'critical');

  return {
    moduleKey: module.key,
    assignedCount: assigned.length,
    totalCount: actions.length,
    state: getWorkModuleAccessState(module, directPermissions),
    criticalAssignedCount: criticalAssigned.length,
    topAssignedActions: assigned.slice(0, 3),
  };
}

/**
 * Retrieve all work modules where employee has at least one assigned capability.
 * Powers future Overview: "Рабочие зоны".
 */
export function getAssignedWorkModules(directPermissions: string[]): StaffWorkModule[] {
  return STAFF_WORK_MODULES.filter(
    (module) => getWorkModuleAccessState(module, directPermissions) !== 'NO_ACCESS'
  );
}

/**
 * Compute access template diff between direct permissions and a preset.
 * Set-based, order-independent.
 */
export function getAccessTemplateDiff(
  directPermissions: string[],
  presetPermissions: string[]
): AccessTemplateDiff {
  const directSet = new Set(directPermissions);
  const presetSet = new Set(presetPermissions);

  const added = directPermissions.filter((p) => !presetSet.has(p)).sort();
  const removed = presetPermissions.filter((p) => !directSet.has(p)).sort();
  const matches = added.length === 0 && removed.length === 0;

  return {
    added,
    removed,
    matches,
  };
}
