import { describe, it, expect } from 'vitest';
import {
  STAFF_WORK_MODULES,
  getAllWorkModuleActions,
  getWorkModuleActions,
  getWorkModuleByCapability,
  getWorkModuleAccessState,
  getWorkModuleSummary,
  getAssignedWorkModules,
  getAccessTemplateDiff,
  STAFF_SCREEN_ACCESS_RULES,
} from './staffWorkModules';
import { getAllCapabilityKeys } from './staffCapabilities';

describe('STAFF_WORK_MODULES Presentation Config', () => {
  // A. exactly 12 modules
  it('A: defines exactly 12 canonical work modules in stable order', () => {
    expect(STAFF_WORK_MODULES).toHaveLength(12);
    const moduleKeys = STAFF_WORK_MODULES.map((m) => m.key);
    expect(moduleKeys).toEqual([
      'staff_access',
      'platform',
      'catalog',
      'moderation',
      'sellers',
      'orders_delivery',
      'warehouse',
      'returns',
      'finance',
      'support',
      'auctions',
      'analytics',
    ]);
  });

  // B. all 86 canonical capabilities mapped
  it('B: maps all 86 canonical capabilities across modules', () => {
    const allActions = getAllWorkModuleActions();
    expect(allActions).toHaveLength(86);

    const canonicalKeys = getAllCapabilityKeys();
    expect(canonicalKeys).toHaveLength(86);

    const mappedKeys = allActions.map((a) => a.capability).sort();
    const sortedCanonical = [...canonicalKeys].sort();
    expect(mappedKeys).toEqual(sortedCanonical);
  });

  // C. no duplicates
  it('C: contains zero duplicate capability mappings', () => {
    const allActions = getAllWorkModuleActions();
    const seen = new Set<string>();
    const duplicates: string[] = [];

    for (const action of allActions) {
      if (seen.has(action.capability)) {
        duplicates.push(action.capability);
      }
      seen.add(action.capability);
    }

    expect(duplicates).toEqual([]);
  });

  // D. no unknown capability keys
  it('D: contains no unknown capabilities not present in canonical registry', () => {
    const canonicalSet = new Set(getAllCapabilityKeys());
    const allActions = getAllWorkModuleActions();

    for (const action of allActions) {
      expect(canonicalSet.has(action.capability)).toBe(true);
    }
  });

  // E. every action has non-empty human title/description
  it('E: ensures every action has a non-empty human title and description', () => {
    const allActions = getAllWorkModuleActions();

    for (const action of allActions) {
      expect(action.title.trim().length).toBeGreaterThan(0);
      expect(action.description.trim().length).toBeGreaterThan(0);
      expect(['read', 'write', 'destructive', 'security']).toContain(action.class);
      expect(['normal', 'sensitive', 'critical']).toContain(action.criticality);
      expect(['current_ui', 'backend_only', 'future']).toContain(action.surface);
    }
  });

  // F. accepted screenless flags are correct
  it('F: flags screenless actions accurately (backend_only / future)', () => {
    const allActions = getAllWorkModuleActions();
    const actionMap = new Map(allActions.map((a) => [a.capability, a]));

    expect(actionMap.get('security.read')?.surface).toBe('backend_only');
    expect(actionMap.get('testing.manage')?.surface).toBe('backend_only');
    expect(actionMap.get('storefront.manage')?.surface).toBe('future');
    expect(actionMap.get('auctions.manage_settings')?.surface).toBe('future');

    // Normal actions with current UI
    expect(actionMap.get('orders.read')?.surface).toBe('current_ui');
    expect(actionMap.get('staff.permissions.manage')?.surface).toBe('current_ui');
  });

  // G. inventory.receipt label is corrected
  it('G: ensures inventory.receipt has the corrected human title and description', () => {
    const action = getAllWorkModuleActions().find((a) => a.capability === 'inventory.receipt');
    expect(action).toBeDefined();
    expect(action?.title).toBe('Приёмка поставок продавцов');
    expect(action?.description).toContain('поставок от продавцов');
  });

  // H. inventory.adjust label is corrected
  it('H: ensures inventory.adjust has the corrected human title and description', () => {
    const action = getAllWorkModuleActions().find((a) => a.capability === 'inventory.adjust');
    expect(action).toBeDefined();
    expect(action?.title).toBe('Корректировки и инвентаризация остатков');
    expect(action?.description).toContain('Проведение инвентаризаций');
  });

  // I. payouts.update label is corrected
  it('I: ensures payouts.update has the corrected human title and description', () => {
    const action = getAllWorkModuleActions().find((a) => a.capability === 'payouts.update');
    expect(action).toBeDefined();
    expect(action?.title).toBe('Процессинг и приостановка выплат');
    expect(action?.description).toContain('Отправка в процессинг');
  });

  // J. critical capabilities retain critical metadata
  it('J: ensures all mandatory critical capabilities have critical metadata', () => {
    const mandatoryCritical = [
      'staff.permissions.manage',
      'roles.manage',
      'settings.manage',
      'commission.manage',
      'testing.manage',
      'auctions.manage_settings',
      'payouts.approve',
    ];

    const actionMap = new Map(getAllWorkModuleActions().map((a) => [a.capability, a]));

    for (const key of mandatoryCritical) {
      const action = actionMap.get(key);
      expect(action, `Capability ${key} must exist`).toBeDefined();
      expect(action?.criticality, `Capability ${key} must be critical`).toBe('critical');
    }
  });

  // K. NO_ACCESS works
  it('K: calculates NO_ACCESS when zero module capabilities are assigned', () => {
    const ordersModule = STAFF_WORK_MODULES.find((m) => m.key === 'orders_delivery')!;
    const state = getWorkModuleAccessState(ordersModule, ['staff.read', 'catalog.read']);
    expect(state).toBe('NO_ACCESS');
  });

  // L. VIEW works
  it('L: calculates VIEW when only read-class actions are assigned', () => {
    const ordersModule = STAFF_WORK_MODULES.find((m) => m.key === 'orders_delivery')!;
    // orders.read and shipments.read are both 'read' class
    const state = getWorkModuleAccessState(ordersModule, ['orders.read', 'shipments.read']);
    expect(state).toBe('VIEW');

    // Single read action
    const stateSingle = getWorkModuleAccessState(ordersModule, ['orders.read']);
    expect(stateSingle).toBe('VIEW');
  });

  // M. LIMITED works
  it('M: calculates LIMITED when write/destructive/security actions are partially assigned', () => {
    const ordersModule = STAFF_WORK_MODULES.find((m) => m.key === 'orders_delivery')!;
    // orders.read ('read') + orders.update_status ('write') -> not all read, not full -> LIMITED
    const state = getWorkModuleAccessState(ordersModule, ['orders.read', 'orders.update_status']);
    expect(state).toBe('LIMITED');

    // only write action
    const stateWriteOnly = getWorkModuleAccessState(ordersModule, ['orders.update_status']);
    expect(stateWriteOnly).toBe('LIMITED');
  });

  // N. FULL requires 100%
  it('N: calculates FULL only when 100% of module capabilities are assigned', () => {
    const returnsModule = STAFF_WORK_MODULES.find((m) => m.key === 'returns')!;
    const returnsCaps = getWorkModuleActions(returnsModule).map((a) => a.capability);
    expect(returnsCaps).toHaveLength(3);

    // 2 out of 3 -> LIMITED
    const statePartial = getWorkModuleAccessState(returnsModule, [returnsCaps[0], returnsCaps[1]]);
    expect(statePartial).toBe('LIMITED');

    // All 3 -> FULL
    const stateFull = getWorkModuleAccessState(returnsModule, returnsCaps);
    expect(stateFull).toBe('FULL');
  });

  // O. work-zone helper returns only non-empty modules
  it('O: getAssignedWorkModules returns only modules with non-zero access', () => {
    const userPermissions = ['orders.read', 'returns.read', 'returns.update_status'];
    const activeModules = getAssignedWorkModules(userPermissions);

    const activeKeys = activeModules.map((m) => m.key);
    expect(activeKeys).toContain('orders_delivery');
    expect(activeKeys).toContain('returns');
    expect(activeKeys).not.toContain('warehouse');
    expect(activeKeys).not.toContain('staff_access');
    expect(activeKeys).toHaveLength(2);
  });

  // P. template diff: direct - preset, preset - direct, equality
  it('P: getAccessTemplateDiff computes set-based additions, removals, and matches', () => {
    const preset = ['orders.read', 'orders.update_status', 'shipments.read'];
    const direct = ['orders.read', 'shipments.read', 'shipments.create'];

    const diff = getAccessTemplateDiff(direct, preset);
    expect(diff.added).toEqual(['shipments.create']);
    expect(diff.removed).toEqual(['orders.update_status']);
    expect(diff.matches).toBe(false);

    // Equality case
    const exactDiff = getAccessTemplateDiff(preset, [...preset].reverse());
    expect(exactDiff.added).toEqual([]);
    expect(exactDiff.removed).toEqual([]);
    expect(exactDiff.matches).toBe(true);
  });

  // Q. screen visibility config has 22 unique keys and valid routes
  it('Q: screen visibility config has exactly 22 unique keys and unique valid routes', () => {
    expect(STAFF_SCREEN_ACCESS_RULES).toHaveLength(22);

    const keys = STAFF_SCREEN_ACCESS_RULES.map((r) => r.key);
    const uniqueKeys = new Set(keys);
    expect(uniqueKeys.size).toBe(22);

    const routes = STAFF_SCREEN_ACCESS_RULES.map((r) => r.route);
    const uniqueRoutes = new Set(routes);
    expect(uniqueRoutes.size).toBe(22);
  });

  // R. warehouse visibility rules: picking, orders receiving, supplies receiving
  it('R: warehouse screen visibility rules use exact accepted anchors', () => {
    // picking -> warehouse.picking
    const pickingRule = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/fulfillment/picking');
    expect(pickingRule).toBeDefined();
    expect(pickingRule?.visibility).toBe('warehouse.picking');

    // orders receiving scanner -> warehouse.receiving
    const ordersReceivingRule = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/orders/receiving');
    expect(ordersReceivingRule).toBeDefined();
    expect(ordersReceivingRule?.visibility).toBe('warehouse.receiving');

    // supplies receiving -> inventory.receipt
    const suppliesReceivingRule = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/supplies/receiving');
    expect(suppliesReceivingRule).toBeDefined();
    expect(suppliesReceivingRule?.visibility).toBe('inventory.receipt');
  });

  // S. individual checks for receiving routes
  it('S: preserves receiving route distinct visibility anchors', () => {
    const ordersRec = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/orders/receiving');
    const suppliesRec = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/supplies/receiving');
    expect(ordersRec?.visibility).toBe('warehouse.receiving');
    expect(suppliesRec?.visibility).toBe('inventory.receipt');
  });

  // T. dashboard/users/settings have proposed explicit visibility rules
  it('T: dashboard, users, and settings have explicit visibility rules', () => {
    const dashboardRule = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/dashboard');
    expect(dashboardRule).toBeDefined();
    expect(dashboardRule?.visibility).toEqual(['dashboard.read', 'analytics.read']);

    const usersRule = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/users');
    expect(usersRule).toBeDefined();
    expect(usersRule?.visibility).toBe('users.read');

    const settingsRule = STAFF_SCREEN_ACCESS_RULES.find((r) => r.route === '/settings');
    expect(settingsRule).toBeDefined();
    expect(settingsRule?.visibility).toBe('settings.read');
  });

  // Additional helper tests
  it('computes summary statistics for a work module', () => {
    const warehouseModule = STAFF_WORK_MODULES.find((m) => m.key === 'warehouse')!;
    const summary = getWorkModuleSummary(warehouseModule, ['inventory.read', 'inventory.write_off']);

    expect(summary.moduleKey).toBe('warehouse');
    expect(summary.assignedCount).toBe(2);
    expect(summary.totalCount).toBe(9);
    expect(summary.state).toBe('LIMITED');
    expect(summary.criticalAssignedCount).toBe(0);
    expect(summary.topAssignedActions).toHaveLength(2);
  });

  it('retrieves owning work module by capability key', () => {
    expect(getWorkModuleByCapability('orders.read')?.key).toBe('orders_delivery');
    expect(getWorkModuleByCapability('warehouse.picking')?.key).toBe('warehouse');
    expect(getWorkModuleByCapability('staff.permissions.manage')?.key).toBe('staff_access');
    expect(getWorkModuleByCapability('unknown.capability')).toBeUndefined();
  });
});
