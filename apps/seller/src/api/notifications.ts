import { request } from '@zamk/api-client/src/client';

export interface Notification {
  id: string;
  type: string;
  kind?: string;
  severity?: string;
  status?: string;
  title: string;
  body: string;
  entityType: string;
  entityId: string;
  actionUrl?: string;
  dedupeKey?: string;
  metadata?: Record<string, unknown> | null;
  readAt: string | null;
  resolvedAt?: string | null;
  createdAt: string;
}

export const notificationsApi = {
  getNotifications: async (limit: number = 20, offset: number = 0) => {
    return request<{items: Notification[], totalCount: number}>('GET', '/seller/notifications', { params: { limit, offset } });
  },
  getUnreadCount: async () => {
    const data = await request<{unreadCount: number}>('GET', '/seller/notifications/unread-count');
    return data.unreadCount;
  },
  markRead: async (id: string) => {
    return request<void>('POST', `/seller/notifications/${id}/read`);
  },
  markAllRead: async () => {
    return request<void>('POST', '/seller/notifications/read-all');
  }
};
