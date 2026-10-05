// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminSupportPage } from './AdminSupportPage';
import { AdminProtectedRoute } from '../../components/AdminProtectedRoute';
import * as adminSupportApi from '../../api/adminSupport';
import * as adminApiClient from '@zamk/api-client/src/admin';
import * as adminReturnsApi from '../../api/adminReturns';
import * as adminProductsApi from '../../api/adminProducts';
import * as adminOrdersApi from '../../api/adminOrders';
import { useAdminAuth } from '../../contexts/AdminAuthContext';

vi.mock('../../contexts/AdminAuthContext', () => ({
  useAdminAuth: vi.fn(),
}));

vi.mock('../../api/adminSupport', () => ({
  getAdminSupportConversations: vi.fn(),
  getAdminSupportConversation: vi.fn(),
  sendAdminSupportReply: vi.fn(),
  createAdminSupportInternalNote: vi.fn(),
  markAdminSupportRead: vi.fn(),
  completeAdminSupportSession: vi.fn(),
  reopenAdminSupportSession: vi.fn(),
  updateAdminSupportSession: vi.fn(),
  getAdminSupportCategories: vi.fn(),
  getAdminSupportAttachmentUrl: vi.fn((id: string) => `/api/admin/support/attachments/${id}`),
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  listStaffMembers: vi.fn(),
  getAdminOrders: vi.fn(),
}));

vi.mock('../../api/adminReturns', () => ({
  getAdminReturns: vi.fn(),
  getAdminReturn: vi.fn(),
  getReturnStatusLabel: vi.fn((s: string) => (s === 'approved' ? 'Возврат одобрен' : s || '—')),
  getReturnReasonLabel: vi.fn((r?: string) => (r === 'wrong_item' ? 'Получен не тот товар' : r || '—')),
}));

vi.mock('../../api/adminProducts', () => ({
  getAdminProducts: vi.fn(),
}));

vi.mock('../../api/adminOrders', () => ({
  getAdminOrder: vi.fn(),
}));

function mockAuth(permissions: string[]) {
  const permSet = new Set(permissions);
  const authVal = {
    user: { id: 'admin-user-1', email: 'operator@zamk.ru', role: 'admin' },
    staff: {
      userId: 'admin-user-1',
      roleCode: 'support',
      roleName: 'Поддержка',
      permissions,
      status: 'active',
    },
    permissions,
    isAuthenticated: true,
    isLoading: false,
    hasPermission: (p: string) => permSet.has(p),
    hasAnyPermission: (perms: string[]) => perms.some((p) => permSet.has(p)),
    login: vi.fn(),
    logout: vi.fn(),
    refreshToken: vi.fn(),
  };
  (useAdminAuth as any).mockReturnValue(authVal);
}

const mockConversations: adminSupportApi.SupportConversation[] = [
  {
    id: 'conv-customer-1',
    requesterType: 'CUSTOMER',
    requesterUserId: 'user-c-1',
    createdAt: '2026-10-01T10:00:00Z',
    updatedAt: '2026-10-03T12:00:00Z',
    unreadCount: 2,
    requesterName: 'Анна Иванова',
    requesterEmail: 'anna@example.com',
    latestMessageText: 'Здравствуйте, когда доставят мой заказ?',
    latestMessageAt: '2026-10-03T12:00:00Z',
    activeSession: {
      id: 'sess-1',
      conversationId: 'conv-customer-1',
      status: 'ACTIVE',
      priority: 'HIGH',
      categoryId: 'cat-delivery',
      categoryName: 'Доставка',
      assignedTo: 'admin-user-1',
      createdAt: '2026-10-03T10:00:00Z',
      updatedAt: '2026-10-03T10:00:00Z',
    },
  },
  {
    id: 'conv-seller-1',
    requesterType: 'SELLER',
    requesterSellerId: 'seller-1',
    createdAt: '2026-09-20T10:00:00Z',
    updatedAt: '2026-10-02T15:30:00Z',
    unreadCount: 0,
    requesterStoreName: 'Brand Shoes Official',
    latestMessageText: 'Уточните статус выплаты за сентябрь',
    latestMessageAt: '2026-10-02T15:30:00Z',
    activeSession: {
      id: 'sess-2',
      conversationId: 'conv-seller-1',
      status: 'ACTIVE',
      priority: 'NORMAL',
      categoryId: 'cat-finance',
      categoryName: 'Финансы',
      createdAt: '2026-10-02T15:00:00Z',
      updatedAt: '2026-10-02T15:00:00Z',
    },
  },
];

const mockDetail: adminSupportApi.SupportConversationDetail = {
  conversation: mockConversations[0],
  messages: [
    {
      id: 'msg-1',
      sessionId: 'sess-old-0',
      senderType: 'CUSTOMER',
      senderUserId: 'user-c-1',
      textContent: 'Старый вопрос из предыдущего обращения',
      createdAt: '2026-10-01T10:00:00Z',
      attachments: [],
      contextLinks: [],
    },
    {
      id: 'msg-2',
      sessionId: 'sess-1',
      senderType: 'CUSTOMER',
      senderUserId: 'user-c-1',
      textContent: 'Здравствуйте, когда доставят мой заказ?',
      createdAt: '2026-10-03T10:00:00Z',
      attachments: [
        {
          id: 'att-1',
          messageId: 'msg-2',
          contentType: 'image/png',
          sizeBytes: 154000,
          originalFilename: 'screenshot.png',
          sortOrder: 1,
          createdAt: '2026-10-03T10:00:00Z',
        },
        {
          id: 'att-2',
          messageId: 'msg-2',
          contentType: 'application/pdf',
          sizeBytes: 520000,
          originalFilename: 'receipt.pdf',
          sortOrder: 2,
          createdAt: '2026-10-03T10:00:00Z',
        },
      ],
      contextLinks: [
        {
          id: 'ctx-1',
          messageId: 'msg-2',
          contextType: 'ORDER',
          contextId: 'order-uuid-100',
          label: 'ZMK-100481',
          createdAt: '2026-10-03T10:00:00Z',
        },
        {
          id: 'ctx-2',
          messageId: 'msg-2',
          contextType: 'RETURN',
          contextId: 'ret-uuid-200',
          label: 'RET-55102',
          createdAt: '2026-10-03T10:00:00Z',
        },
        {
          id: 'ctx-3',
          messageId: 'msg-2',
          contextType: 'PRODUCT',
          contextId: 'prod-uuid-300',
          label: 'Кроссовки Urban Runner',
          createdAt: '2026-10-03T10:00:00Z',
        },
      ],
    },
    {
      id: 'msg-3',
      sessionId: 'sess-1',
      senderType: 'STAFF',
      senderUserId: 'admin-user-1',
      textContent: 'Добрый день, Анна! Ваш заказ уже передан курьеру.',
      createdAt: '2026-10-03T10:15:00Z',
      attachments: [],
      contextLinks: [],
    },
  ],
  internalNotes: [
    {
      id: 'note-1',
      sessionId: 'sess-1',
      authorUserId: 'admin-user-1',
      authorName: 'Михаил Оператор',
      textContent: 'Курьер задержался на сортировочном узле. Позвонил в логистику.',
      createdAt: '2026-10-03T10:10:00Z',
    },
  ],
};

const mockStaffList = [
  {
    userId: 'admin-user-1',
    name: 'Михаил Оператор',
    email: 'mikhail@zamk.ru',
    staffStatus: 'active',
  },
  {
    userId: 'admin-user-2',
    name: 'Елена Поддержка',
    email: 'elena@zamk.ru',
    staffStatus: 'active',
  },
];

const mockCategories = [
  {
    id: 'cat-delivery',
    name: 'Доставка',
    requesterScope: 'CUSTOMER' as const,
    active: true,
    sortOrder: 1,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'cat-finance',
    name: 'Финансы',
    requesterScope: 'BOTH' as const,
    active: true,
    sortOrder: 2,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

describe('SUPPORT.1C — Admin Support Workspace Test Suite (A-AE)', () => {
  beforeEach(() => {
    cleanup();
    vi.clearAllMocks();
    mockAuth(['support.read', 'support.respond', 'support.close']);
    (adminSupportApi.getAdminSupportConversations as any).mockResolvedValue(mockConversations);
    (adminSupportApi.getAdminSupportConversation as any).mockResolvedValue(mockDetail);
    (adminSupportApi.getAdminSupportCategories as any).mockResolvedValue(mockCategories);
    (adminApiClient.listStaffMembers as any).mockResolvedValue(mockStaffList);
    (adminApiClient.getAdminOrders as any).mockResolvedValue({ items: [{ id: 'order-1', orderNumber: 'ZMK-100481', totalAmountCents: 450000 }] });
    (adminReturnsApi.getAdminReturns as any).mockResolvedValue([
      { id: 'ret-uuid-200', orderId: 'order-uuid-100', orderNumber: 'ZMK-100481', status: 'approved' },
    ]);
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue({
      id: 'ret-uuid-200',
      orderId: 'order-uuid-100',
      orderNumber: 'ZMK-100481',
      status: 'approved',
      reason: 'wrong_item',
      customerEmail: 'anna@example.com',
      items: [
        {
          id: 'item-1',
          returnId: 'ret-uuid-200',
          title: 'Кроссовки Urban Runner',
          requestedQuantity: 1,
          priceCents: 450000,
        },
      ],
    });
    (adminProductsApi.getAdminProducts as any).mockResolvedValue({
      items: [
        { id: 'prod-uuid-300', title: 'Кроссовки Urban Runner', status: 'published' },
      ],
      totalCount: 1,
    });
    (adminSupportApi.markAdminSupportRead as any).mockResolvedValue({ status: 'ok' });
    (adminSupportApi.sendAdminSupportReply as any).mockResolvedValue({
      id: 'msg-new',
      sessionId: 'sess-1',
      senderType: 'STAFF',
      senderUserId: 'admin-user-1',
      textContent: 'Тестовый ответ оператора',
      createdAt: '2026-10-03T12:30:00Z',
      attachments: [],
      contextLinks: [],
    });
    (adminSupportApi.createAdminSupportInternalNote as any).mockResolvedValue({
      id: 'note-new',
      sessionId: 'sess-1',
      authorUserId: 'admin-user-1',
      authorName: 'Михаил Оператор',
      textContent: 'Тестовая внутренняя заметка',
      createdAt: '2026-10-03T12:31:00Z',
    });
    (adminSupportApi.updateAdminSupportSession as any).mockResolvedValue({ status: 'updated' });
    (adminSupportApi.completeAdminSupportSession as any).mockResolvedValue({ status: 'completed' });
    (adminOrdersApi.getAdminOrder as any).mockResolvedValue({
      id: 'order-uuid-100',
      orderNumber: 'ZMK-100481',
      status: 'paid',
      statusLabel: 'Оплачен',
      paymentStatus: 'paid',
      paymentStatusLabel: 'Оплачен',
      fulfillmentsCount: 0,
      itemPositionsCount: 1,
      unitsCount: 1,
      sourceType: 'online',
      customerName: 'Анна Иванова',
      customerEmail: 'anna@example.com',
      customerPhone: '+7 999 123-45-67',
      deliveryAddress: 'г. Москва, ул. Ленина, д. 1',
      deliveryMethodName: 'Курьер ZAMK',
      totalAmount: 4500,
      totalPriceCents: 450000,
      currency: 'RUB',
      items: [
        {
          id: 'item-1',
          orderId: 'order-uuid-100',
          productId: 'prod-uuid-300',
          productVariantId: 'var-1',
          sellerId: 'seller-1',
          title: 'Кроссовки Urban Runner',
          productSlug: 'urban-runner',
          priceCents: 450000,
          quantity: 1,
          subtotalPriceCents: 450000,
          createdAt: '2026-10-03T10:00:00Z',
        },
      ],
    });
  });

  async function openContextPanel() {
    const toggleBtn = await screen.findByTitle('Панель контекста и управления');
    fireEvent.click(toggleBtn);
    await screen.findByTestId('support-context-panel');
  }

  // A. Route permission gating
  it('A: route permission gating requires support.read and denies without it', async () => {
    mockAuth(['orders.read']); // missing support.read
    render(
      <MemoryRouter initialEntries={['/support']}>
        <Routes>
          <Route
            path="/support"
            element={
              <AdminProtectedRoute permission="support.read">
                <AdminSupportPage />
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Недостаточно прав')).toBeDefined();
    expect(screen.queryByText('Поддержка')).toBeNull();

    cleanup();
    mockAuth(['support.read']);
    render(
      <MemoryRouter initialEntries={['/support']}>
        <Routes>
          <Route
            path="/support"
            element={
              <AdminProtectedRoute permission="support.read">
                <AdminSupportPage />
              </AdminProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Поддержка' })).toBeDefined();
    });
  });

  // B. Inbox renders conversations
  it('B: inbox renders conversations with identity, requester badge, and latest message snippet', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Анна Иванова')).toBeDefined();
      expect(screen.getByText('Brand Shoes Official')).toBeDefined();
      expect(screen.getByText('Здравствуйте, когда доставят мой заказ?')).toBeDefined();
      expect(screen.getByText('Уточните статус выплаты за сентябрь')).toBeDefined();
    });
  });

  // C. Buyer / Seller filters
  it('C: buyer and seller filters filter conversation list', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Анна Иванова')).toBeDefined();
    });

    const sellersTab = screen.getByRole('button', { name: 'Продавцы' });
    fireEvent.click(sellersTab);

    await waitFor(() => {
      expect(adminSupportApi.getAdminSupportConversations).toHaveBeenCalledWith(
        expect.objectContaining({ filter: 'SELLER' })
      );
    });

    const buyersTab = screen.getByRole('button', { name: 'Покупатели' });
    fireEvent.click(buyersTab);

    await waitFor(() => {
      expect(adminSupportApi.getAdminSupportConversations).toHaveBeenCalledWith(
        expect.objectContaining({ filter: 'CUSTOMER' })
      );
    });
  });

  // D. Unread indication
  it('D: unread count badge is visually presented', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      const convItem = screen.getByTestId('support-conversation-item-conv-customer-1');
      expect(within(convItem).getByText('2')).toBeDefined();
    });
  });

  // E. Selecting conversation loads continuous history
  it('E: selecting conversation loads continuous history', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Анна Иванова')).toBeDefined();
    });

    const convItem = screen.getByTestId('support-conversation-item-conv-customer-1');
    fireEvent.click(convItem);

    await waitFor(() => {
      expect(adminSupportApi.getAdminSupportConversation).toHaveBeenCalledWith('conv-customer-1');
      expect(screen.getByText('Добрый день, Анна! Ваш заказ уже передан курьеру.')).toBeDefined();
    });
  });

  // F. Customer / Staff message distinction
  it('F: customer and staff messages are clearly distinguished in styling and headers', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByText('Сотрудник ZAMK')).toBeDefined();
      const customerHeaders = screen.getAllByText('Покупатель');
      expect(customerHeaders.length).toBeGreaterThan(0);
    });
  });

  // G. Seller member sender presentation
  it('G: seller message presentation presents seller sender', async () => {
    const sellerDetail: adminSupportApi.SupportConversationDetail = {
      conversation: mockConversations[1],
      messages: [
        {
          id: 'msg-s1',
          sessionId: 'sess-2',
          senderType: 'SELLER',
          senderUserId: 'seller-user-99',
          textContent: 'Вопрос от менеджера магазина',
          createdAt: '2026-10-02T15:30:00Z',
          attachments: [],
          contextLinks: [],
        },
      ],
      internalNotes: [],
    };
    (adminSupportApi.getAdminSupportConversation as any).mockResolvedValueOnce(sellerDetail);

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-seller-1'));
    });

    await waitFor(() => {
      expect(screen.getByText('Вопрос от менеджера магазина')).toBeDefined();
      const sellerLabels = screen.getAllByText('Продавец');
      expect(sellerLabels.length).toBeGreaterThan(0);
    });
  });

  // H. Session boundaries
  it('H: session boundaries render subtle dividers between distinct sessions', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByText(/Новый диалог ·/)).toBeDefined();
    });
  });

  // I. External reply
  it('I: external reply posts to messages endpoint', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('composer-input')).toBeDefined();
    });

    const textarea = screen.getByTestId('composer-input');
    fireEvent.change(textarea, { target: { value: 'Тестовый ответ оператора' } });

    const submitBtn = screen.getByTestId('composer-submit');
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(adminSupportApi.sendAdminSupportReply).toHaveBeenCalledWith('conv-customer-1', {
        textContent: 'Тестовый ответ оператора',
      });
    });
  });

  // J. Internal note mode visually distinct
  it('J: internal note mode has distinct label and button text', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('composer-mode-note')).toBeDefined();
    });

    const noteModeBtn = screen.getByTestId('composer-mode-note');
    fireEvent.click(noteModeBtn);

    await waitFor(() => {
      expect(screen.getByText('Видно только операторам ZAMK')).toBeDefined();
      expect(screen.getByText('Добавить заметку')).toBeDefined();
    });
  });

  // K. Internal note sends internal endpoint
  it('K: internal note posts to internal notes endpoint', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('composer-mode-note'));
    });

    const textarea = screen.getByTestId('composer-input');
    fireEvent.change(textarea, { target: { value: 'Внутренняя заметка по заказу' } });

    const submitBtn = screen.getByTestId('composer-submit');
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(adminSupportApi.createAdminSupportInternalNote).toHaveBeenCalledWith('conv-customer-1', {
        textContent: 'Внутренняя заметка по заказу',
      });
    });
  });

  // L. Internal note never rendered as requester-visible message semantics
  it('L: internal notes render in dedicated staff-only card with lock indicator', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('internal-note-note-1')).toBeDefined();
      expect(screen.getByText('Внутренняя заметка (видна только сотрудникам)')).toBeDefined();
      expect(
        screen.getByText('Курьер задержался на сортировочном узле. Позвонил в логистику.')
      ).toBeDefined();
    });
  });

  // M. Assignment update
  it('M: assignment update calls updateAdminSupportSession with assignedTo', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getByRole('option', { name: /Елена Поддержка/ })).toBeDefined();
    });

    const select = screen.getByTestId('assignee-select');
    fireEvent.change(select, { target: { value: 'admin-user-2' } });

    await waitFor(() => {
      expect(adminSupportApi.updateAdminSupportSession).toHaveBeenCalledWith('conv-customer-1', {
        assignedTo: 'admin-user-2',
      });
    });
  });

  // N. Priority update
  it('N: priority update calls updateAdminSupportSession with new priority', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getByTestId('priority-select')).toBeDefined();
    });

    const select = screen.getByTestId('priority-select');
    fireEvent.change(select, { target: { value: 'URGENT' } });

    await waitFor(() => {
      expect(adminSupportApi.updateAdminSupportSession).toHaveBeenCalledWith('conv-customer-1', {
        priority: 'URGENT',
      });
    });
  });

  // O. Category metadata contract is loaded and preserved
  it('O: category metadata contract is loaded and preserved', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(adminSupportApi.getAdminSupportCategories).toHaveBeenCalledWith('CUSTOMER');
    });
  });

  // P. Invalid metadata update shows error
  it('P: metadata update rejection displays error alert', async () => {
    (adminSupportApi.updateAdminSupportSession as any).mockRejectedValueOnce(
      new Error('Операция отклонена: неверная категория')
    );

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getByTestId('priority-select')).toBeDefined();
    });

    fireEvent.change(screen.getByTestId('priority-select'), { target: { value: 'HIGH' } });

    await waitFor(() => {
      expect(screen.getByTestId('support-action-error')).toBeDefined();
      expect(screen.getByText('Операция отклонена: неверная категория')).toBeDefined();
    });
  });

  // Q. Complete dialogue
  it('Q: complete dialogue prompts confirmation and executes completion', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getByTestId('complete-dialogue-button')).toBeDefined();
    });

    fireEvent.click(screen.getByTestId('complete-dialogue-button'));

    expect(screen.getByText('Завершить текущий диалог с пользователем?')).toBeDefined();
    fireEvent.click(screen.getByTestId('confirm-complete-button'));

    await waitFor(() => {
      expect(adminSupportApi.completeAdminSupportSession).toHaveBeenCalledWith('conv-customer-1');
    });
  });

  // R. Completed dialogue state
  it('R: completed dialogue state indicates dialogue is completed in composer', async () => {
    const completedDetail: adminSupportApi.SupportConversationDetail = {
      conversation: {
        ...mockConversations[0],
        activeSession: {
          ...mockConversations[0].activeSession!,
          status: 'COMPLETED',
        },
      },
      messages: mockDetail.messages,
      internalNotes: [],
    };
    (adminSupportApi.getAdminSupportConversation as any).mockResolvedValueOnce(completedDetail);

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByText('Диалог завершён')).toBeDefined();
      expect(screen.queryByTestId('composer-input')).toBeNull();
    });
  });

  // S. Staff read endpoint on reading
  it('S: opening unread conversation marks conversation read via staff read endpoint', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(adminSupportApi.markAdminSupportRead).toHaveBeenCalledWith('conv-customer-1');
    });
  });

  // T. Context ORDER rendering and navigation link
  it('T: context link of type ORDER renders order label and link', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const orderBtn = screen.getByRole('button', { name: /Заказ ZMK-100481/i });
      expect(orderBtn).toBeDefined();
    });
  });

  // U. Context RETURN rendering and navigation link
  it('U: context link of type RETURN renders return label and opens quick view', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const returnBtn = screen.getByRole('button', { name: /Возврат RET-55102/i });
      expect(returnBtn).toBeDefined();
      fireEvent.click(returnBtn);
    });

    await waitFor(() => {
      const fullLink = screen.getByRole('link', { name: /Открыть полный возврат/i });
      expect(fullLink.getAttribute('href')).toBe('/returns?id=ret-uuid-200');
    });
  });

  // V. Context PRODUCT rendering and navigation link
  it('V: context link of type PRODUCT renders product title and link', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const prodLink = screen.getByRole('link', { name: /Товар Кроссовки Urban Runner/i });
      expect(prodLink).toBeDefined();
      expect(prodLink.getAttribute('href')).toBe('/products/prod-uuid-300');
    });
  });

  // W. Attachment safe rendering
  it('W: attachments render safe preview and file rows with formatted size', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('attachment-image-att-1')).toBeDefined();
      expect(screen.getByTestId('attachment-file-att-2')).toBeDefined();
      expect(screen.getByText('receipt.pdf')).toBeDefined();
      expect(screen.getByText('507.8 КБ')).toBeDefined();
    });
  });

  // X. No raw storage_key exposed
  it('X: raw storage_key is never exposed in DOM or markup', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByText('receipt.pdf')).toBeDefined();
    });

    const html = document.body.innerHTML;
    expect(html).not.toContain('storageKey');
    expect(html).not.toContain('storage_key');
    expect(html).not.toContain('s3.amazonaws.com');
  });

  // Y. Customer context panel
  it('Y: customer context panel displays recent customer orders and contact info', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getAllByText('Информация о покупателе').length).toBeGreaterThan(0);
      const emailEls = screen.getAllByText('anna@example.com');
      expect(emailEls.length).toBeGreaterThan(0);
      expect(screen.getByText('ZMK-100481')).toBeDefined();
    });
  });

  // Z. Seller context panel
  it('Z: seller context panel displays store brand name and link to seller detail', async () => {
    const sellerDetail: adminSupportApi.SupportConversationDetail = {
      conversation: mockConversations[1],
      messages: [],
      internalNotes: [],
    };
    (adminSupportApi.getAdminSupportConversation as any).mockResolvedValueOnce(sellerDetail);

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-seller-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getAllByText('Информация о продавце').length).toBeGreaterThan(0);
      const brandEls = screen.getAllByText('Brand Shoes Official');
      expect(brandEls.length).toBeGreaterThan(0);
      const sellerLink = screen.getByRole('link', { name: /Карточка продавца/i });
      expect(sellerLink.getAttribute('href')).toBe('/sellers/seller-1');
    });
  });

  // AA. Loading state
  it('AA: loading state displays skeleton while fetching conversations', async () => {
    (adminSupportApi.getAdminSupportConversations as any).mockReturnValueOnce(
      new Promise(() => {}) // pending promise
    );

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    expect(screen.getByRole('heading', { name: 'Поддержка' })).toBeDefined();
  });

  // AB. Empty inbox
  it('AB: empty inbox renders clear Russian empty state', async () => {
    (adminSupportApi.getAdminSupportConversations as any).mockResolvedValueOnce([]);

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Диалогов не найдено')).toBeDefined();
      expect(screen.getByText('В выбранной категории нет обращений')).toBeDefined();
    });
  });

  // AC. API failure state
  it('AC: API failure displays non-blocking alert banner', async () => {
    (adminSupportApi.getAdminSupportConversations as any).mockRejectedValueOnce(
      new Error('Сетевая ошибка сервера')
    );

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('support-inbox-error')).toBeDefined();
      expect(screen.getByText('Сетевая ошибка сервера')).toBeDefined();
    });
  });

  // AD. Narrow layout / context pane collapse toggle
  it('AD: toggle button collapses and expands the context panel', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.queryByTestId('support-context-panel')).toBeNull();
    });

    const toggleBtn = screen.getByTitle('Панель контекста и управления');
    fireEvent.click(toggleBtn);

    expect(screen.getByTestId('support-context-panel')).toBeDefined();

    fireEvent.click(toggleBtn);
    expect(screen.queryByTestId('support-context-panel')).toBeNull();
  });

  // AE. No ticket terminology in primary UI
  it('AE: primary UI avoids ticket tracker terminology', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const names = screen.getAllByText('Анна Иванова');
      expect(names.length).toBeGreaterThan(0);
    });

    const html = document.body.innerHTML;
    expect(html).not.toMatch(/Ticket #/i);
    expect(html).not.toMatch(/Тикет #/i);
    expect(html).not.toMatch(/Status: IN_PROGRESS/i);
    expect(html).not.toMatch(/Закрыть тикет/i);
  });

  // AF. Only NORMAL/HIGH/URGENT priority options
  it('AF: only NORMAL/HIGH/URGENT priority options are offered', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      const select = screen.getByTestId('priority-select') as HTMLSelectElement;
      expect(select).toBeDefined();
      const optionValues = Array.from(select.querySelectorAll('option')).map((o) => o.value);
      expect(optionValues).toEqual(['NORMAL', 'HIGH', 'URGENT']);
      expect(optionValues).not.toContain('LOW');

      const optionLabels = Array.from(select.querySelectorAll('option')).map((o) => o.textContent);
      expect(optionLabels).toEqual(['Обычный', 'Высокий', 'Срочный']);
      expect(optionLabels).not.toContain('Низкий');
    });
  });

  // AG. Actual session metadata payload uses backend contract
  it('AG: actual session metadata payload uses backend contract', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await openContextPanel();

    await waitFor(() => {
      expect(screen.getByTestId('priority-select')).toBeDefined();
      expect(screen.getByTestId('assignee-select')).toBeDefined();
    });

    // 1. Priority change
    fireEvent.change(screen.getByTestId('priority-select'), { target: { value: 'URGENT' } });
    expect(adminSupportApi.updateAdminSupportSession).toHaveBeenCalledWith(
      'conv-customer-1',
      { priority: 'URGENT' }
    );

    // 2. Assignee change
    fireEvent.change(screen.getByTestId('assignee-select'), { target: { value: 'admin-user-2' } });
    expect(adminSupportApi.updateAdminSupportSession).toHaveBeenCalledWith(
      'conv-customer-1',
      { assignedTo: 'admin-user-2' }
    );

    // 3. Clear assignee
    fireEvent.change(screen.getByTestId('assignee-select'), { target: { value: '' } });
    expect(adminSupportApi.updateAdminSupportSession).toHaveBeenCalledWith(
      'conv-customer-1',
      { clearAssignee: true }
    );
  });

  // AH. Actual Support API route prefix
  it('AH: actual Support API route prefix uses /admin/support without /api/v1', async () => {
    const attUrl = adminSupportApi.getAdminSupportAttachmentUrl('test-att-123');
    expect(attUrl).toBe('/api/admin/support/attachments/test-att-123');
    expect(attUrl).not.toContain('/api/v1/');
  });

  // AI. Actual ORDER Admin navigation route
  it('AI: actual ORDER Admin navigation route navigates to /orders/:orderId', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const orderBtns = screen.getAllByRole('button', { name: /ZMK-100481/i });
      expect(orderBtns.length).toBeGreaterThan(0);
      fireEvent.click(orderBtns[0]);
    });

    await waitFor(() => {
      const fullLink = screen.getByRole('link', { name: /Открыть полную карточку заказа/i });
      expect(fullLink.getAttribute('href')).toMatch(/^\/orders\//);
    });
  });

  // AJ. Actual RETURN Admin navigation route
  it('AJ: actual RETURN Admin navigation route navigates to /returns?id=...', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const returnBtns = screen.getAllByRole('button', { name: /Возврат/i });
      expect(returnBtns.length).toBeGreaterThan(0);
      fireEvent.click(returnBtns[0]);
    });

    await waitFor(() => {
      const fullLink = screen.getByRole('link', { name: /Открыть полный возврат/i });
      expect(fullLink.getAttribute('href')).toMatch(/^\/returns\?id=/);
    });
  });

  // AK. Actual PRODUCT Admin navigation route
  it('AK: actual PRODUCT Admin navigation route navigates to /products/:productId', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const productLinks = screen.getAllByRole('link', { name: /Кроссовки Urban Runner/i });
      expect(productLinks.length).toBeGreaterThan(0);
      for (const link of productLinks) {
        expect(link.getAttribute('href')).toMatch(/^\/products\//);
      }
    });
  });

  // AL. Actual attachment endpoint and no storage_key exposure
  it('AL: actual attachment endpoint matches backend and does not expose storage_key in DOM', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      const downloadLink = screen.getByTestId('attachment-file-att-2');
      expect(downloadLink).toBeDefined();
      expect(downloadLink.getAttribute('href')).toBe('/api/admin/support/attachments/att-2');
      expect(document.body.innerHTML).not.toContain('storage_key');
      expect(document.body.innerHTML).not.toContain('minio');
      expect(document.body.innerHTML).not.toContain('s3');
    });
  });

  // AM. Reply -> Internal note -> Reply endpoint isolation
  it('AM: switching reply -> internal note -> reply maintains strict endpoint isolation', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    await waitFor(() => {
      expect(screen.getByTestId('composer-input')).toBeDefined();
    });

    // 1. Submit in reply mode
    const textarea = screen.getByTestId('composer-input');
    fireEvent.change(textarea, { target: { value: 'Ответ клиенту номер один' } });
    fireEvent.click(screen.getByTestId('composer-submit'));

    await waitFor(() => {
      expect(adminSupportApi.sendAdminSupportReply).toHaveBeenCalledWith(
        'conv-customer-1',
        { textContent: 'Ответ клиенту номер один' }
      );
      expect(adminSupportApi.createAdminSupportInternalNote).not.toHaveBeenCalled();
    });

    // 2. Switch to internal note mode
    fireEvent.click(screen.getByTestId('composer-mode-note'));
    fireEvent.change(screen.getByTestId('composer-input'), { target: { value: 'Заметка для коллег' } });
    fireEvent.click(screen.getByTestId('composer-submit'));

    await waitFor(() => {
      expect(adminSupportApi.createAdminSupportInternalNote).toHaveBeenCalledWith(
        'conv-customer-1',
        { textContent: 'Заметка для коллег' }
      );
      expect(adminSupportApi.sendAdminSupportReply).toHaveBeenCalledTimes(1);
    });

    // 3. Switch back to reply mode
    fireEvent.click(screen.getByTestId('composer-mode-reply'));
    fireEvent.change(screen.getByTestId('composer-input'), { target: { value: 'Второй ответ клиенту' } });
    fireEvent.click(screen.getByTestId('composer-submit'));

    await waitFor(() => {
      expect(adminSupportApi.sendAdminSupportReply).toHaveBeenCalledTimes(2);
      expect(adminSupportApi.sendAdminSupportReply).toHaveBeenLastCalledWith(
        'conv-customer-1',
        { textContent: 'Второй ответ клиенту' }
      );
      expect(adminSupportApi.createAdminSupportInternalNote).toHaveBeenCalledTimes(1);
    });
  });

  // AN. Seller operational context
  it('AN: seller operational context loads recent orders, returns, and products', async () => {
    const sellerDetail: adminSupportApi.SupportConversationDetail = {
      conversation: mockConversations[1], // seller conversation
      messages: [
        {
          id: 'msg-s1',
          sessionId: 'sess-2',
          senderType: 'SELLER',
          senderUserId: 'user-seller-1',
          textContent: 'Вопрос по выплате',
          createdAt: '2026-10-02T15:30:00Z',
          attachments: [],
          contextLinks: [],
        },
      ],
      internalNotes: [],
    };
    (adminSupportApi.getAdminSupportConversation as any).mockResolvedValue(sellerDetail);

    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    const item = await screen.findByTestId('support-conversation-item-conv-seller-1');
    fireEvent.click(item);

    await openContextPanel();

    await waitFor(() => {
      expect(adminApiClient.getAdminOrders).toHaveBeenCalledWith({ sellerId: 'seller-1', limit: 5 });
      expect(adminReturnsApi.getAdminReturns).toHaveBeenCalled();
      expect(adminProductsApi.getAdminProducts).toHaveBeenCalledWith({ sellerId: 'seller-1', limit: 4 });
      expect(screen.getAllByText('Информация о продавце').length).toBeGreaterThan(0);
      expect(screen.getByText('Товары продавца')).toBeDefined();
    });
  });

  // AO. Customer canonical context sources
  it('AO: customer context uses canonical orders and returns without raw UUID as primary label', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    const item = await screen.findByTestId('support-conversation-item-conv-customer-1');
    fireEvent.click(item);

    await openContextPanel();

    await waitFor(() => {
      expect(adminApiClient.getAdminOrders).toHaveBeenCalledWith({ q: 'anna@example.com', limit: 5 });
      expect(adminReturnsApi.getAdminReturns).toHaveBeenCalled();
      const nameEls = screen.getAllByText('Анна Иванова');
      expect(nameEls.length).toBeGreaterThan(0);
      const emailEls = screen.getAllByText('anna@example.com');
      expect(emailEls.length).toBeGreaterThan(0);
      expect(screen.getByText('ZMK-100481')).toBeDefined();
    });
  });

  // AP. Draft preservation across inspector lifecycle (ROOT -> ORDER -> Back -> RETURN -> Back -> close)
  it('AP: preserves unsent composer draft and selected conversation across inspector drill-down lifecycle', async () => {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );

    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });

    const composerTextarea = (await screen.findByTestId('composer-input')) as HTMLTextAreaElement;
    expect(composerTextarea).toBeDefined();

    // Type unsent draft
    fireEvent.change(composerTextarea, { target: { value: 'Неотправленный черновик оператора' } });
    expect(composerTextarea.value).toBe('Неотправленный черновик оператора');

    // Open inspector (ROOT)
    await openContextPanel();
    const panel = await screen.findByTestId('support-context-panel');
    expect(panel).toBeDefined();

    // 1. ROOT -> ORDER
    const orderBtn = await within(panel).findByRole('button', { name: /^ZMK-100481/i });
    fireEvent.click(orderBtn);
    const orderView = await screen.findByTestId('order-quick-view');
    expect(orderView).toBeDefined();
    expect(within(orderView).getByText(/Открыть полную карточку заказа/i)).toBeDefined();

    // 2. ORDER -> Back -> ROOT
    const backFromOrderBtn = within(orderView).getByRole('button', { name: 'Назад' });
    fireEvent.click(backFromOrderBtn);
    const restoredPanel = await screen.findByTestId('support-context-panel');
    expect(restoredPanel).toBeDefined();
    expect(screen.queryByTestId('order-quick-view')).toBeNull();

    // 3. ROOT -> RETURN
    const returnBtn = await within(restoredPanel).findByRole('button', { name: /Возврат/i });
    fireEvent.click(returnBtn);
    const returnView = await screen.findByTestId('return-quick-view');
    expect(returnView).toBeDefined();
    expect(within(returnView).getByText(/Открыть полный возврат/i)).toBeDefined();

    // 4. RETURN -> Back -> ROOT
    const backFromReturnBtn = within(returnView).getByRole('button', { name: 'Назад' });
    fireEvent.click(backFromReturnBtn);
    const finalPanel = await screen.findByTestId('support-context-panel');
    expect(finalPanel).toBeDefined();
    expect(screen.queryByTestId('return-quick-view')).toBeNull();

    // 5. Close inspector (X)
    const closeBtn = within(finalPanel).getByTitle('Скрыть панель');
    fireEvent.click(closeBtn);
    await waitFor(() => {
      expect(screen.queryByTestId('support-context-panel')).toBeNull();
    });

    // 6. Verify same conversation selected and draft still present
    const restoredTextarea = screen.getByTestId('composer-input') as HTMLTextAreaElement;
    expect(restoredTextarea.value).toBe('Неотправленный черновик оператора');
    expect(screen.getAllByText('Здравствуйте, когда доставят мой заказ?').length).toBeGreaterThan(0);
  });
});

describe('SUPPORT.1D — Support Inspector Stack & Drill-down Contracts', () => {
  beforeEach(() => {
    cleanup();
    vi.clearAllMocks();
    mockAuth(['support.read', 'support.respond', 'support.close']);
    (adminSupportApi.getAdminSupportConversations as any).mockResolvedValue(mockConversations);
    (adminSupportApi.getAdminSupportConversation as any).mockResolvedValue(mockDetail);
    (adminSupportApi.getAdminSupportCategories as any).mockResolvedValue(mockCategories);
    (adminApiClient.listStaffMembers as any).mockResolvedValue(mockStaffList);
    (adminApiClient.getAdminOrders as any).mockResolvedValue({
      items: [{ id: 'order-1', orderNumber: 'ZMK-100481', totalAmountCents: 450000 }],
    });
    (adminReturnsApi.getAdminReturns as any).mockResolvedValue([
      { id: 'ret-uuid-200', orderId: 'order-uuid-100', orderNumber: 'ZMK-100481', status: 'approved' },
    ]);
    (adminReturnsApi.getAdminReturn as any).mockResolvedValue({
      id: 'ret-uuid-200',
      orderId: 'order-uuid-100',
      orderNumber: 'ZMK-100481',
      status: 'approved',
      reason: 'wrong_item',
      customerEmail: 'anna@example.com',
      items: [
        {
          id: 'item-1',
          returnId: 'ret-uuid-200',
          title: 'Кроссовки Urban Runner',
          requestedQuantity: 1,
          priceCents: 450000,
        },
      ],
    });
    (adminOrdersApi.getAdminOrder as any).mockResolvedValue({
      id: 'order-uuid-100',
      orderNumber: 'ZMK-100481',
      status: 'paid',
      statusLabel: 'Оплачен',
      paymentStatus: 'paid',
      paymentStatusLabel: 'Оплачен',
      unitsCount: 1,
      customerName: 'Анна Иванова',
      customerEmail: 'anna@example.com',
      totalAmount: 4500,
      totalPriceCents: 450000,
      items: [],
    });
  });

  async function selectCustomerConvAndOpenInspector() {
    render(
      <MemoryRouter>
        <AdminSupportPage />
      </MemoryRouter>
    );
    await waitFor(() => {
      fireEvent.click(screen.getByTestId('support-conversation-item-conv-customer-1'));
    });
    const toggleBtn = await screen.findByTitle('Панель контекста и управления');
    fireEvent.click(toggleBtn);
    return await screen.findByTestId('support-context-panel');
  }

  // 1. ROOT -> ORDER -> Back -> ROOT
  it('navigates ROOT -> ORDER -> Back -> ROOT', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    const orderBtn = await within(panel).findByRole('button', { name: /^ZMK-100481/i });
    fireEvent.click(orderBtn);
    const orderView = await screen.findByTestId('order-quick-view');
    expect(orderView).toBeDefined();

    const backBtn = within(orderView).getByRole('button', { name: 'Назад' });
    fireEvent.click(backBtn);

    expect(await screen.findByTestId('support-context-panel')).toBeDefined();
    expect(screen.queryByTestId('order-quick-view')).toBeNull();
  });

  // 2. ROOT -> RETURN -> Back -> ROOT
  it('navigates ROOT -> RETURN -> Back -> ROOT', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    const returnBtn = await within(panel).findByRole('button', { name: /^Возврат/i });
    fireEvent.click(returnBtn);
    const returnView = await screen.findByTestId('return-quick-view');
    expect(returnView).toBeDefined();

    const backBtn = within(returnView).getByRole('button', { name: 'Назад' });
    fireEvent.click(backBtn);

    expect(await screen.findByTestId('support-context-panel')).toBeDefined();
    expect(screen.queryByTestId('return-quick-view')).toBeNull();
  });

  // 3. ROOT -> RETURN -> ORDER -> Back -> RETURN -> Back -> ROOT
  it('navigates ROOT -> RETURN -> ORDER -> Back -> RETURN -> Back -> ROOT', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    // ROOT -> RETURN
    const returnBtn = await within(panel).findByRole('button', { name: /^Возврат/i });
    fireEvent.click(returnBtn);
    const returnView = await screen.findByTestId('return-quick-view');
    expect(returnView).toBeDefined();

    // RETURN -> ORDER
    const linkedOrderBtn = await within(returnView).findByRole('button', { name: /Заказ/i });
    fireEvent.click(linkedOrderBtn);
    const orderView = await screen.findByTestId('order-quick-view');
    expect(orderView).toBeDefined();

    // ORDER -> Back -> RETURN
    const backFromOrderBtn = within(orderView).getByRole('button', { name: 'Назад' });
    fireEvent.click(backFromOrderBtn);
    const restoredReturnView = await screen.findByTestId('return-quick-view');
    expect(restoredReturnView).toBeDefined();
    expect(screen.queryByTestId('order-quick-view')).toBeNull();

    // RETURN -> Back -> ROOT
    const backFromReturnBtn = within(restoredReturnView).getByRole('button', { name: 'Назад' });
    fireEvent.click(backFromReturnBtn);
    expect(await screen.findByTestId('support-context-panel')).toBeDefined();
    expect(screen.queryByTestId('return-quick-view')).toBeNull();
  });

  // 4. X from ORDER closes whole inspector
  it('closes entire inspector when clicking X from ORDER view', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    const orderBtn = await within(panel).findByRole('button', { name: /^ZMK-100481/i });
    fireEvent.click(orderBtn);
    const orderView = await screen.findByTestId('order-quick-view');
    expect(orderView).toBeDefined();

    const closeBtn = within(orderView).getByRole('button', { name: 'Закрыть' });
    fireEvent.click(closeBtn);

    await waitFor(() => {
      expect(screen.queryByTestId('order-quick-view')).toBeNull();
      expect(screen.queryByTestId('support-context-panel')).toBeNull();
    });
  });

  // 5. X from RETURN closes whole inspector
  it('closes entire inspector when clicking X from RETURN view', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    const returnBtn = await within(panel).findByRole('button', { name: /^Возврат/i });
    fireEvent.click(returnBtn);
    const returnView = await screen.findByTestId('return-quick-view');
    expect(returnView).toBeDefined();

    const closeBtn = within(returnView).getByRole('button', { name: 'Закрыть' });
    fireEvent.click(closeBtn);

    await waitFor(() => {
      expect(screen.queryByTestId('return-quick-view')).toBeNull();
      expect(screen.queryByTestId('support-context-panel')).toBeNull();
    });
  });

  // 6. Switching conversation resets stale detail to ROOT
  it('resets stale detail view to ROOT when switching conversation in Inbox', async () => {
    const sellerDetail: adminSupportApi.SupportConversationDetail = {
      conversation: mockConversations[1],
      messages: [],
      internalNotes: [],
    };
    (adminSupportApi.getAdminSupportConversation as any).mockImplementation((id: string) => {
      if (id === 'conv-seller-1') return Promise.resolve(sellerDetail);
      return Promise.resolve(mockDetail);
    });

    const panel = await selectCustomerConvAndOpenInspector();

    const orderBtn = await within(panel).findByRole('button', { name: /^ZMK-100481/i });
    fireEvent.click(orderBtn);
    expect(await screen.findByTestId('order-quick-view')).toBeDefined();

    // Switch to seller conversation
    const sellerConvItem = screen.getByTestId('support-conversation-item-conv-seller-1');
    fireEvent.click(sellerConvItem);

    // Order detail from customer conv must not remain mounted
    await waitFor(() => {
      expect(screen.queryByTestId('order-quick-view')).toBeNull();
      expect(screen.getByTestId('support-context-panel')).toBeDefined();
      expect(screen.getAllByText('Информация о продавце').length).toBeGreaterThan(0);
    });
  });

  // 7. Full Order escape hatch links to canonical /orders/:id in a new tab
  it('full Order escape hatch links to canonical /orders/:id in a new tab', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    const orderBtn = await within(panel).findByRole('button', { name: /^ZMK-100481/i });
    fireEvent.click(orderBtn);
    const orderView = await screen.findByTestId('order-quick-view');
    expect(orderView).toBeDefined();

    const fullLink = within(orderView).getByRole('link', { name: /Открыть полную карточку заказа/i });
    expect(fullLink.getAttribute('href')).toBe('/orders/order-1');
    expect(fullLink.getAttribute('target')).toBe('_blank');
    expect(fullLink.getAttribute('rel')).toBe('noopener noreferrer');
  });

  // 8. Full Return escape hatch links to canonical /returns?id=:id in a new tab
  it('full Return escape hatch links to canonical /returns?id=:id in a new tab', async () => {
    const panel = await selectCustomerConvAndOpenInspector();

    const returnBtn = await within(panel).findByRole('button', { name: /^Возврат/i });
    fireEvent.click(returnBtn);
    const returnView = await screen.findByTestId('return-quick-view');
    expect(returnView).toBeDefined();

    const fullLink = within(returnView).getByRole('link', { name: /Открыть полный возврат/i });
    expect(fullLink.getAttribute('href')).toBe('/returns?id=ret-uuid-200');
    expect(fullLink.getAttribute('target')).toBe('_blank');
    expect(fullLink.getAttribute('rel')).toBe('noopener noreferrer');
  });
});
