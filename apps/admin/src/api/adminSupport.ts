import { request } from '@zamk/api-client/src/client';

export type SupportRequesterType = 'CUSTOMER' | 'SELLER';
export type SupportSessionStatus = 'ACTIVE' | 'COMPLETED';
export type SupportSessionPriority = 'NORMAL' | 'HIGH' | 'URGENT';
export type SupportSenderType = 'CUSTOMER' | 'SELLER' | 'STAFF';
export type SupportContextType = 'ORDER' | 'RETURN' | 'PRODUCT';

export interface SupportCategory {
  id: string;
  name: string;
  requesterScope: 'CUSTOMER' | 'SELLER' | 'BOTH';
  active: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
}

export interface SupportSession {
  id: string;
  conversationId: string;
  status: SupportSessionStatus;
  priority: SupportSessionPriority;
  categoryId?: string;
  categoryName?: string;
  assignedTo?: string;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface SupportAttachment {
  id: string;
  messageId: string;
  contentType: string;
  sizeBytes: number;
  originalFilename?: string;
  sortOrder: number;
  createdAt: string;
}

export interface SupportContextLink {
  id: string;
  messageId: string;
  contextType: SupportContextType;
  contextId: string;
  label?: string;
  createdAt: string;
}

export interface SupportMessage {
  id: string;
  sessionId: string;
  senderType: SupportSenderType;
  senderUserId: string;
  textContent: string;
  createdAt: string;
  attachments: SupportAttachment[];
  contextLinks: SupportContextLink[];
}

export interface SupportInternalNote {
  id: string;
  sessionId: string;
  authorUserId: string;
  authorName?: string;
  textContent: string;
  createdAt: string;
}

export interface SupportConversation {
  id: string;
  requesterType: SupportRequesterType;
  requesterUserId?: string;
  requesterSellerId?: string;
  createdAt: string;
  updatedAt: string;
  activeSession?: SupportSession;
  unreadCount: number;
  requesterName?: string;
  requesterEmail?: string;
  requesterPhone?: string;
  requesterStoreName?: string;
  latestMessageText?: string;
  latestMessageAt?: string;
}

export interface SupportConversationDetail {
  conversation: SupportConversation;
  messages: SupportMessage[];
  internalNotes?: SupportInternalNote[];
}

export interface UpdateSupportSessionRequest {
  priority?: SupportSessionPriority;
  categoryId?: string;
  clearCategory?: boolean;
  assignedTo?: string;
  clearAssignee?: boolean;
}

export const getAdminSupportConversations = async (params?: {
  filter?: 'CUSTOMER' | 'SELLER';
  search?: string;
}): Promise<SupportConversation[]> => {
  return request<SupportConversation[]>('GET', '/admin/support/conversations', {
    params: {
      filter: params?.filter,
      q: params?.search,
    },
  });
};

export const getAdminSupportConversation = async (
  id: string
): Promise<SupportConversationDetail> => {
  return request<SupportConversationDetail>('GET', `/admin/support/conversations/${id}`);
};

export const sendAdminSupportReply = async (
  id: string,
  data: { textContent: string; attachmentIds?: string[] }
): Promise<SupportMessage> => {
  return request<SupportMessage>('POST', `/admin/support/conversations/${id}/messages`, {
    body: data,
  });
};

export const createAdminSupportInternalNote = async (
  id: string,
  data: { textContent: string }
): Promise<SupportInternalNote> => {
  return request<SupportInternalNote>('POST', `/admin/support/conversations/${id}/internal-notes`, {
    body: data,
  });
};

export const markAdminSupportRead = async (id: string): Promise<{ status: string }> => {
  return request<{ status: string }>('POST', `/admin/support/conversations/${id}/read`);
};

export const completeAdminSupportSession = async (id: string): Promise<{ status: string }> => {
  return request<{ status: string }>('POST', `/admin/support/conversations/${id}/complete`);
};

export const reopenAdminSupportSession = async (sessionId: string): Promise<{ status: string }> => {
  return request<{ status: string }>('POST', `/admin/support/sessions/${sessionId}/reopen`);
};

export const updateAdminSupportSession = async (
  id: string,
  data: UpdateSupportSessionRequest
): Promise<{ status: string }> => {
  return request<{ status: string }>('PATCH', `/admin/support/conversations/${id}/session`, {
    body: data,
  });
};

export const getAdminSupportCategories = async (
  scope?: 'CUSTOMER' | 'SELLER'
): Promise<SupportCategory[]> => {
  return request<SupportCategory[]>('GET', '/admin/support/categories', {
    params: { scope },
  });
};

export const getAdminSupportAttachmentUrl = (id: string): string => {
  return `/api/admin/support/attachments/${id}`;
};
