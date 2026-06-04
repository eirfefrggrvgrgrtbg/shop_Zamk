export interface User {
  id: string;
  name: string;
  email: string;
  role: 'admin' | 'moderator' | 'support' | 'customer';
  status: 'active' | 'blocked' | 'unverified';
  createdAt: string;
}

export interface Seller {
  id: string;
  brandName: string;
  contactEmail: string;
  status: 'active' | 'pending' | 'suspended';
  productsCount: number;
  pendingProductsCount: number;
  totalSales: number;
}

export interface Product {
  id: string;
  title: string;
  sellerName: string;
  status: 'published' | 'moderation' | 'rejected' | 'archived';
  price: number;
  stock: number;
  category: string;
  createdAt: string;
}

export interface Order {
  id: string;
  customerName: string;
  status: 'pending' | 'processing' | 'shipped' | 'delivered' | 'cancelled';
  total: number;
  paymentStatus: 'paid' | 'unpaid' | 'refunded';
  createdAt: string;
}

export interface InventoryItem {
  id: string;
  productTitle: string;
  sellerName: string;
  variant: string;
  totalStock: number;
  reservedStock: number;
  availableStock: number;
}

export interface ReturnRequest {
  id: string;
  orderId: string;
  customerName: string;
  status: 'pending' | 'approved' | 'rejected' | 'completed';
  reason: string;
  createdAt: string;
}

export interface Payout {
  id: string;
  sellerName: string;
  amount: number;
  status: 'pending' | 'processing' | 'paid' | 'failed';
  requestedAt: string;
}

export interface AuditLog {
  id: string;
  actor: string;
  action: string;
  entity: string;
  createdAt: string;
}

// MOCK DATA

export const mockUsers: User[] = [
  { id: 'u1', name: 'Ivan Ivanov', email: 'ivan@example.com', role: 'customer', status: 'active', createdAt: '2026-05-10T10:00:00Z' },
  { id: 'u2', name: 'Admin One', email: 'admin@zamk.com', role: 'admin', status: 'active', createdAt: '2026-01-01T10:00:00Z' },
  { id: 'u3', name: 'Support Bob', email: 'bob@zamk.com', role: 'support', status: 'active', createdAt: '2026-02-15T12:30:00Z' },
  { id: 'u4', name: 'Spammer Guy', email: 'spam@example.com', role: 'customer', status: 'blocked', createdAt: '2026-06-01T08:15:00Z' },
];

export const mockSellers: Seller[] = [
  { id: 's1', brandName: 'ZAMK Selected', contactEmail: 'brand1@example.com', status: 'active', productsCount: 150, pendingProductsCount: 2, totalSales: 2500000 },
  { id: 's2', brandName: 'Cool Streetwear', contactEmail: 'contact@coolstreet.com', status: 'active', productsCount: 45, pendingProductsCount: 0, totalSales: 450000 },
  { id: 's3', brandName: 'New Brand', contactEmail: 'hello@newbrand.com', status: 'pending', productsCount: 0, pendingProductsCount: 5, totalSales: 0 },
];

export const mockProducts: Product[] = [
  { id: 'p1', title: 'Basic White Tee', sellerName: 'ZAMK Selected', status: 'published', price: 2500, stock: 120, category: 'T-Shirts', createdAt: '2026-05-20T10:00:00Z' },
  { id: 'p2', title: 'Black Denim Jacket', sellerName: 'Cool Streetwear', status: 'published', price: 12500, stock: 15, category: 'Outerwear', createdAt: '2026-05-21T11:00:00Z' },
  { id: 'p3', title: 'Oversized Hoodie', sellerName: 'New Brand', status: 'moderation', price: 6800, stock: 50, category: 'Hoodies', createdAt: '2026-06-02T09:00:00Z' },
];

export const mockOrders: Order[] = [
  { id: 'ORD-001', customerName: 'Ivan Ivanov', status: 'delivered', total: 15000, paymentStatus: 'paid', createdAt: '2026-05-25T14:30:00Z' },
  { id: 'ORD-002', customerName: 'Anna Smith', status: 'processing', total: 6800, paymentStatus: 'paid', createdAt: '2026-06-03T09:15:00Z' },
  { id: 'ORD-003', customerName: 'John Doe', status: 'pending', total: 2500, paymentStatus: 'unpaid', createdAt: '2026-06-03T11:00:00Z' },
];

export const mockInventory: InventoryItem[] = [
  { id: 'inv1', productTitle: 'Basic White Tee', sellerName: 'ZAMK Selected', variant: 'Size M', totalStock: 50, reservedStock: 5, availableStock: 45 },
  { id: 'inv2', productTitle: 'Basic White Tee', sellerName: 'ZAMK Selected', variant: 'Size L', totalStock: 70, reservedStock: 2, availableStock: 68 },
  { id: 'inv3', productTitle: 'Black Denim Jacket', sellerName: 'Cool Streetwear', variant: 'Size S', totalStock: 15, reservedStock: 10, availableStock: 5 },
];

export const mockReturns: ReturnRequest[] = [
  { id: 'RET-001', orderId: 'ORD-001', customerName: 'Ivan Ivanov', status: 'pending', reason: 'Wrong size', createdAt: '2026-06-01T10:00:00Z' },
];

export const mockPayouts: Payout[] = [
  { id: 'PAY-001', sellerName: 'ZAMK Selected', amount: 450000, status: 'pending', requestedAt: '2026-06-01T12:00:00Z' },
  { id: 'PAY-002', sellerName: 'Cool Streetwear', amount: 85000, status: 'paid', requestedAt: '2026-05-15T09:00:00Z' },
];

export const mockAuditLogs: AuditLog[] = [
  { id: 'log1', actor: 'Admin One', action: 'Approved Product', entity: 'p1', createdAt: '2026-05-20T10:05:00Z' },
  { id: 'log2', actor: 'Support Bob', action: 'Blocked User', entity: 'u4', createdAt: '2026-06-01T08:16:00Z' },
  { id: 'log3', actor: 'Admin One', action: 'Processed Payout', entity: 'PAY-002', createdAt: '2026-05-16T10:00:00Z' },
];
