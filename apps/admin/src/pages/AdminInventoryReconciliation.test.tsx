import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import {
  AdminInventoryReconciliation,
  formatResolvedDiscrepanciesBanner,
  classifyResolutionBucket,
} from "./AdminInventoryReconciliation";
import * as api from "../api/adminInventory";

vi.mock("../api/adminInventory", () => ({
  getInventoryReconciliation: vi.fn(),
  scanInventoryReconciliation: vi.fn(),
  completeInventoryReconciliation: vi.fn(),
  moveInventoryReconciliationToReview: vi.fn(),
  cancelInventoryReconciliation: vi.fn(),
  getInventoryReconciliationReview: vi.fn(),
  getReconciliationResolutionPlan: vi.fn(),
  resolveInventoryReconciliationCase: vi.fn(),
  getAdminInventoryItem: vi.fn(),
}));


const mockSession: api.ReconciliationSession = {
  id: "session-1",
  variantId: "variant-1",
  status: "in_progress",
  startedBy: "admin-1",
  startedAt: "2026-09-03T10:00:00Z",
  variantTitle: "Шерстяное пальто",
  variantSize: "M",
  variantColor: "Графит",
  variantSKU: "COAT-M-GRP",
  expectedCount: 4,
  foundExpectedCount: 2,
  unexpectedCount: 0,
  problemsCount: 0,
};

describe("AdminInventoryReconciliation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders scanner view with variant info and scan prompt", async () => {
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(mockSession);

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Шерстяное пальто/i)).toBeDefined();
      expect(screen.getByText(/Отсканируйте ZMU/i)).toBeDefined();
    });

    expect(screen.getByText(/SKU COAT-M-GRP/i)).toBeDefined();
    expect(screen.getByPlaceholderText(/Штрихкод ZMU.../i)).toBeDefined();
  });

  it("renders humanized wrong-variant context when scanned", async () => {
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(mockSession);
    vi.mocked(api.scanInventoryReconciliation).mockResolvedValue({
      classification: "wrong_variant",
      session: mockSession,
      unitContext: {
        unitCode: "ZMU-WRONG123",
        productTitle: "Шёлковый шарф",
        size: "XL",
        color: "Красный",
        sku: "SCARF-XL-RED",
        barcode: "BAR-SCARF",
        status: "warehouse",
      },
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Штрихкод ZMU.../i)).toBeDefined();
    });

    const input = screen.getByPlaceholderText(/Штрихкод ZMU.../i);
    fireEvent.change(input, { target: { value: "ZMU-WRONG123" } });
    fireEvent.submit(input);

    await waitFor(() => {
      expect(screen.getByText(/Другой вариант товара/i)).toBeDefined();
      expect(screen.getByText(/Это другая позиция:/i)).toBeDefined();
      expect(screen.getByText(/Шёлковый шарф · XL · Красный/i)).toBeDefined();
    });
  });

  it("renders humanized unexpected-found status when scanned", async () => {
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(mockSession);
    vi.mocked(api.scanInventoryReconciliation).mockResolvedValue({
      classification: "unexpected_found",
      session: { ...mockSession, unexpectedCount: 1 },
      unitContext: {
        unitCode: "ZMU-SHIPPED999",
        productTitle: "Шерстяное пальто",
        size: "M",
        color: "Графит",
        sku: "COAT-M-GRP",
        status: "shipped",
      },
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Штрихкод ZMU.../i)).toBeDefined();
    });

    const input = screen.getByPlaceholderText(/Штрихкод ZMU.../i);
    fireEvent.change(input, { target: { value: "ZMU-SHIPPED999" } });
    fireEvent.submit(input);

    await waitFor(() => {
      expect(screen.getByText(/Неожиданная единица/i)).toBeDefined();
      expect(screen.getByText(/Система считает эту ZMU отгруженной./i)).toBeDefined();
    });
  });

  it("renders review mode with currentStatus and human change wording", async () => {
    const reviewSession: api.ReconciliationSession = {
      ...mockSession,
      status: "review",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(reviewSession);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      missing: [{ unitId: "u-1", unitCode: "ZMU-MISSING-1", snapshotStatus: "warehouse", currentStatus: "warehouse", classification: "" }],
      unexpectedFound: [{ unitId: "u-2", unitCode: "ZMU-UNEXP-2", snapshotStatus: "", currentStatus: "shipped", classification: "unexpected_found" }],
      changedDuringCount: [{ unitId: "u-3", unitCode: "ZMU-CHANGED-3", snapshotStatus: "warehouse", currentStatus: "shipped", classification: "expected_found" }],
      expectedFound: [{ unitId: "u-4", unitCode: "ZMU-FOUND-4", snapshotStatus: "warehouse", currentStatus: "warehouse", classification: "expected_found" }],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /Проверка результатов/i })).toBeDefined();
    });

    // unexpectedFound uses currentStatus
    expect(screen.getByText(/ZMU-UNEXP-2/i)).toBeDefined();
    expect(screen.getByText(/Текущий статус: Отгружена/i)).toBeDefined();

    // changedDuringCount uses human wording
    expect(screen.getByText(/ZMU-CHANGED-3/i)).toBeDefined();
    expect(screen.getByText(/Было при начале проверки: На складе → Сейчас: Отгружена/i)).toBeDefined();
  });

  it("handles cancel via in-UI modal without native confirm", async () => {
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(mockSession);
    vi.mocked(api.cancelInventoryReconciliation).mockResolvedValue({ status: "cancelled" });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
          <Route path="/inventory" element={<div>Inventory Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Отменить/i })).toBeDefined();
    });

    fireEvent.click(screen.getByRole("button", { name: /Отменить/i }));

    // Modal opens with truthful copy
    expect(screen.getByText(/Отмена инвентаризации/i)).toBeDefined();
    expect(screen.getByText(/Инвентаризация будет отменена. Уже собранные данные сохранятся в истории./i)).toBeDefined();

    // Confirm cancel
    fireEvent.click(screen.getByRole("button", { name: /Подтвердить отмену/i }));

    await waitFor(() => {
      expect(api.cancelInventoryReconciliation).toHaveBeenCalledWith("session-1");
    });
  });

  it("renders completed session in read-only mode with banner", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      missing: [],
      unexpectedFound: [],
      changedDuringCount: [],
      expectedFound: [{ unitId: "u-4", unitCode: "ZMU-FOUND-4", snapshotStatus: "warehouse", currentStatus: "warehouse", classification: "expected_found" }],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Инвентаризация завершена. Складской учёт не изменён./i)).toBeDefined();
      expect(screen.getByText(/Результаты инвентаризации/i)).toBeDefined();
    });

    // Barcode input is NOT present in completed session
    expect(screen.queryByPlaceholderText(/Штрихкод ZMU.../i)).toBeNull();
  });

  it("loads persisted session by URL id on direct refresh with no reliance on navigation state", async () => {
    const directSession: api.ReconciliationSession = {
      ...mockSession,
      id: "session-direct-99",
      variantTitle: "Dev Wool Coat",
      variantSKU: "DEV-SKU-0",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(directSession);

    // Direct URL navigation / refresh at canonical route: /inventory/reconciliation/session-direct-99
    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-direct-99"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    // Verifies backend call by route param
    await waitFor(() => {
      expect(api.getInventoryReconciliation).toHaveBeenCalledWith("session-direct-99");
      expect(screen.getByText(/Dev Wool Coat/i)).toBeDefined();
      expect(screen.getByText(/SKU DEV-SKU-0/i)).toBeDefined();
      expect(screen.getByText(/Отсканируйте ZMU/i)).toBeDefined();
    });
  });

  it("renders submit arrow button on ZMU input and submits via click and enter identically", async () => {
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(mockSession);
    vi.mocked(api.scanInventoryReconciliation).mockResolvedValue({
      classification: "expected_found",
      session: { ...mockSession, foundExpectedCount: 3 },
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Штрихкод ZMU.../i)).toBeDefined();
    });

    const input = screen.getByPlaceholderText(/Штрихкод ZMU.../i) as HTMLInputElement;
    const submitBtn = screen.getByRole("button", { name: "Отсканировать" }) as HTMLButtonElement;

    // Disabled initially when input is empty
    expect(submitBtn.disabled).toBe(true);

    // Type code -> button becomes enabled
    fireEvent.change(input, { target: { value: "ZMU-CLICK-01" } });
    expect(submitBtn.disabled).toBe(false);

    // Click submit button
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(api.scanInventoryReconciliation).toHaveBeenCalledWith("session-1", "ZMU-CLICK-01");
    });

    // Enter submit works identically
    vi.mocked(api.scanInventoryReconciliation).mockClear();
    fireEvent.change(input, { target: { value: "ZMU-ENTER-02" } });
    fireEvent.submit(input);

    await waitFor(() => {
      expect(api.scanInventoryReconciliation).toHaveBeenCalledWith("session-1", "ZMU-ENTER-02");
    });
  });

  it("renders in_progress progress labels correctly", async () => {
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue({
      ...mockSession,
      expectedCount: 10,
      foundExpectedCount: 7,
      unexpectedCount: 2,
      problemsCount: 1,
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("Ожидалось")).toBeDefined();
      expect(screen.getByText("10")).toBeDefined();
    });

    expect(screen.getByText("Найдено ожидаемых")).toBeDefined();
    expect(screen.getByText("7")).toBeDefined();
    expect(screen.getByText("Осталось найти")).toBeDefined();
    expect(screen.getByText("3")).toBeDefined();
    expect(screen.getByText("Неожиданно найдено")).toBeDefined();
    expect(screen.getByText("2")).toBeDefined();
    expect(screen.getByText("Ошибки сканирования")).toBeDefined();
    expect(screen.getByText("1")).toBeDefined();

    // Ensure old label is not present
    expect(screen.queryByText(/Проблемы \(ошибки скана\)/i)).toBeNull();
  });

  it("renders review and completed progress labels with canonical missing count", async () => {
    const reviewSession: api.ReconciliationSession = {
      ...mockSession,
      status: "review",
      expectedCount: 10,
      foundExpectedCount: 7,
      unexpectedCount: 1,
      problemsCount: 0,
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(reviewSession);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      missing: [
        { unitId: "u-1", unitCode: "ZMU-M1", snapshotStatus: "warehouse", currentStatus: "warehouse", classification: "" },
        { unitId: "u-2", unitCode: "ZMU-M2", snapshotStatus: "warehouse", currentStatus: "warehouse", classification: "" },
        { unitId: "u-3", unitCode: "ZMU-M3", snapshotStatus: "warehouse", currentStatus: "warehouse", classification: "" },
      ],
      unexpectedFound: [],
      changedDuringCount: [],
      expectedFound: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("Ожидалось")).toBeDefined();
    });

    // In review mode: "Найдено" instead of "Найдено ожидаемых"
    expect(screen.getByText("Найдено")).toBeDefined();
    expect(screen.queryByText("Найдено ожидаемых")).toBeNull();

    // In review mode: "Не найдено" instead of "Осталось найти"
    expect(screen.getByText("Не найдено")).toBeDefined();
    expect(screen.queryByText("Осталось найти")).toBeNull();

    // Canonical review missing count used in both badge and progress stats
    expect(screen.getAllByText("3").length).toBe(2);

    expect(screen.getByText("Неожиданно найдено")).toBeDefined();
    expect(screen.getByText("Ошибки сканирования")).toBeDefined();
  });

  it("renders resolution plan tab with Russian human titles, severities, historical context and actions", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };

    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [],
      missing: [],
      unexpectedFound: [],
      changedDuringCount: [],
    });

    const mockPlan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-1",
          unitCode: "ZMU-TEST1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
            sku: "COAT-M-GRP",
          },
          caseType: "missing_live_allocated",
          title: "Не найдена — назначена заказу",
          severity: "high",
          explanation: "Единица не найдена, но назначена активному заказу ORD-100192. Заказ не может быть скомплектован.",
          currentAllocationCtx: "Заказ ORD-100192 (Оплачен)",
          historicalContext: {
            orderNumber: "ORD-100192",
            orderStatus: "paid",
          },
          allowedActions: [
            {
              id: "open_order",
              safetyLevel: "WORKFLOW_HANDOFF",
              label: "Открыть заказ ORD-100192",
              route: "/orders/order-1",
              enabled: true,
            },
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Списать недостачу",
              blockedReason: "Списание недостачи будет доступно в P2.2B",
              enabled: false,
            },
          ],
        },
        {
          unitId: "unit-2",
          unitCode: "ZMU-TEST2",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
          },
          caseType: "shipped_found",
          title: "Найдена, хотя числится отгруженной",
          severity: "critical",
          explanation: "Единица числится отгруженной по заказу ORD-100191, однако была физически обнаружена на складе.",
          allowedActions: [
            {
              id: "investigate_shipped_found",
              safetyLevel: "BLOCKED",
              label: "Требует ручного расследования",
              blockedReason: "Требуется служебная проверка",
              enabled: false,
            },
          ],
        },
      ],
    };

    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(mockPlan);

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
          <Route path="/orders/:orderId" element={<div>Order Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ")).toBeDefined();
    });

    // Click "РАЗБОР РАСХОЖДЕНИЙ"
    fireEvent.click(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ"));

    await waitFor(() => {
      expect(screen.getByText("Не найдена — назначена заказу")).toBeDefined();
      expect(screen.getByText("Найдена, хотя числится отгруженной")).toBeDefined();
    });

    // Verify raw caseType is NOT rendered as primary visible text
    expect(screen.queryByText("missing_live_allocated")).toBeNull();
    expect(screen.queryByText("shipped_found")).toBeNull();

    // Verify Russian severity labels
    expect(screen.getByText("Высокий риск")).toBeDefined();
    expect(screen.getByText("Критично")).toBeDefined();

    // Verify historical context chip with humanized status
    expect(screen.getByText("Заказ ORD-100192 · Оплачен")).toBeDefined();

    // Verify enabled action is rendered as Link with correct target
    const orderLink = screen.getByRole("link", { name: "Открыть заказ ORD-100192" });
    expect(orderLink.getAttribute("href")).toBe("/orders/order-1");

    // Verify non-executable mutation action is disabled with blockedReason
    const writeOffBtn = screen.getByRole("button", { name: "Списать недостачу" });
    expect(writeOffBtn.hasAttribute("disabled")).toBe(true);
    expect(screen.getByText("(Списание недостачи будет доступно в P2.2B)")).toBeDefined();

    // Verify BLOCKED investigation is rendered in informational callout, not as a button
    expect(screen.getByText("Требует ручного расследования")).toBeDefined();
    expect(screen.queryByRole("button", { name: "Требует ручного расследования" })).toBeNull();
  });

  it("renders empty resolution plan state when no discrepancies exist", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };

    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [],
      missing: [],
      unexpectedFound: [],
      changedDuringCount: [],
    });

    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue({
      sessionId: "session-1",
      cases: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ")).toBeDefined();
    });

    fireEvent.click(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ"));

    await waitFor(() => {
      expect(screen.getByText("Расхождений не обнаружено")).toBeDefined();
      expect(screen.getByText(/Все ожидаемые единицы найдены/i)).toBeDefined();
    });
  });

  it("humanizes domain statuses, removes warehouse cell fake actions, and presents BLOCKED as informational callout", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };

    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [],
      missing: [],
      unexpectedFound: [],
      changedDuringCount: [],
    });

    const mockPlan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-101",
          unitCode: "ZMU-101",
          variantId: "variant-1",
          variant: {
            productTitle: "Пальто мужское",
            size: "L",
            color: "Чёрный",
            sku: "COAT-BLK-L",
          },
          caseType: "shipped_found",
          title: "Обнаружена единица со статусом «Отгружен»",
          severity: "critical",
          explanation: "Единица числится отгруженной по заказу ORD-100192, однако была физически обнаружена на складе.",
          historicalContext: {
            orderId: "order-100192",
            orderNumber: "ORD-100192",
            orderStatus: "delivered",
            shipmentStatus: "delivered",
            returnStatus: "needs_info",
            supplyNumber: "SUP-001197",
          },
          allowedActions: [
            {
              id: "open_order",
              safetyLevel: "WORKFLOW_HANDOFF",
              label: "Открыть заказ ORD-100192",
              route: "/orders/order-100192",
              enabled: true,
            },
            {
              id: "investigate_shipped_found",
              safetyLevel: "BLOCKED",
              label: "Требуется ручная проверка отгрузки",
              blockedReason: "Автоматическое исправление недоступно.",
              enabled: false,
            },
          ],
        },
        {
          unitId: "unit-102",
          unitCode: "ZMU-102",
          variantId: "variant-1",
          variant: {
            productTitle: "Пальто мужское",
            size: "M",
            color: "Синий",
            sku: "COAT-BLU-M",
          },
          caseType: "stale_allocation",
          title: "Зависшая аллокация на ненайденной единице",
          severity: "high",
          explanation: "Единица не найдена, и на неё числится резерв под заказ ORD-100193.",
          historicalContext: {
            orderId: "order-100193",
            orderNumber: "ORD-100193",
            orderStatus: "delivered",
            shipmentStatus: "delivered",
            returnStatus: "rejected",
            supplyNumber: "SUP-001197",
          },
          allowedActions: [
            {
              id: "open_order",
              safetyLevel: "WORKFLOW_HANDOFF",
              label: "Открыть заказ ORD-100193",
              route: "/orders/order-100193",
              enabled: true,
            },
            {
              id: "recount",
              safetyLevel: "WORKFLOW_HANDOFF",
              label: "Перепроверить ZMU",
              route: "/warehouse/free-scan?unitCode=ZMU-102",
              enabled: true,
            },
            {
              id: "close_stale_allocation",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Освободить зависшее назначение",
              blockedReason: "Автоматическое освобождение аллокации будет доступно в P2.2B",
              enabled: false,
            },
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Списать недостачу",
              blockedReason: "Списание недостачи будет доступно в P2.2B",
              enabled: false,
            },
          ],
        },
      ],
    };

    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(mockPlan);

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
          <Route path="/orders/:orderId" element={<div>Order Page</div>} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ")).toBeDefined();
    });

    fireEvent.click(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ"));

    await waitFor(() => {
      expect(screen.getByText("Обнаружена единица со статусом «Отгружен»")).toBeDefined();
      expect(screen.getByText("Зависшая аллокация на ненайденной единице")).toBeDefined();
    });

    // 1. Raw domain statuses are NOT visible
    expect(screen.queryByText(/delivered/i)).toBeNull();
    expect(screen.queryByText(/needs_info/i)).toBeNull();
    expect(screen.queryByText(/rejected/i)).toBeNull();
    expect(screen.queryByText(/\(delivered\)/i)).toBeNull();

    // 2. Human statuses are visible in context chips
    expect(screen.getByText("Заказ ORD-100192 · Доставлен")).toBeDefined();
    expect(screen.getByText("Заказ ORD-100193 · Доставлен")).toBeDefined();
    expect(screen.getAllByText("Отгрузка · Доставлена").length).toBe(2);
    expect(screen.getByText("Возврат · Требует уточнения")).toBeDefined();
    expect(screen.getByText("Возврат · Отклонён")).toBeDefined();
    expect(screen.getAllByText("Поставка SUP-001197").length).toBe(2);

    // 3. Fake warehouse cell terminology is ABSENT, Перепроверить ZMU is present
    expect(screen.queryByText(/ячейк/i)).toBeNull();
    const recountLink = screen.getByRole("link", { name: "Перепроверить ZMU" });
    expect(recountLink).toBeDefined();
    expect(recountLink.getAttribute("href")).toBe("/warehouse/free-scan?unitCode=ZMU-102");

    // 4. BLOCKED investigation is an informational callout, NOT a clickable or disabled button
    expect(screen.getByText("Требуется ручная проверка отгрузки")).toBeDefined();
    expect(screen.getByText("Автоматическое исправление недоступно.")).toBeDefined();
    expect(screen.queryByRole("button", { name: "Требуется ручная проверка отгрузки" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Требует ручного расследования" })).toBeNull();

    // 5. Actual navigation actions remain enabled
    const order1Link = screen.getByRole("link", { name: "Открыть заказ ORD-100192" });
    expect(order1Link.getAttribute("href")).toBe("/orders/order-100192");
    const order2Link = screen.getByRole("link", { name: "Открыть заказ ORD-100193" });
    expect(order2Link.getAttribute("href")).toBe("/orders/order-100193");

    // 6. Future mutation actions remain disabled buttons
    const releaseBtn = screen.getByRole("button", { name: "Освободить зависшее назначение" });
    expect(releaseBtn.hasAttribute("disabled")).toBe(true);
    const writeOffBtn = screen.getByRole("button", { name: "Списать недостачу" });
    expect(writeOffBtn.hasAttribute("disabled")).toBe(true);
  });

  it("P2.2B: Stale Allocation modal opens and submits without native alert/confirm", async () => {
    const alertSpy = vi.spyOn(window, "alert");
    const confirmSpy = vi.spyOn(window, "confirm");

    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getAdminInventoryItem).mockResolvedValue({
      id: "variant-1",
      productId: "prod-1",
      productTitle: "Шерстяное пальто",
      productVariantId: "variant-1",
      variant: "M · Графит",
      source: "fbo",
      totalStock: 5,
      reservedStock: 1,
      availableStock: 4,
      aggregate: { total: 5, reserved: 1, available: 4 },
      physical: { warehouse: 5, allocated: 1, picked: 0, free: 4, expected: 0, damaged: 0, writtenOff: 0, shipped: 0 },
      legacy: { onHand: 0, reserved: 0, available: 0 },
      accountingMode: "serialized",
      health: { status: "healthy", issues: [] },
    });

    const planWithEnabledStale: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-stale",
          unitCode: "ZMU-STALE-1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "stale_allocation",
          title: "Зависшая аллокация на ненайденной единице",
          severity: "high",
          explanation: "Единица не найдена, и на неё числится резерв под завершённый заказ.",
          allowedActions: [
            {
              id: "close_stale_allocation",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Освободить зависшее назначение",
              enabled: true,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(planWithEnabledStale);
    vi.mocked(api.resolveInventoryReconciliationCase).mockResolvedValue({
      sessionId: "session-1",
      cases: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ")).toBeDefined();
    });
    fireEvent.click(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ"));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Освободить зависшее назначение" })).toBeDefined();
    });

    const btn = screen.getByRole("button", { name: "Освободить зависшее назначение" });
    expect(btn.hasAttribute("disabled")).toBe(false);
    fireEvent.click(btn);

    // Modal is rendered with clear explanation
    await waitFor(() => {
      expect(screen.getByText(/Старое зависшее назначение по завершённому или отменённому заказу будет освобождено/i)).toBeDefined();
      expect(screen.getAllByText("ZMU-STALE-1").length).toBeGreaterThanOrEqual(1);
    });

    // Submit mutation
    const confirmBtn = screen.getAllByRole("button", { name: "Освободить зависшее назначение" }).pop()!;
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(api.resolveInventoryReconciliationCase).toHaveBeenCalledWith("session-1", {
        unitId: "unit-stale",
        actionId: "close_stale_allocation",
        replacementUnitId: undefined,
      });
    });

    expect(alertSpy).not.toHaveBeenCalled();
    expect(confirmSpy).not.toHaveBeenCalled();
    alertSpy.mockRestore();
    confirmSpy.mockRestore();
  });

  it("P2.2B: Missing Free modal renders exact -1 stock impact and submits", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getAdminInventoryItem).mockResolvedValue({
      id: "variant-1",
      productId: "prod-1",
      productTitle: "Шерстяное пальто",
      productVariantId: "variant-1",
      variant: "M · Графит",
      source: "fbo",
      totalStock: 5,
      reservedStock: 0,
      availableStock: 5,
      aggregate: { total: 5, reserved: 0, available: 5 },
      physical: { warehouse: 5, allocated: 0, picked: 0, free: 5, expected: 0, damaged: 0, writtenOff: 0, shipped: 0 },
      legacy: { onHand: 0, reserved: 0, available: 0 },
      accountingMode: "serialized",
      health: { status: "healthy", issues: [] },
    });

    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-missing-free",
          unitCode: "ZMU-FREE-1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "missing_free",
          title: "Единица не найдена",
          severity: "warning",
          explanation: "Ожидаемая единица товара не найдена на складе.",
          allowedActions: [
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Подтвердить отсутствие",
              enabled: true,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);
    vi.mocked(api.resolveInventoryReconciliationCase).mockResolvedValue({
      sessionId: "session-1",
      cases: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "РАЗБОР РАСХОЖДЕНИЙ" })).toBeDefined();
    });
    fireEvent.click(screen.getByRole("tab", { name: "РАЗБОР РАСХОЖДЕНИЙ" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Подтвердить отсутствие" })).toBeDefined();
    });

    fireEvent.click(screen.getByRole("button", { name: "Подтвердить отсутствие" }));

    // Verify modal displays exact stock impact
    await waitFor(() => {
      expect(screen.getByText("ОБЩИЙ ОСТАТОК")).toBeDefined();
      expect(screen.getAllByText(/5\s*→/).length).toBe(2);
      expect(screen.getByText("ФИЗИЧЕСКИЕ ZMU НА СКЛАДЕ")).toBeDefined();
      expect(screen.getByText("БЕЗ ZMU")).toBeDefined();
      expect(screen.getByText("-1")).toBeDefined();
      expect(screen.getAllByText("ZMU-FREE-1").length).toBeGreaterThanOrEqual(1);
    });

    // Confirm writeoff
    const confirmBtn = screen.getAllByRole("button", { name: "Подтвердить отсутствие" }).pop()!;
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(api.resolveInventoryReconciliationCase).toHaveBeenCalledWith("session-1", {
        unitId: "unit-missing-free",
        actionId: "confirm_missing",
        replacementUnitId: undefined,
      });
    });
  });

  it("P2.2B: Missing Live Allocated renders candidate selector, prevents auto-selection, blocks empty submit, and submits selected candidate", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getAdminInventoryItem).mockResolvedValue({
      id: "variant-1",
      productId: "prod-1",
      productTitle: "Шерстяное пальто",
      productVariantId: "variant-1",
      variant: "M · Графит",
      source: "fbo",
      totalStock: 5,
      reservedStock: 1,
      availableStock: 4,
      aggregate: { total: 5, reserved: 1, available: 4 },
      physical: { warehouse: 5, allocated: 1, picked: 0, free: 4, expected: 0, damaged: 0, writtenOff: 0, shipped: 0 },
      legacy: { onHand: 0, reserved: 0, available: 0 },
      accountingMode: "serialized",
      health: { status: "healthy", issues: [] },
    });

    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-live-alloc",
          unitCode: "ZMU-MISSING-LIVE",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "missing_live_allocated",
          title: "Не найдена — назначена заказу",
          severity: "high",
          explanation: "Единица назначена заказу ORD-1001.",
          replacementCandidates: [
            {
              unitId: "cand-1",
              unitCode: "ZMU-CAND-01",
              variantId: "variant-1",
              status: "warehouse",
              createdAt: "2026-09-01T10:00:00Z",
            },
            {
              unitId: "cand-2",
              unitCode: "ZMU-CAND-02",
              variantId: "variant-1",
              status: "warehouse",
              createdAt: "2026-09-02T10:00:00Z",
            },
          ],
          allowedActions: [
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Подтвердить отсутствие и заменить единицу",
              enabled: true,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);
    vi.mocked(api.resolveInventoryReconciliationCase).mockResolvedValue({
      sessionId: "session-1",
      cases: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ")).toBeDefined();
    });
    fireEvent.click(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ"));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Подтвердить отсутствие и заменить единицу" })).toBeDefined();
    });

    fireEvent.click(screen.getByRole("button", { name: "Подтвердить отсутствие и заменить единицу" }));

    // Verify candidate selector is rendered and NO candidate is auto-selected
    await waitFor(() => {
      expect(screen.getByText(/Выберите единицу на замену/i)).toBeDefined();
    });

    const select = screen.getByRole("combobox") as HTMLSelectElement;
    expect(select.value).toBe(""); // NO auto-selection!

    // Confirm button is disabled without explicit selection
    const confirmBtn = screen.getAllByRole("button", { name: "Подтвердить отсутствие и заменить единицу" }).pop()!;
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(true);

    // Option labels say "— Свободна"
    expect(screen.getByText("ZMU-CAND-01 — Свободна")).toBeDefined();
    expect(screen.getByText("ZMU-CAND-02 — Свободна")).toBeDefined();

    // Now select cand-2
    fireEvent.change(select, { target: { value: "cand-2" } });
    expect(select.value).toBe("cand-2");
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(false);

    // Change back to placeholder -> confirm button disabled again
    fireEvent.change(select, { target: { value: "" } });
    expect(select.value).toBe("");
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(true);

    // Re-select cand-2 -> confirm button enabled
    fireEvent.change(select, { target: { value: "cand-2" } });
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(false);

    // Click confirm
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(api.resolveInventoryReconciliationCase).toHaveBeenCalledWith("session-1", {
        unitId: "unit-live-alloc",
        actionId: "confirm_missing",
        replacementUnitId: "cand-2",
      });
    });
  });

  it("P2.2B: 409 conflict renders human error message and preserves UI state", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getAdminInventoryItem).mockResolvedValue({
      id: "variant-1",
      productId: "prod-1",
      productTitle: "Шерстяное пальто",
      productVariantId: "variant-1",
      variant: "M · Графит",
      source: "fbo",
      totalStock: 5,
      reservedStock: 0,
      availableStock: 5,
      aggregate: { total: 5, reserved: 0, available: 5 },
      physical: { warehouse: 5, allocated: 0, picked: 0, free: 5, expected: 0, damaged: 0, writtenOff: 0, shipped: 0 },
      legacy: { onHand: 0, reserved: 0, available: 0 },
      accountingMode: "serialized",
      health: { status: "healthy", issues: [] },
    });

    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-free",
          unitCode: "ZMU-FREE-1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "missing_free",
          title: "Единица не найдена",
          severity: "warning",
          explanation: "Ожидаемая единица товара не найдена на складе.",
          allowedActions: [
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Подтвердить отсутствие",
              enabled: true,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);

    const conflictErr = new Error("Конфликт состояния при разрешении расхождения: unit status changed");
    (conflictErr as any).status = 409;
    vi.mocked(api.resolveInventoryReconciliationCase).mockRejectedValue(conflictErr);

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ")).toBeDefined();
    });
    fireEvent.click(screen.getByText("РАЗБОР РАСХОЖДЕНИЙ"));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Подтвердить отсутствие" })).toBeDefined();
    });

    fireEvent.click(screen.getByRole("button", { name: "Подтвердить отсутствие" }));

    const confirmBtn = screen.getAllByRole("button", { name: "Подтвердить отсутствие" }).pop()!;
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(screen.getByText("Состояние изменилось. Обновите данные и повторите действие.")).toBeDefined();
    });
  });

  it("P2.2B: completed reconciliation preserves historical evidence and displays dedicated resolution progress", async () => {
    // 1. Completed session with expected 4, foundExpected 3 -> historical missing count remains 1
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
      expectedCount: 4,
      foundExpectedCount: 3,
      unexpectedCount: 1,
      problemsCount: 1,
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);

    // 2. Resolution summary shows: total 2, resolved 1, manual review 1, actionable 0
    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      resolutionsCount: 1,
      cases: [
        {
          unitId: "unit-resolved",
          unitCode: "ZMU-RESOLVED-1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "missing_free",
          title: "Единица не найдена",
          severity: "warning",
          explanation: "Единица списана со склада.",
          allowedActions: [],
          resolution: {
            actionId: "confirm_missing",
            performedBy: "admin",
            performedAt: "2026-09-04T10:07:52Z",
          },
        },
        {
          unitId: "unit-manual",
          unitCode: "ZMU-MANUAL-1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "shipped_found",
          title: "Отгруженная единица обнаружена",
          severity: "critical",
          explanation: "Требуется ручной разбор отгрузки.",
          allowedActions: [
            {
              id: "blocked_inspect",
              label: "Ручная проверка",
              safetyLevel: "BLOCKED",
              blockedReason: "Требуется физическая верификация",
              enabled: false,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [{ unitId: "u1", unitCode: "ZMU-1", classification: "expected_found", snapshotStatus: "warehouse", currentStatus: "warehouse" }],
      missing: [{ unitId: "unit-resolved", unitCode: "ZMU-RESOLVED-1", classification: "missing", snapshotStatus: "warehouse", currentStatus: "written_off" }],
      unexpectedFound: [{ unitId: "u-unexp", unitCode: "ZMU-UNEXP", classification: "unexpected_found", snapshotStatus: "sold", currentStatus: "sold" }],
      changedDuringCount: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    // Verify historical stats: expected 4, found 3, missing 1
    await waitFor(() => {
      expect(screen.getByText("Ожидалось").nextElementSibling?.textContent).toBe("4");
      expect(screen.getByText("Найдено").nextElementSibling?.textContent).toBe("3");
      expect(screen.getByText("Не найдено").nextElementSibling?.textContent).toBe("1");
    });

    // Verify separate resolution summary card
    expect(screen.getByText("Разбор расхождений")).toBeDefined();
    expect(screen.getByText("Всего расхождений").nextElementSibling?.textContent).toBe("2");
    expect(screen.getByText("Исправлено").nextElementSibling?.textContent).toBe("1");
    expect(screen.getByText("Требует ручной проверки").nextElementSibling?.textContent).toBe("1");
    expect(screen.getByText("Требует действий").nextElementSibling?.textContent).toBe("0");

    // 4. Banner truth: resolved cases = 1 -> "Инвентаризация завершена. Исправлено 1 расхождение."
    expect(screen.getByText("Инвентаризация завершена. Исправлено 1 расхождение.")).toBeDefined();
    expect(screen.queryByText("Инвентаризация завершена. Складской учёт не изменён.")).toBeNull();

    // Central card truth: resolvedDiscrepancies > 0 -> "Обнаружены расхождения. Часть расхождений уже исправлена."
    expect(screen.getByText(/Часть расхождений уже исправлена/)).toBeDefined();
    expect(screen.queryByText(/Они зафиксированы, но складской учёт не изменён/)).toBeNull();
  });

  it("P2.2B: 2 resolution audit records for 1 discrepancy case displays '1' in summary and 'Исправлено 1 расхождение' in banner", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      resolutionsCount: 2, // 2 audit rows (e.g. close_stale_allocation + confirm_missing)
      resolvedCasesCount: 1, // 1 resolved discrepancy case
      cases: [
        {
          unitId: "unit-resolved",
          unitCode: "ZMU-RESOLVED-1",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "missing_free",
          severity: "critical",
          title: "Отсутствует",
          explanation: "Unit missing",
          resolution: {
            actionId: "confirm_missing",
            performedBy: "admin",
            performedAt: "2026-09-04T12:00:00Z",
          },
          allowedActions: [],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [],
      missing: [{ unitId: "unit-resolved", unitCode: "ZMU-RESOLVED-1", classification: "missing", snapshotStatus: "warehouse", currentStatus: "written_off" }],
      unexpectedFound: [],
      changedDuringCount: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    // Summary card must say "Исправлено: 1"
    await waitFor(() => {
      expect(screen.getByText("Исправлено").nextElementSibling?.textContent).toBe("1");
    });

    // Banner must say "Инвентаризация завершена. Исправлено 1 расхождение." (NOT 2)
    expect(screen.getByText("Инвентаризация завершена. Исправлено 1 расхождение.")).toBeDefined();
    expect(screen.queryByText(/Исправлено 2/)).toBeNull();
  });

  it("P2.2B: banner shows 'Складской учёт не изменён' when resolutionsCount is 0", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      resolutionsCount: 0,
      cases: [],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);

    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [],
      missing: [{ unitId: "u-miss", unitCode: "ZMU-MISS", classification: "missing", snapshotStatus: "warehouse", currentStatus: "warehouse" }],
      unexpectedFound: [],
      changedDuringCount: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText("Инвентаризация завершена. Складской учёт не изменён.")).toBeDefined();
    });
    expect(screen.queryByText(/Исправлено \d+ расхожден/)).toBeNull();

    // Central card message: 0 resolved -> "Они зафиксированы, но складской учёт не изменён."
    expect(screen.getByText(/Они зафиксированы, но складской учёт не изменён/)).toBeDefined();
    expect(screen.queryByText(/Часть расхождений уже исправлена/)).toBeNull();
  });

  it("P2.2B: confirmation modal displays canonical 29 -> 28, physical 4 -> 3, legacy 25 -> 25 without broken values", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getAdminInventoryItem).mockResolvedValue({
      id: "variant-1",
      productId: "prod-1",
      productTitle: "Шерстяное пальто",
      productVariantId: "variant-1",
      variant: "M · Графит",
      source: "fbo",
      totalStock: 29,
      reservedStock: 2,
      availableStock: 27,
      aggregate: { total: 29, reserved: 2, available: 27 },
      physical: { warehouse: 4, allocated: 0, picked: 0, free: 4, expected: 0, damaged: 0, writtenOff: 0, shipped: 0 },
      legacy: { onHand: 25, reserved: 2, available: 23 },
      accountingMode: "mixed",
      health: { status: "healthy", issues: [] },
    });

    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-writeoff",
          unitCode: "ZMU-XUJBQQ5ADSW4BWTX",
          variantId: "variant-1",
          variant: {
            productTitle: "Шерстяное пальто",
            size: "M",
            color: "Графит",
          },
          caseType: "missing_free",
          title: "Единица не найдена",
          severity: "warning",
          explanation: "Ожидаемая единица товара не найдена на складе.",
          allowedActions: [
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Подтвердить отсутствие",
              enabled: true,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "РАЗБОР РАСХОЖДЕНИЙ" })).toBeDefined();
    });
    fireEvent.click(screen.getByRole("tab", { name: "РАЗБОР РАСХОЖДЕНИЙ" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Подтвердить отсутствие" })).toBeDefined();
    });
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить отсутствие" }));

    // Verify modal displays exact canonical stock impact
    await waitFor(() => {
      expect(screen.getByText("ОБЩИЙ ОСТАТОК")).toBeDefined();
      expect(screen.getByText(/29\s*→/)).toBeDefined();

      expect(screen.getByText("ФИЗИЧЕСКИЕ ZMU НА СКЛАДЕ")).toBeDefined();
      expect(screen.getByText(/4\s*→/)).toBeDefined();

      expect(screen.getByText("БЕЗ ZMU")).toBeDefined();
      expect(screen.getByText("25 → 25")).toBeDefined();

      expect(screen.getByText("-1")).toBeDefined();
    });

    // Ensure broken legacy format "— -1" is never rendered
    expect(screen.queryByText(/—\s*-1/)).toBeNull();
    expect(screen.queryByText(/Текущий общий остаток: 4 шт./)).toBeNull();
  });

  it("P2.2B: real live-allocation replacement modal safety polish (disabled button, truthful stock impact, free candidate labeling, replacement preview)", async () => {
    const completedSession: api.ReconciliationSession = {
      ...mockSession,
      status: "completed",
      expectedCount: 4,
    };
    vi.mocked(api.getInventoryReconciliation).mockResolvedValue(completedSession);
    vi.mocked(api.getAdminInventoryItem).mockResolvedValue({
      id: "variant-1",
      productId: "prod-1",
      productTitle: "Dev Wool Coat",
      productVariantId: "variant-1",
      variant: "M · Graphite",
      source: "fbo",
      totalStock: 28,
      reservedStock: 1,
      availableStock: 27,
      aggregate: { total: 28, reserved: 1, available: 27 },
      accountingMode: "mixed",
      physical: {
        warehouse: 3,
        allocated: 1,
        picked: 0,
        free: 2,
        expected: 1,
        damaged: 1,
        writtenOff: 1,
        shipped: 2,
      },
      legacy: {
        onHand: 25,
        reserved: 0,
        available: 25,
      },
      health: {
        status: "healthy",
        issues: [],
      },
    });

    const plan: api.ReconciliationResolutionPlan = {
      sessionId: "session-1",
      cases: [
        {
          unitId: "unit-wjef",
          unitCode: "ZMU-WJEFXRQDGPYY6JF7",
          variantId: "variant-1",
          variant: {
            productTitle: "Dev Wool Coat",
            size: "M",
            color: "Graphite",
          },
          caseType: "missing_live_allocated",
          title: "Не найдена — назначена заказу",
          severity: "high",
          explanation: "Единица назначена заказу ORD-100196.",
          historicalContext: {
            orderNumber: "ORD-100196",
            orderStatus: "assembling",
          },
          replacementCandidates: [
            {
              unitId: "cand-brfe",
              unitCode: "ZMU-BRFEA757ZAMUQYVW",
              variantId: "variant-1",
              status: "warehouse",
              createdAt: "2026-09-01T10:00:00Z",
            },
            {
              unitId: "cand-zyn",
              unitCode: "ZMU-ZYN3FBJXEWUH4GQZ",
              variantId: "variant-1",
              status: "warehouse",
              createdAt: "2026-09-02T10:00:00Z",
            },
          ],
          allowedActions: [
            {
              id: "confirm_missing",
              safetyLevel: "MUTATION_REQUIRES_CONFIRMATION",
              label: "Подтвердить отсутствие и заменить единицу",
              enabled: true,
            },
          ],
        },
      ],
    };
    vi.mocked(api.getReconciliationResolutionPlan).mockResolvedValue(plan);
    vi.mocked(api.getInventoryReconciliationReview).mockResolvedValue({
      expectedFound: [],
      missing: [{ unitId: "unit-wjef", unitCode: "ZMU-WJEFXRQDGPYY6JF7", classification: "missing", snapshotStatus: "warehouse", currentStatus: "warehouse" }],
      unexpectedFound: [],
      changedDuringCount: [],
    });
    vi.mocked(api.resolveInventoryReconciliationCase).mockResolvedValue({
      sessionId: "session-1",
      cases: [],
    });

    render(
      <MemoryRouter initialEntries={["/inventory/reconciliation/session-1"]}>
        <Routes>
          <Route path="/inventory/reconciliation/:id" element={<AdminInventoryReconciliation />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "РАЗБОР РАСХОЖДЕНИЙ" })).toBeDefined();
    });
    fireEvent.click(screen.getByRole("tab", { name: "РАЗБОР РАСХОЖДЕНИЙ" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Подтвердить отсутствие и заменить единицу" })).toBeDefined();
    });
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить отсутствие и заменить единицу" }));

    // Verify modal elements
    await waitFor(() => {
      expect(screen.getByText(/Отсутствующая единица будет списана/)).toBeDefined();
      expect(screen.getByText(/Выбранная свободная ZMU будет назначена заказу ORD-100196/)).toBeDefined();
      expect(screen.getByText(/Количество зарезервированного товара для заказа не изменится/)).toBeDefined();
    });

    // Stock impact checks: 28 -> 27, 3 -> 2, 25 -> 25, 1 -> 1
    expect(screen.getByText("ОБЩИЙ ОСТАТОК")).toBeDefined();
    expect(screen.getByText(/28\s*→/)).toBeDefined();
    expect(screen.getByText("ФИЗИЧЕСКИЕ ZMU НА СКЛАДЕ")).toBeDefined();
    expect(screen.getByText(/3\s*→/)).toBeDefined();
    expect(screen.getByText("БЕЗ ZMU")).toBeDefined();
    expect(screen.getByText("25 → 25")).toBeDefined();
    expect(screen.getByText("В РЕЗЕРВЕ")).toBeDefined();
    expect(screen.getByText("1 → 1")).toBeDefined();

    // Check candidate options label format: "— Свободна"
    const select = screen.getByRole("combobox") as HTMLSelectElement;
    expect(select.value).toBe(""); // No auto-selection!
    expect(screen.getByText("ZMU-BRFEA757ZAMUQYVW — Свободна")).toBeDefined();
    expect(screen.getByText("ZMU-ZYN3FBJXEWUH4GQZ — Свободна")).toBeDefined();

    // Confirm button is disabled without selection
    const confirmBtn = screen.getAllByRole("button", { name: "Подтвердить отсутствие и заменить единицу" }).pop()!;
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(true);

    // No replacement preview before selection
    expect(screen.queryByText("ЗАМЕНА")).toBeNull();

    // Select first candidate
    fireEvent.change(select, { target: { value: "cand-brfe" } });
    expect(select.value).toBe("cand-brfe");
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(false);

    // Replacement preview is rendered: ZMU-WJEF... -> ZMU-BRFE...
    expect(screen.getByText("ЗАМЕНА")).toBeDefined();
    const previewBrfe = screen.getByText("ЗАМЕНА").parentElement!;
    expect(previewBrfe.textContent).toContain("ZMU-WJEFXRQDGPYY6JF7");
    expect(previewBrfe.textContent).toContain("ZMU-BRFEA757ZAMUQYVW");

    // Change back to placeholder
    fireEvent.change(select, { target: { value: "" } });
    expect(select.value).toBe("");
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByText("ЗАМЕНА")).toBeNull();

    // Select second candidate
    fireEvent.change(select, { target: { value: "cand-zyn" } });
    expect(select.value).toBe("cand-zyn");
    expect((confirmBtn as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByText("ЗАМЕНА")).toBeDefined();
    const previewZyn = screen.getByText("ЗАМЕНА").parentElement!;
    expect(previewZyn.textContent).toContain("ZMU-WJEFXRQDGPYY6JF7");
    expect(previewZyn.textContent).toContain("ZMU-ZYN3FBJXEWUH4GQZ");

    // Submit
    fireEvent.click(confirmBtn);
    await waitFor(() => {
      expect(api.resolveInventoryReconciliationCase).toHaveBeenCalledWith("session-1", {
        unitId: "unit-wjef",
        actionId: "confirm_missing",
        replacementUnitId: "cand-zyn",
      });
    });
  });

  describe("formatResolvedDiscrepanciesBanner pluralization", () => {
    it("formats 0 count as 'Инвентаризация завершена. Складской учёт не изменён.'", () => {
      expect(formatResolvedDiscrepanciesBanner(0)).toBe("Инвентаризация завершена. Складской учёт не изменён.");
      expect(formatResolvedDiscrepanciesBanner(-1)).toBe("Инвентаризация завершена. Складской учёт не изменён.");
    });

    it("formats 1 count as 'Инвентаризация завершена. Исправлено 1 расхождение.'", () => {
      expect(formatResolvedDiscrepanciesBanner(1)).toBe("Инвентаризация завершена. Исправлено 1 расхождение.");
      expect(formatResolvedDiscrepanciesBanner(21)).toBe("Инвентаризация завершена. Исправлено 21 расхождение.");
      expect(formatResolvedDiscrepanciesBanner(101)).toBe("Инвентаризация завершена. Исправлено 101 расхождение.");
    });

    it("formats 2-4 count as 'Инвентаризация завершена. Исправлено N расхождения.'", () => {
      expect(formatResolvedDiscrepanciesBanner(2)).toBe("Инвентаризация завершена. Исправлено 2 расхождения.");
      expect(formatResolvedDiscrepanciesBanner(3)).toBe("Инвентаризация завершена. Исправлено 3 расхождения.");
      expect(formatResolvedDiscrepanciesBanner(4)).toBe("Инвентаризация завершена. Исправлено 4 расхождения.");
      expect(formatResolvedDiscrepanciesBanner(22)).toBe("Инвентаризация завершена. Исправлено 22 расхождения.");
    });

    it("formats 5-20 and teens count as 'Инвентаризация завершена. Исправлено N расхождений.'", () => {
      expect(formatResolvedDiscrepanciesBanner(5)).toBe("Инвентаризация завершена. Исправлено 5 расхождений.");
      expect(formatResolvedDiscrepanciesBanner(11)).toBe("Инвентаризация завершена. Исправлено 11 расхождений.");
      expect(formatResolvedDiscrepanciesBanner(12)).toBe("Инвентаризация завершена. Исправлено 12 расхождений.");
      expect(formatResolvedDiscrepanciesBanner(14)).toBe("Инвентаризация завершена. Исправлено 14 расхождений.");
      expect(formatResolvedDiscrepanciesBanner(20)).toBe("Инвентаризация завершена. Исправлено 20 расхождений.");
    });
  });

  describe("Resolution summary mutually exclusive buckets contract", () => {
    const unresolvedMissingCase: api.ReconciliationResolutionCase = {
      unitId: "u-missing",
      unitCode: "ZMU-MISSING",
      variantId: "v-1",
      variant: { productTitle: "Coat" },
      caseType: "missing_free",
      title: "Единица не найдена",
      severity: "warning",
      explanation: "Не найдена",
      allowedActions: [
        { id: "confirm_missing", label: "Подтвердить отсутствие", safetyLevel: "MUTATION_REQUIRES_CONFIRMATION", enabled: true },
      ],
    };

    const shippedFoundCase: api.ReconciliationResolutionCase = {
      unitId: "u-shipped",
      unitCode: "ZMU-C5MXPTQ7WH8WZYQP",
      variantId: "v-1",
      variant: { productTitle: "Coat" },
      caseType: "shipped_found",
      title: "Найдена, хотя числится отгруженной",
      severity: "critical",
      explanation: "Отгруженная единица найдена на складе",
      allowedActions: [
        { id: "open_order", label: "Открыть заказ", safetyLevel: "WORKFLOW_HANDOFF", enabled: true },
        { id: "investigate_shipped_found", label: "Ручная проверка", safetyLevel: "BLOCKED", enabled: false },
        { id: "open_unit_history", label: "История движения", safetyLevel: "WORKFLOW_HANDOFF", enabled: true },
      ],
    };

    const resolvedCase: api.ReconciliationResolutionCase = {
      unitId: "u-resolved",
      unitCode: "ZMU-XUJBQQ5ADSW4BWTX",
      variantId: "v-1",
      variant: { productTitle: "Coat" },
      caseType: "missing_free",
      title: "Единица не найдена",
      severity: "critical",
      explanation: "Списана со склада",
      allowedActions: [],
      resolution: {
        actionId: "confirm_missing",
        performedBy: "admin",
        performedAt: "2026-09-04T13:07:52Z",
      },
    };

    it("unresolved missing_free -> actionRequired", () => {
      const bucket = classifyResolutionBucket(unresolvedMissingCase);
      expect(bucket).toBe("action_required");
      expect(bucket).not.toBe("resolved");
      expect(bucket).not.toBe("manual_review");
    });

    it("shipped_found -> manualReview only (even with enabled workflow actions)", () => {
      const bucket = classifyResolutionBucket(shippedFoundCase);
      expect(bucket).toBe("manual_review");
      expect(bucket).not.toBe("action_required");
      expect(bucket).not.toBe("resolved");
    });

    it("resolved case -> resolved only", () => {
      const bucket = classifyResolutionBucket(resolvedCase);
      expect(bucket).toBe("resolved");
      expect(bucket).not.toBe("manual_review");
      expect(bucket).not.toBe("action_required");
    });

    it("no case belongs to more than one summary bucket and all buckets are mutually exclusive", () => {
      const testCases = [unresolvedMissingCase, shippedFoundCase, resolvedCase];
      testCases.forEach((c) => {
        const bucket = classifyResolutionBucket(c);
        const validBuckets = ["resolved", "manual_review", "action_required"];
        expect(validBuckets).toContain(bucket);
      });
    });

    it("real semantic session: total=2, resolved=1, manualReview=1, actionRequired=0 (1 + 1 + 0 == 2)", () => {
      const realCases = [shippedFoundCase, resolvedCase];
      const total = realCases.length;
      const resolved = realCases.filter((c) => classifyResolutionBucket(c) === "resolved").length;
      const manualReview = realCases.filter((c) => classifyResolutionBucket(c) === "manual_review").length;
      const actionRequired = realCases.filter((c) => classifyResolutionBucket(c) === "action_required").length;

      expect(total).toBe(2);
      expect(resolved).toBe(1);
      expect(manualReview).toBe(1);
      expect(actionRequired).toBe(0);

      // Invariant assertion:
      expect(resolved + manualReview + actionRequired).toBe(total);
      expect(1 + 1 + 0).toBe(2);
    });
  });
});
