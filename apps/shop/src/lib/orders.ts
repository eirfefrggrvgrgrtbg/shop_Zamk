import type { Product } from './mock-data';

export interface OrderItem {
  id: string;
  name: string;
  size?: string;
  color?: string;
  quantity?: number;
  price: number;
  image: string;
}

export interface OrderDelivery {
  address: string;
  service: string;
}

export interface OrderRecord {
  id: string;
  date: string;
  status: string;
  statusColor?: string;
  total: number;
  delivery: OrderDelivery;
  deliveryCost?: number;
  paymentMethod?: string;
  items: OrderItem[];
}

export interface CreateOrderOptions {
  deliveryCost: number;
  deliveryService: string;
  paymentMethod: string;
}

export const ORDER_STORAGE_KEY = 'zamk_orders';
export const ORDER_STORAGE_EVENT = 'zamk-orders-updated';

const DATE_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
};

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function normalizeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : '';
}

function normalizeNumber(value: unknown, fallback: number) {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function normalizeOrderItem(value: unknown): OrderItem | null {
  if (!isObject(value)) {
    return null;
  }

  const id = normalizeText(value.id);
  const name = normalizeText(value.name);
  const image = normalizeText(value.image);
  const price = normalizeNumber(value.price, NaN);

  if (!id || !name || !image || Number.isNaN(price)) {
    return null;
  }

  return {
    id,
    name,
    size: normalizeText(value.size) || undefined,
    color: normalizeText(value.color) || undefined,
    quantity: normalizeNumber(value.quantity, 1),
    price,
    image,
  };
}

function normalizeOrderRecord(value: unknown): OrderRecord | null {
  if (!isObject(value)) {
    return null;
  }

  const id = normalizeText(value.id);
  const date = normalizeText(value.date);
  const status = normalizeText(value.status);
  const total = normalizeNumber(value.total, NaN);
  const items = Array.isArray(value.items) ? value.items.map(normalizeOrderItem).filter((item): item is OrderItem => Boolean(item)) : [];

  if (!id || !date || !status || Number.isNaN(total)) {
    return null;
  }

  const deliveryValue = isObject(value.delivery) ? value.delivery : null;

  return {
    id,
    date,
    status,
    statusColor: normalizeText(value.statusColor) || undefined,
    total,
    delivery: {
      address: normalizeText(deliveryValue?.address) || '',
      service: normalizeText(deliveryValue?.service) || '',
    },
    deliveryCost: typeof value.deliveryCost === 'number' ? value.deliveryCost : undefined,
    paymentMethod: normalizeText(value.paymentMethod) || undefined,
    items,
  };
}

export function getProductEffectivePrice(product: Pick<Product, 'price' | 'discountPrice'>) {
  return product.discountPrice ?? product.price;
}

export function getCartLineTotal(item: any) {
  return getProductEffectivePrice(item.product) * item.quantity;
}

export function formatOrderDate(date: Date = new Date()) {
  return date.toLocaleDateString('ru-RU', DATE_FORMAT_OPTIONS);
}

function buildOrderId() {
  const timestamp = Date.now().toString(36).toUpperCase();
  const randomPart = Math.random().toString(36).slice(2, 5).toUpperCase();
  return `ZMK-${timestamp}-${randomPart}`;
}

function readStoredOrders(): OrderRecord[] {
  if (typeof window === 'undefined') {
    return [];
  }

  try {
    const raw = window.localStorage.getItem(ORDER_STORAGE_KEY);
    if (!raw) {
      return [];
    }

    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed.map(normalizeOrderRecord).filter((order): order is OrderRecord => Boolean(order));
  } catch {
    return [];
  }
}

export function getOrdersWithDefaults(defaultOrders: OrderRecord[]) {
  const storedOrders = readStoredOrders();
  const knownIds = new Set(defaultOrders.map((order) => order.id));

  return [...defaultOrders, ...storedOrders.filter((order) => !knownIds.has(order.id))];
}

export function createOrderFromCart(items: any[], options: CreateOrderOptions): OrderRecord {
  const orderItems = items.map((item) => ({
    id: item.product.id,
    name: item.product.name,
    size: item.selectedSize,
    color: item.selectedColor,
    quantity: item.quantity,
    price: getCartLineTotal(item),
    image: item.product.images?.[0] ?? item.product.image,
  }));

  const subtotal = orderItems.reduce((sum, item) => sum + item.price, 0);

  return {
    id: buildOrderId(),
    date: formatOrderDate(),
    status: 'Принят',
    total: subtotal + options.deliveryCost,
    deliveryCost: options.deliveryCost,
    delivery: {
      address: 'Адрес будет подтверждён после обработки',
      service: options.deliveryService,
    },
    paymentMethod: options.paymentMethod,
    items: orderItems,
  };
}

export function saveOrder(order: OrderRecord) {
  if (typeof window === 'undefined') {
    return false;
  }

  try {
    const storedOrders = readStoredOrders();
    const nextOrders = storedOrders.some((item) => item.id === order.id)
      ? storedOrders.map((item) => (item.id === order.id ? order : item))
      : [order, ...storedOrders];

    window.localStorage.setItem(ORDER_STORAGE_KEY, JSON.stringify(nextOrders));
    window.dispatchEvent(new Event(ORDER_STORAGE_EVENT));
    return true;
  } catch {
    return false;
  }
}