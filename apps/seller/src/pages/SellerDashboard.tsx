import {
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  BarChart3,
  Boxes,
  CheckCircle2,
  ClipboardList,
  CreditCard,
  Edit3,
  Eye,
  FileText,
  Megaphone,
  MousePointerClick,
  PackageCheck,
  Percent,
  ReceiptText,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  ShoppingBag,
  Store,
  Target,
  Truck,
  Users,
  Wallet,
  Zap,
  type LucideIcon,
} from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const numberFormatter = new Intl.NumberFormat('ru-RU');

const percentFormatter = new Intl.NumberFormat('ru-RU', {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

const formatCurrency = (value: number) => currencyFormatter.format(value);
const formatNumber = (value: number) => numberFormatter.format(value);
const formatPercent = (value: number) => `${percentFormatter.format(value)}%`;

const miniStats = [
  { label: 'Прибыль', value: formatCurrency(337850), hint: '+18% к прошлой неделе' },
  { label: 'Выкупы', value: '286', hint: '83,6% от заказов' },
  { label: 'Средний чек', value: formatCurrency(4520), hint: '+420 ₽ за неделю' },
  { label: 'Рейтинг', value: '4,7', hint: 'стабильно высокий' },
];

const metrics: Array<{
  title: string;
  value: string;
  change: string;
  trend: 'up' | 'down';
  icon: LucideIcon;
}> = [
  { title: 'Выручка', value: formatCurrency(486500), change: '+14,8% к прошлой неделе', trend: 'up', icon: Wallet },
  { title: 'Заказы', value: '342', change: '+32 заказа', trend: 'up', icon: ShoppingBag },
  { title: 'Просмотры', value: formatNumber(48920), change: '+9,4% охватов', trend: 'up', icon: Eye },
  { title: 'Конверсия', value: '2,7%', change: '-0,4 п.п.', trend: 'down', icon: Percent },
  { title: 'Расходы на продвижение', value: formatCurrency(63500), change: '+11,2% расходов', trend: 'down', icon: Megaphone },
  { title: 'Возвраты', value: '18', change: '+5 возвратов', trend: 'down', icon: ReceiptText },
];

const revenueByDay = [
  { day: 'Пн', value: 52000 },
  { day: 'Вт', value: 61000 },
  { day: 'Ср', value: 57500 },
  { day: 'Чт', value: 74000 },
  { day: 'Пт', value: 89500 },
  { day: 'Сб', value: 81000 },
  { day: 'Вс', value: 71500 },
];

const funnel = [
  { label: 'Показы', value: 48920, icon: Eye },
  { label: 'Переходы', value: 12480, icon: MousePointerClick },
  { label: 'Корзина', value: 1840, icon: ShoppingBag },
  { label: 'Заказы', value: 342, icon: PackageCheck },
  { label: 'Выкупы', value: 286, icon: Target },
];

const alerts = [
  {
    title: 'Топ базовый асимметричный смотрят, но редко покупают',
    description: 'CTR ниже среднего, заказов мало. Проверь первое фото, цену и описание.',
    tone: 'warning',
  },
  {
    title: 'Слипоны мягкой формы почти закончились',
    description: 'Осталось 5 единиц. Товар продаётся стабильно, лучше пополнить остатки.',
    tone: 'success',
  },
  {
    title: 'Ремень с матовой пряжкой даёт слабую отдачу от рекламы',
    description: 'Расходы на продвижение растут, но заказов мало. Стоит снизить ставку или обновить карточку.',
    tone: 'danger',
  },
];

const searchRows = [
  {
    query: 'анорaк мужской',
    impressions: 8400,
    clicks: 1320,
    orders: 54,
    conversion: 4.1,
    insight: 'Усилить продвижение',
  },
  {
    query: 'топ асимметричный',
    impressions: 7100,
    clicks: 504,
    orders: 9,
    conversion: 1.8,
    insight: 'Проверить фото и цену',
  },
  {
    query: 'слипоны черные',
    impressions: 3900,
    clicks: 445,
    orders: 22,
    conversion: 4.9,
    insight: 'Пополнить остатки',
  },
];

const productRows = [
  {
    product: 'Анорак ледяной линии',
    views: 12800,
    ctr: 14.2,
    orders: 84,
    revenue: 1251600,
    ads: 18500,
    stock: 18,
    rating: 4.8,
    returns: 4,
    status: 'Растёт',
  },
  {
    product: 'Топ базовый асимметричный',
    views: 9100,
    ctr: 7.1,
    orders: 19,
    revenue: 237500,
    ads: 14200,
    stock: 31,
    rating: 4.3,
    returns: 7,
    status: 'Слабая карточка',
  },
  {
    product: 'Слипоны мягкой формы',
    views: 6200,
    ctr: 11.4,
    orders: 36,
    revenue: 302400,
    ads: 7200,
    stock: 5,
    rating: 4.7,
    returns: 2,
    status: 'Пополнить',
  },
  {
    product: 'Ремень с матовой пряжкой',
    views: 4100,
    ctr: 5.2,
    orders: 8,
    revenue: 46400,
    ads: 6100,
    stock: 44,
    rating: 4.1,
    returns: 5,
    status: 'Проблема',
  },
];

const financeRows = [
  { label: 'Выручка', value: 486500, type: 'income' },
  { label: 'Комиссия площадки', value: -48650, type: 'expense' },
  { label: 'Логистика', value: -21300, type: 'expense' },
  { label: 'Продвижение', value: -63500, type: 'expense' },
  { label: 'Возвраты', value: -15200, type: 'expense' },
  { label: 'Чистая прибыль', value: 337850, type: 'profit' },
];

const recommendations = [
  'Пополнить размеры 39–40 для “Слипоны мягкой формы”.',
  'Переснять первое фото у “Топ базовый асимметричный”.',
  'Снизить рекламную ставку у “Ремень с матовой пряжкой”.',
  'Усилить продвижение по запросу “анорак мужской”.',
  'Проверить описание и цену товаров с низкой конверсией.',
];

const sellerSections: Array<{ id: string; label: string; icon: LucideIcon; description: string }> = [
  { id: 'finance', label: 'Финансы', icon: Wallet, description: 'единый расчёт прибыли' },
  { id: 'products', label: 'Товары', icon: Boxes, description: 'остатки, карточки, статусы' },
  { id: 'analytics', label: 'Аналитика', icon: BarChart3, description: 'воронка и спрос' },
  { id: 'promotion', label: 'Продвижение', icon: Megaphone, description: 'ставки и окупаемость' },
  { id: 'orders', label: 'Заказы', icon: ClipboardList, description: 'отгрузки и возвраты' },
  { id: 'settings', label: 'Магазин', icon: Store, description: 'выплаты, документы, команда' },
];

const productTasks = [
  {
    product: 'Слипоны мягкой формы',
    signal: 'Остаток 5 единиц',
    action: 'Создать заявку на поставку размеров 39–40',
    effect: 'Сохранить стабильные продажи и не потерять выдачу',
    icon: PackageCheck,
  },
  {
    product: 'Топ базовый асимметричный',
    signal: 'Высокие просмотры, низкие заказы',
    action: 'Переснять первое фото и переписать первые 180 символов описания',
    effect: 'Поднять конверсию карточки выше 3%',
    icon: Eye,
  },
  {
    product: 'Ремень с матовой пряжкой',
    signal: 'Реклама ест маржу',
    action: 'Снизить ставку на 18% и протестировать новый заголовок',
    effect: 'Вернуть продвижение к окупаемости',
    icon: Megaphone,
  },
];

const promotionRows = [
  { campaign: 'Анорак ледяной линии', spend: 18500, orders: 84, revenue: 1251600, roas: 67.7, status: 'Масштабировать' },
  { campaign: 'Топ базовый асимметричный', spend: 14200, orders: 19, revenue: 237500, roas: 16.7, status: 'Проверить карточку' },
  { campaign: 'Ремень с матовой пряжкой', spend: 6100, orders: 8, revenue: 46400, roas: 7.6, status: 'Снизить ставку' },
];

const sellerOrders = [
  { status: 'Новые', count: 48, detail: '18 нужно подтвердить до 14:00', icon: ShoppingBag },
  { status: 'В сборке', count: 76, detail: '5 заказов близко к дедлайну', icon: Boxes },
  { status: 'Доставляются', count: 214, detail: 'логистика в норме', icon: Truck },
  { status: 'Возвраты', count: 18, detail: '7 связаны с размерной сеткой', icon: ReceiptText },
];

const payoutRows = [
  { date: 'Сегодня', label: 'Доступно к выплате', amount: 182400 },
  { date: '29 апреля', label: 'Ожидаемая выплата', amount: 96300 },
  { date: '3 мая', label: 'После удержаний', amount: 59150 },
];

type StoreSettingId = 'profile' | 'documents' | 'payouts' | 'team';

type StoreSettingsState = {
  storeName: string;
  legalName: string;
  city: string;
  category: string;
  verificationStatus: string;
  taxId: string;
  renewalDate: string;
  documentOwner: string;
  payoutCard: string;
  payoutFrequency: string;
  payoutLimit: string;
  payoutContact: string;
  teamMembers: string;
  pendingRole: string;
  managerEmail: string;
  dailyDigest: boolean;
  riskAlerts: boolean;
};

const initialStoreSettings: StoreSettingsState = {
  storeName: 'ZAMK Selected Seller',
  legalName: 'ИП Морозова А. С.',
  city: 'Москва',
  category: 'Одежда и аксессуары',
  verificationStatus: 'Проверены',
  taxId: '7704 982314',
  renewalDate: 'через 42 дня',
  documentOwner: 'Анна Морозова',
  payoutCard: 'Карта **** 4592',
  payoutFrequency: 'еженедельно',
  payoutLimit: '50 000 ₽',
  payoutContact: 'finance@zamk.seller',
  teamMembers: '3 сотрудника',
  pendingRole: '1 роль на проверке',
  managerEmail: 'manager@zamk.seller',
  dailyDigest: true,
  riskAlerts: true,
};

const storeSettingGroups: Record<StoreSettingId, { title: string; icon: LucideIcon; description: string; fields: Array<{ key: keyof StoreSettingsState; label: string }> }> = {
  profile: {
    title: 'Профиль магазина',
    icon: Store,
    description: 'Название, юридическое лицо, город и основная категория продавца.',
    fields: [
      { key: 'storeName', label: 'Название магазина' },
      { key: 'legalName', label: 'Юридическое имя' },
      { key: 'city', label: 'Город отгрузки' },
      { key: 'category', label: 'Основная категория' },
    ],
  },
  documents: {
    title: 'Документы',
    icon: FileText,
    description: 'Статус проверки, ИНН, владелец документов и дата обновления.',
    fields: [
      { key: 'verificationStatus', label: 'Статус проверки' },
      { key: 'taxId', label: 'ИНН / налоговый номер' },
      { key: 'renewalDate', label: 'Следующее обновление' },
      { key: 'documentOwner', label: 'Ответственный' },
    ],
  },
  payouts: {
    title: 'Выплаты',
    icon: CreditCard,
    description: 'Карта, частота выплат, минимальная сумма и контакт финансов.',
    fields: [
      { key: 'payoutCard', label: 'Платёжный метод' },
      { key: 'payoutFrequency', label: 'Частота выплат' },
      { key: 'payoutLimit', label: 'Минимальная сумма' },
      { key: 'payoutContact', label: 'Финансовый контакт' },
    ],
  },
  team: {
    title: 'Доступы команды',
    icon: ShieldCheck,
    description: 'Команда, роли на проверке, управляющий и уведомления.',
    fields: [
      { key: 'teamMembers', label: 'Активные сотрудники' },
      { key: 'pendingRole', label: 'Роли на проверке' },
      { key: 'managerEmail', label: 'Главный менеджер' },
    ],
  },
};

function MiniStat({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <div className="rounded-2xl border border-white/70 bg-white/86 p-4 shadow-sm backdrop-blur-xl dark:border-white/18 dark:bg-black/28">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-graphite-light dark:text-white/58">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-graphite dark:text-white">{value}</p>
      <p className="mt-1 text-xs font-medium text-graphite-light dark:text-white/72">{hint}</p>
    </div>
  );
}

function SellerWorkspaceNav() {
  return (
    <section className="mt-6 grid gap-3 md:grid-cols-2 xl:grid-cols-6">
      {sellerSections.map((section) => {
        const Icon = section.icon;

        return (
          <a
            key={section.id}
            href={`#${section.id}`}
            className="rounded-2xl border border-border-lighter bg-white/78 p-4 text-left shadow-sm transition-all hover:-translate-y-0.5 hover:border-graphite/25 hover:bg-white dark:border-white/16 dark:bg-black/28 dark:hover:border-white/28 dark:hover:bg-white/10"
          >
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-graphite text-white dark:bg-white dark:text-black">
                <Icon className="h-4 w-4" />
              </span>
              <span className="text-sm font-semibold text-graphite dark:text-white">{section.label}</span>
            </div>
            <p className="mt-3 text-xs leading-relaxed text-graphite-light dark:text-white/64">{section.description}</p>
          </a>
        );
      })}
    </section>
  );
}

function MetricCard({ title, value, change, trend, icon: Icon }: (typeof metrics)[number]) {
  const isPositive = trend === 'up';

  return (
    <article className="glass-panel p-5 md:p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash">{title}</p>
          <p className="mt-3 text-3xl font-semibold tracking-normal text-graphite dark:text-white">{value}</p>
        </div>
        <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl border border-border-lighter bg-white/80 text-graphite-light dark:border-white/10 dark:bg-white/8 dark:text-white/76">
          <Icon className="h-5 w-5" />
        </div>
      </div>
      <div
        className={cn(
          'mt-5 inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium',
          isPositive
            ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300'
            : 'bg-red-50 text-red-700 dark:bg-red-400/10 dark:text-red-300'
        )}
      >
        {isPositive ? <ArrowUpRight className="h-3.5 w-3.5" /> : <ArrowDownRight className="h-3.5 w-3.5" />}
        {change}
      </div>
    </article>
  );
}

function DecisionLoop() {
  return (
    <section className="glass-panel-strong p-6 md:p-8">
      <p className="studio-label">Логика кабинета</p>
      <h2 className="studio-title mt-2">От сигнала к действию</h2>
      <div className="mt-7 grid gap-4 md:grid-cols-3">
        {[
          { title: 'Сигнал', text: 'Система подсвечивает деньги, спрос, остатки, рекламу и возвраты.', icon: AlertTriangle },
          { title: 'Причина', text: 'Каждый сигнал объясняет, где именно просадка: фото, цена, остаток, ставка или описание.', icon: Search },
          { title: 'Действие', text: 'Продавец получает конкретный следующий шаг и ожидаемый эффект.', icon: Zap },
        ].map((item) => {
          const Icon = item.icon;

          return (
            <article key={item.title} className="rounded-[2rem] border border-border-lighter bg-white/74 p-5 dark:border-white/16 dark:bg-black/26">
              <Icon className="h-5 w-5 text-ash" />
              <h3 className="mt-4 text-2xl font-serif text-graphite dark:text-white">{item.title}</h3>
              <p className="mt-3 text-sm leading-relaxed text-graphite-light dark:text-white/68">{item.text}</p>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function WeeklyRevenueChart() {
  const maxValue = Math.max(...revenueByDay.map((item) => item.value));

  return (
    <section className="glass-panel-strong p-6 md:p-8">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="studio-label">Неделя в деньгах</p>
          <h2 className="studio-title mt-2">Выручка за неделю</h2>
          <p className="studio-subtitle mt-2">Пик пришёлся на пятницу: карточки верхней одежды и обуви дали основной рост.</p>
        </div>
        <div className="inline-flex items-center gap-2 rounded-full border border-border-lighter bg-white/70 px-4 py-2 text-sm text-graphite dark:border-white/10 dark:bg-white/6 dark:text-white/80">
          <BarChart3 className="h-4 w-4 text-ash" />
          +14,8%
        </div>
      </div>

      <div className="mt-8 flex h-72 items-end gap-3 sm:gap-4">
        {revenueByDay.map((item) => {
          const height = Math.max(16, (item.value / maxValue) * 100);

          return (
            <div key={item.day} className="flex h-full min-w-0 flex-1 flex-col items-center justify-end gap-3">
              <div className="flex w-full flex-1 items-end rounded-full bg-ice/70 p-1.5 dark:bg-black/18">
                <div
                  className="w-full rounded-full bg-gradient-to-t from-graphite to-accent shadow-[0_14px_30px_rgba(95,112,133,0.18)] dark:from-white dark:to-accent"
                  style={{ height: `${height}%` }}
                  aria-label={`${item.day}: ${formatCurrency(item.value)}`}
                />
              </div>
              <div className="text-center">
                <p className="text-sm font-semibold text-graphite dark:text-white">{item.day}</p>
                <p className="mt-1 text-[11px] text-ash">{formatNumber(Math.round(item.value / 1000))}к</p>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function ProductActionBoard() {
  return (
    <section className="glass-panel-strong p-6 md:p-8">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="studio-label">Очередь карточек</p>
          <h2 className="studio-title mt-2">Что исправить в товарах</h2>
          <p className="studio-subtitle mt-2">Каждая задача связана с метрикой, а не с абстрактным советом.</p>
        </div>
        <Link
          to="/seller-products"
          className="inline-flex h-11 items-center justify-center gap-2 rounded-full border border-border-lighter bg-white/72 px-5 text-sm font-semibold text-graphite transition-colors hover:bg-white dark:border-white/16 dark:bg-white/8 dark:text-white dark:hover:bg-white/12"
        >
          <SlidersHorizontal className="h-4 w-4" />
          Управлять товарами
        </Link>
      </div>
      <div className="mt-7 grid gap-4 lg:grid-cols-3">
        {productTasks.map((task) => {
          const Icon = task.icon;

          return (
            <article key={task.product} className="rounded-[2rem] border border-border-lighter bg-white/74 p-5 dark:border-white/16 dark:bg-black/26">
              <div className="flex items-start justify-between gap-4">
                <Icon className="h-5 w-5 text-ash" />
                <span className="rounded-full bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-400/10 dark:text-amber-300">
                  {task.signal}
                </span>
              </div>
              <h3 className="mt-4 text-xl font-serif leading-tight text-graphite dark:text-white">{task.product}</h3>
              <p className="mt-3 text-sm leading-relaxed text-graphite dark:text-white/82">{task.action}</p>
              <p className="mt-3 text-xs leading-relaxed text-graphite-light dark:text-white/62">{task.effect}</p>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function BuyerFunnel() {
  const maxValue = funnel[0].value;

  return (
    <section className="glass-panel-strong p-6 md:p-8">
      <p className="studio-label">Где теряются покупатели</p>
      <h2 className="studio-title mt-2">Воронка покупателя</h2>
      <p className="studio-subtitle mt-2">Самая заметная просадка сейчас между переходом в карточку и добавлением в корзину.</p>

      <div className="mt-7 space-y-5">
        {funnel.map((item) => {
          const width = (item.value / maxValue) * 100;
          const Icon = item.icon;

          return (
            <div key={item.label}>
              <div className="mb-2 flex items-center justify-between gap-4 text-sm">
                <div className="flex items-center gap-2 text-graphite dark:text-white">
                  <Icon className="h-4 w-4 text-ash" />
                  <span className="font-medium">{item.label}</span>
                </div>
                <span className="text-graphite-light dark:text-white/70">{formatNumber(item.value)}</span>
              </div>
              <div className="h-3 overflow-hidden rounded-full bg-ice dark:bg-white/8">
                <div className="h-full rounded-full bg-graphite dark:bg-white" style={{ width: `${width}%` }} />
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function PromotionCenter() {
  return (
    <section id="promotion" className="glass-panel-strong scroll-mt-28 p-6 md:p-8">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="studio-label">Продвижение</p>
          <h2 className="studio-title mt-2">Куда уходит реклама</h2>
          <p className="studio-subtitle mt-2">Расходы показываются рядом с заказами и выручкой, чтобы быстро понять окупаемость.</p>
        </div>
        <div className="rounded-full border border-border-lighter bg-white/74 px-4 py-2 text-sm font-semibold text-graphite dark:border-white/16 dark:bg-black/26 dark:text-white">
          ROAS недели: 25,9
        </div>
      </div>
      <div className="mt-7 overflow-x-auto">
        <table className="min-w-[760px] w-full text-left text-sm">
          <thead>
            <tr className="border-b border-border-lighter text-[11px] uppercase tracking-[0.14em] text-ash dark:border-white/10">
              <th className="py-3 pr-4 font-semibold">Кампания</th>
              <th className="py-3 pr-4 font-semibold">Расход</th>
              <th className="py-3 pr-4 font-semibold">Заказы</th>
              <th className="py-3 pr-4 font-semibold">Выручка</th>
              <th className="py-3 pr-4 font-semibold">ROAS</th>
              <th className="py-3 font-semibold">Решение</th>
            </tr>
          </thead>
          <tbody>
            {promotionRows.map((row) => (
              <tr key={row.campaign} className="border-b border-border-lighter/70 last:border-b-0 dark:border-white/8">
                <td className="py-4 pr-4 font-medium text-graphite dark:text-white">{row.campaign}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatCurrency(row.spend)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.orders)}</td>
                <td className="py-4 pr-4 text-graphite dark:text-white">{formatCurrency(row.revenue)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{row.roas.toFixed(1)}</td>
                <td className="py-4 text-graphite dark:text-white">{row.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function AlertCard({ title, description, tone }: (typeof alerts)[number]) {
  const toneClasses = {
    warning: 'border-amber-200/70 bg-amber-50/70 text-amber-700 dark:border-amber-300/15 dark:bg-amber-300/8 dark:text-amber-200',
    success: 'border-emerald-200/70 bg-emerald-50/70 text-emerald-700 dark:border-emerald-300/15 dark:bg-emerald-300/8 dark:text-emerald-200',
    danger: 'border-red-200/70 bg-red-50/70 text-red-700 dark:border-red-300/15 dark:bg-red-300/8 dark:text-red-200',
  };

  return (
    <article className="rounded-[2rem] border border-border-lighter bg-white/74 p-5 shadow-sm backdrop-blur-xl dark:border-white/16 dark:bg-black/26">
      <div className={cn('mb-4 flex h-10 w-10 items-center justify-center rounded-2xl border', toneClasses[tone as keyof typeof toneClasses])}>
        <AlertTriangle className="h-5 w-5" />
      </div>
      <h3 className="text-xl font-serif leading-tight text-graphite dark:text-white">{title}</h3>
      <p className="mt-3 text-sm leading-relaxed text-graphite-light dark:text-white/68">{description}</p>
    </article>
  );
}

function OrdersControl() {
  return (
    <section id="orders" className="glass-panel-strong scroll-mt-28 p-6 md:p-8">
      <p className="studio-label">Операции</p>
      <h2 className="studio-title mt-2">Заказы и отгрузки</h2>
      <div className="mt-7 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {sellerOrders.map((item) => {
          const Icon = item.icon;

          return (
            <article key={item.status} className="rounded-[2rem] border border-border-lighter bg-white/74 p-5 dark:border-white/16 dark:bg-black/26">
              <Icon className="h-5 w-5 text-ash" />
              <p className="mt-4 text-[11px] font-semibold uppercase tracking-[0.14em] text-ash">{item.status}</p>
              <p className="mt-2 text-3xl font-semibold text-graphite dark:text-white">{formatNumber(item.count)}</p>
              <p className="mt-2 text-sm leading-relaxed text-graphite-light dark:text-white/68">{item.detail}</p>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function PayoutsPanel() {
  return (
    <section className="glass-panel-strong p-6 md:p-8">
      <p className="studio-label">Выплаты</p>
      <h2 className="studio-title mt-2">Деньги по датам</h2>
      <div className="mt-7 space-y-3">
        {payoutRows.map((row) => (
          <div key={row.date} className="flex items-center justify-between gap-4 rounded-2xl border border-border-lighter bg-white/74 p-4 dark:border-white/16 dark:bg-black/26">
            <div>
              <p className="text-sm font-semibold text-graphite dark:text-white">{row.date}</p>
              <p className="mt-1 text-xs text-graphite-light dark:text-white/64">{row.label}</p>
            </div>
            <p className="text-lg font-semibold text-graphite dark:text-white">{formatCurrency(row.amount)}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    Растёт: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300',
    'Слабая карточка': 'bg-amber-50 text-amber-700 dark:bg-amber-400/10 dark:text-amber-300',
    Пополнить: 'bg-sky-50 text-sky-700 dark:bg-sky-400/10 dark:text-sky-300',
    Проблема: 'bg-red-50 text-red-700 dark:bg-red-400/10 dark:text-red-300',
  };

  return (
    <span className={cn('inline-flex rounded-full px-3 py-1 text-xs font-semibold', styles[status] || 'bg-ice text-graphite dark:bg-white/10 dark:text-white')}>
      {status}
    </span>
  );
}

function SearchDemandTable() {
  return (
    <section className="glass-panel-strong p-6 md:p-8">
      <div className="flex items-start gap-3">
        <Search className="mt-1 h-5 w-5 text-ash" />
        <div>
          <p className="studio-label">Спрос</p>
          <h2 className="studio-title mt-2">Поисковые запросы</h2>
          <p className="studio-subtitle mt-2">Запросы показывают, где стоит усиливать рекламу, а где карточка не убеждает после перехода.</p>
        </div>
      </div>

      <div className="mt-6 overflow-x-auto">
        <table className="min-w-[720px] w-full text-left text-sm">
          <thead>
            <tr className="border-b border-border-lighter text-[11px] uppercase tracking-[0.14em] text-ash dark:border-white/10">
              <th className="py-3 pr-4 font-semibold">Запрос</th>
              <th className="py-3 pr-4 font-semibold">Показы</th>
              <th className="py-3 pr-4 font-semibold">Переходы</th>
              <th className="py-3 pr-4 font-semibold">Заказы</th>
              <th className="py-3 pr-4 font-semibold">Конверсия</th>
              <th className="py-3 font-semibold">Вывод</th>
            </tr>
          </thead>
          <tbody>
            {searchRows.map((row) => (
              <tr key={row.query} className="border-b border-border-lighter/70 last:border-b-0 dark:border-white/8">
                <td className="py-4 pr-4 font-medium text-graphite dark:text-white">{row.query}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.impressions)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.clicks)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.orders)}</td>
                <td className="py-4 pr-4 text-graphite dark:text-white">{formatPercent(row.conversion)}</td>
                <td className="py-4 text-graphite dark:text-white">{row.insight}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function ProductAnalyticsTable() {
  return (
    <section id="products" className="glass-panel-strong scroll-mt-28 p-6 md:p-8">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="studio-label">Товары</p>
          <h2 className="studio-title mt-2">Аналитика по карточкам</h2>
          <p className="studio-subtitle mt-2">Здесь видно, какие позиции масштабировать, а какие требуют фото, цены или рекламной ставки.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link
            to="/seller-products"
            className="inline-flex h-11 items-center justify-center gap-2 rounded-full border border-border-lighter bg-white/72 px-5 text-sm font-semibold text-graphite transition-colors hover:bg-white dark:border-white/16 dark:bg-white/8 dark:text-white dark:hover:bg-white/12"
          >
            <Boxes className="h-4 w-4" />
            Открыть ассортимент
          </Link>
          <Link
            to="/seller-products/new"
            className="inline-flex h-11 items-center justify-center rounded-full bg-graphite px-5 text-sm font-semibold text-white transition-colors hover:bg-graphite-light dark:bg-white dark:text-black dark:hover:bg-white/86"
          >
            Добавить товар
          </Link>
        </div>
      </div>

      <div className="mt-7 overflow-x-auto">
        <table className="min-w-[1060px] w-full text-left text-sm">
          <thead>
            <tr className="border-b border-border-lighter text-[11px] uppercase tracking-[0.14em] text-ash dark:border-white/10">
              <th className="py-3 pr-4 font-semibold">Товар</th>
              <th className="py-3 pr-4 font-semibold">Просмотры</th>
              <th className="py-3 pr-4 font-semibold">CTR</th>
              <th className="py-3 pr-4 font-semibold">Заказы</th>
              <th className="py-3 pr-4 font-semibold">Выручка</th>
              <th className="py-3 pr-4 font-semibold">Реклама</th>
              <th className="py-3 pr-4 font-semibold">Остаток</th>
              <th className="py-3 pr-4 font-semibold">Рейтинг</th>
              <th className="py-3 pr-4 font-semibold">Возвраты</th>
              <th className="py-3 font-semibold">Статус</th>
            </tr>
          </thead>
          <tbody>
            {productRows.map((row) => (
              <tr key={row.product} className="border-b border-border-lighter/70 last:border-b-0 dark:border-white/8">
                <td className="py-4 pr-4 font-medium text-graphite dark:text-white">{row.product}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.views)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatPercent(row.ctr)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.orders)}</td>
                <td className="py-4 pr-4 text-graphite dark:text-white">{formatCurrency(row.revenue)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatCurrency(row.ads)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.stock)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{row.rating.toFixed(1)}</td>
                <td className="py-4 pr-4 text-graphite-light dark:text-white/68">{formatNumber(row.returns)}</td>
                <td className="py-4"><StatusBadge status={row.status} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function FinancialTransparency() {
  const revenue = financeRows[0].value;

  return (
    <section id="finance" className="glass-panel-strong scroll-mt-28 p-6 md:p-8">
      <div className="grid gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-center">
        <div>
          <p className="studio-label">Расчёт без разрывов</p>
          <h2 className="studio-title mt-2">Финансовая прозрачность</h2>
          <p className="studio-subtitle mt-3">
            Все удержания собраны в одну цепочку: продавец видит, почему выручка и чистая прибыль отличаются, и где можно управлять расходами.
          </p>
          <div className="mt-6 rounded-[2rem] border border-border-lighter bg-white/72 p-5 backdrop-blur-xl dark:border-white/16 dark:bg-black/26">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash">Чистая прибыль</p>
            <p className="mt-2 text-4xl font-semibold text-graphite dark:text-white">{formatCurrency(337850)}</p>
          </div>
        </div>

        <div className="space-y-3">
          {financeRows.map((row) => {
            const width = Math.max(8, Math.abs(row.value / revenue) * 100);
            const isProfit = row.type === 'profit';
            const isIncome = row.type === 'income';

            return (
              <div key={row.label} className="rounded-2xl border border-border-lighter bg-white/72 p-4 backdrop-blur-xl dark:border-white/16 dark:bg-black/26">
                <div className="flex items-center justify-between gap-4 text-sm">
                  <span className={cn('font-medium', isProfit ? 'text-graphite dark:text-white' : 'text-graphite-light dark:text-white/70')}>{row.label}</span>
                  <span className={cn('font-semibold', row.value < 0 ? 'text-red-600 dark:text-red-300' : 'text-graphite dark:text-white')}>
                    {formatCurrency(row.value)}
                  </span>
                </div>
                <div className="mt-3 h-2 overflow-hidden rounded-full bg-ice dark:bg-white/8">
                  <div
                    className={cn(
                      'h-full rounded-full',
                      isProfit ? 'bg-emerald-500 dark:bg-emerald-300' : isIncome ? 'bg-graphite dark:bg-white' : 'bg-red-400 dark:bg-red-300'
                    )}
                    style={{ width: `${width}%` }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function NextSteps() {
  return (
    <section id="recommendations" className="glass-panel-strong scroll-mt-28 p-6 md:p-8">
      <p className="studio-label">Решения на сегодня</p>
      <h2 className="studio-title mt-2">Что делать дальше</h2>
      <div className="mt-7 grid gap-3 md:grid-cols-2">
        {recommendations.map((item, index) => (
          <div key={item} className="flex gap-3 rounded-2xl border border-border-lighter bg-white/72 p-4 backdrop-blur-xl dark:border-white/16 dark:bg-black/26">
            <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-graphite text-xs font-semibold text-white dark:bg-white dark:text-black">
              {index + 1}
            </span>
            <p className="text-sm leading-relaxed text-graphite dark:text-white/82">{item}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function SettingInput({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="block">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="seller-setting-input mt-2 h-12 w-full rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none transition-all focus:border-graphite/30 focus:bg-white dark:border-white/16 dark:bg-black/24 dark:text-white dark:focus:border-white/32 dark:focus:bg-black/32"
      />
    </label>
  );
}

function SettingToggle({
  checked,
  label,
  description,
  onChange,
}: {
  checked: boolean;
  label: string;
  description: string;
  onChange: (value: boolean) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className="flex w-full items-center justify-between gap-4 rounded-2xl border border-border-lighter bg-white/72 p-4 text-left transition-all hover:border-graphite/25 dark:border-white/16 dark:bg-black/24 dark:hover:border-white/28"
    >
      <span>
        <span className="block text-sm font-semibold text-graphite dark:text-white">{label}</span>
        <span className="mt-1 block text-xs leading-relaxed text-graphite-light dark:text-white/62">{description}</span>
      </span>
      <span className={cn('flex h-7 w-12 shrink-0 items-center rounded-full p-1 transition-colors', checked ? 'bg-graphite dark:bg-white' : 'bg-ash-light/60 dark:bg-white/14')}>
        <span className={cn('h-5 w-5 rounded-full bg-white shadow-sm transition-transform dark:bg-black', checked && 'translate-x-5')} />
      </span>
    </button>
  );
}

function getSettingSummary(settings: StoreSettingsState, id: StoreSettingId) {
  const summaries: Record<StoreSettingId, string> = {
    profile: `${settings.storeName}, ${settings.city}`,
    documents: `${settings.verificationStatus}, обновление ${settings.renewalDate}`,
    payouts: `${settings.payoutCard}, ${settings.payoutFrequency}`,
    team: `${settings.teamMembers}, ${settings.pendingRole}`,
  };

  return summaries[id];
}

function StoreSettingsPanel() {
  const [settings, setSettings] = useState(initialStoreSettings);
  const [draft, setDraft] = useState(initialStoreSettings);
  const [activeId, setActiveId] = useState<StoreSettingId>('profile');
  const [savedSection, setSavedSection] = useState<StoreSettingId | null>(null);
  const activeGroup = storeSettingGroups[activeId];
  const ActiveIcon = activeGroup.icon;

  const updateDraft = (key: keyof StoreSettingsState, value: string | boolean) => {
    setDraft((current) => ({ ...current, [key]: value }));
    setSavedSection(null);
  };

  const selectSection = (id: StoreSettingId) => {
    setActiveId(id);
    setDraft(settings);
    setSavedSection(null);
  };

  const saveSettings = () => {
    setSettings(draft);
    setSavedSection(activeId);
  };

  const resetSettings = () => {
    setDraft(settings);
    setSavedSection(null);
  };

  return (
    <section id="settings" className="glass-panel-strong scroll-mt-28 p-6 md:p-8">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="studio-label">Настройки магазина</p>
          <h2 className="studio-title mt-2">Контур управления</h2>
        </div>
        <div className="inline-flex items-center gap-2 rounded-full border border-border-lighter bg-white/74 px-4 py-2 text-xs font-semibold text-graphite dark:border-white/16 dark:bg-black/26 dark:text-white">
          <Users className="h-4 w-4 text-ash" />
          Настройки сохраняются локально
        </div>
      </div>

      <div className="mt-7 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {(Object.keys(storeSettingGroups) as StoreSettingId[]).map((id) => {
          const item = storeSettingGroups[id];
          const Icon = item.icon;
          const isActive = activeId === id;

          return (
            <button
              key={id}
              type="button"
              onClick={() => selectSection(id)}
              className={cn(
                'flex min-h-[132px] gap-4 rounded-[2rem] border bg-white/74 p-5 text-left transition-all hover:-translate-y-0.5 dark:bg-black/26',
                isActive
                  ? 'border-graphite/30 shadow-sm dark:border-white/34 dark:bg-white/10'
                  : 'border-border-lighter hover:border-graphite/20 dark:border-white/16 dark:hover:border-white/28'
              )}
            >
              <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-ice text-graphite dark:bg-white/10 dark:text-white">
                <Icon className="h-5 w-5" />
              </span>
              <span className="min-w-0">
                <span className="flex items-center gap-2 text-lg font-serif leading-tight text-graphite dark:text-white">
                  {item.title}
                  {savedSection === id && <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-300" />}
                </span>
                <span className="mt-2 block text-sm leading-relaxed text-graphite-light dark:text-white/68">{getSettingSummary(settings, id)}</span>
              </span>
            </button>
          );
        })}
      </div>

      <div className="mt-5 rounded-[2rem] border border-border-lighter bg-white/72 p-5 dark:border-white/16 dark:bg-black/26 md:p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex max-w-3xl gap-4">
            <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-graphite text-white dark:bg-white dark:text-black">
              <ActiveIcon className="h-5 w-5" />
            </span>
            <div>
              <h3 className="text-3xl font-serif leading-tight text-graphite dark:text-white">{activeGroup.title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-graphite-light dark:text-white/68">{activeGroup.description}</p>
            </div>
          </div>
          <span className="inline-flex w-fit items-center gap-2 rounded-full bg-ice px-3 py-1.5 text-xs font-semibold text-graphite-light dark:bg-white/10 dark:text-white/68">
            <Edit3 className="h-3.5 w-3.5" />
            Редактирование
          </span>
        </div>

        <div className="mt-7 grid gap-4 md:grid-cols-2">
          {activeGroup.fields.map((field) => (
            <SettingInput
              key={field.key}
              label={field.label}
              value={String(draft[field.key])}
              onChange={(value) => updateDraft(field.key, value)}
            />
          ))}
        </div>

        {activeId === 'team' && (
          <div className="mt-5 grid gap-3 lg:grid-cols-2">
            <SettingToggle
              checked={draft.dailyDigest}
              label="Ежедневная утренняя сводка"
              description="Присылать менеджеру краткий список денег, рисков и задач."
              onChange={(value) => updateDraft('dailyDigest', value)}
            />
            <SettingToggle
              checked={draft.riskAlerts}
              label="Срочные тревожные сигналы"
              description="Подсвечивать падение рейтинга, остатки и рекламу без окупаемости."
              onChange={(value) => updateDraft('riskAlerts', value)}
            />
          </div>
        )}

        <div className="mt-6 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <p className="max-w-xl text-xs leading-relaxed text-graphite-light dark:text-white/58">
            {savedSection === activeId ? 'Изменения сохранены в моковом состоянии кабинета.' : 'Изменения появятся в карточках после сохранения.'}
          </p>
          <div className="flex flex-col gap-2 sm:flex-row">
            <button
              type="button"
              onClick={resetSettings}
              className="h-11 rounded-full border border-border-lighter bg-white/70 px-5 text-sm font-semibold text-graphite transition-colors hover:bg-white dark:border-white/16 dark:bg-white/8 dark:text-white dark:hover:bg-white/12"
            >
              Сбросить
            </button>
            <button
              type="button"
              onClick={saveSettings}
              className="h-11 rounded-full bg-graphite px-5 text-sm font-semibold text-white transition-colors hover:bg-graphite-light dark:bg-white dark:text-black dark:hover:bg-white/86"
            >
              Сохранить
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}

export function SellerDashboard() {
  return (
    <div className="relative z-10 min-h-screen pt-24 pb-24 md:pt-28 md:pb-20">
      <div className="container mx-auto max-w-[1400px] px-4 sm:px-6">
        <section className="glass-panel-strong overflow-hidden p-7 md:p-10 lg:p-12">
          <div className="grid gap-8 lg:grid-cols-[1.05fr_0.95fr] lg:items-center">
            <div>
              <p className="studio-label">Кабинет продавца</p>
              <h1 className="mt-4 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-6xl">
                С добрым утром, чемпион
              </h1>
              <p className="mt-5 max-w-3xl text-base leading-relaxed text-graphite-light dark:text-white/72 md:text-lg">
                За время отсутствия: 48 новых заказов, 7 товаров добавили в избранное, 2 карточки требуют внимания, 1 популярный размер заканчивается.
              </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              {miniStats.map((stat) => (
                <MiniStat key={stat.label} {...stat} />
              ))}
            </div>
          </div>
        </section>

        <SellerWorkspaceNav />

        <section className="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {metrics.map((metric) => (
            <MetricCard key={metric.title} {...metric} />
          ))}
        </section>

        <div className="mt-6">
          <DecisionLoop />
        </div>

        <div id="analytics" className="mt-6 grid scroll-mt-28 gap-6 lg:grid-cols-2">
          <WeeklyRevenueChart />
          <BuyerFunnel />
        </div>

        <div className="mt-6 grid gap-6 lg:grid-cols-[0.9fr_1.1fr]">
          <section className="glass-panel-strong p-6 md:p-8">
            <p className="studio-label">Тревожные сигналы</p>
            <h2 className="studio-title mt-2">Что просит внимания</h2>
            <div className="mt-7 grid gap-4">
              {alerts.map((alert) => (
                <AlertCard key={alert.title} {...alert} />
              ))}
            </div>
          </section>
          <SearchDemandTable />
        </div>

        <div className="mt-6">
          <ProductAnalyticsTable />
        </div>

        <div className="mt-6">
          <ProductActionBoard />
        </div>

        <div className="mt-6">
          <PromotionCenter />
        </div>

        <div className="mt-6">
          <OrdersControl />
        </div>

        <div className="mt-6 grid gap-6 lg:grid-cols-[1.05fr_0.95fr]">
          <FinancialTransparency />
          <PayoutsPanel />
        </div>

        <div className="mt-6">
          <NextSteps />
        </div>

        <div className="mt-6">
          <StoreSettingsPanel />
        </div>

        <section className="mt-6 rounded-[2rem] border border-border-lighter bg-white/82 p-5 text-sm leading-relaxed text-graphite-light dark:border-white/16 dark:bg-black/30 dark:text-white/70">
          <CheckCircle2 className="mb-3 h-5 w-5 text-emerald-600 dark:text-emerald-300" />
          Кабинет построен вокруг ежедневного цикла продавца: утром увидеть деньги и риски, днём закрыть операционные задачи, вечером проверить окупаемость рекламы и подготовить карточки к следующему дню.
          <Link to="/catalog" className="ml-1 font-semibold text-graphite hover:underline dark:text-white">
            Вернуться к витрине
          </Link>
          .
        </section>
      </div>
    </div>
  );
}
