import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AdminInventoryReconciliation } from "./AdminInventoryReconciliation";
import * as api from "../api/adminInventory";

vi.mock("../api/adminInventory", () => ({
  getInventoryReconciliation: vi.fn(),
  scanInventoryReconciliation: vi.fn(),
  completeInventoryReconciliation: vi.fn(),
  moveInventoryReconciliationToReview: vi.fn(),
  cancelInventoryReconciliation: vi.fn(),
  getInventoryReconciliationReview: vi.fn(),
  getReconciliationResolutionPlan: vi.fn(),
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
});
