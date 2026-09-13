import { describe, it, expect } from 'vitest';
import {
  STAFF_CAPABILITIES,
  STAFF_CAPABILITY_GROUPS,
  STAFF_CAPABILITY_MAP,
  getCapabilityDefinition,
  getCapabilitiesByGroup,
  getAllCapabilityKeys,
  type StaffCapabilityGroupKey,
} from './staffCapabilities';

// Canonical 86 backend capabilities for ZAMK staff RBAC
const EXPECTED_CAPABILITIES: string[] = [
  'analytics.read',
  'auctions.cancel',
  'auctions.create',
  'auctions.finalize',
  'auctions.manage_settings',
  'auctions.move_to_direct_sale',
  'auctions.pause',
  'auctions.publish',
  'auctions.read',
  'auctions.resume',
  'auctions.update',
  'audit.read',
  'brands.create',
  'brands.delete',
  'brands.read',
  'brands.update',
  'categories.create',
  'categories.delete',
  'categories.read',
  'categories.update',
  'commission.manage',
  'complaints.read',
  'complaints.resolve',
  'dashboard.read',
  'exports.excel',
  'inventory.adjust',
  'inventory.movements.read',
  'inventory.read',
  'inventory.receipt',
  'inventory.write_off',
  'orders.read',
  'orders.update_status',
  'payments.read',
  'payouts.approve',
  'payouts.create',
  'payouts.mark_paid',
  'payouts.read',
  'payouts.reject',
  'payouts.update',
  'products.approve',
  'products.block',
  'products.hide',
  'products.moderate',
  'products.publish',
  'products.read',
  'products.reject',
  'refunds.create',
  'refunds.read',
  'reports.read',
  'returns.read',
  'returns.update_status',
  'reviews.approve',
  'reviews.block',
  'reviews.hide',
  'reviews.read',
  'reviews.reject',
  'roles.manage',
  'roles.read',
  'security.read',
  'sellers.create_access',
  'sellers.message',
  'sellers.read',
  'sellers.update_status',
  'sellers.verify',
  'sellers.warn',
  'settings.manage',
  'settings.read',
  'shipments.create',
  'shipments.read',
  'shipments.update_status',
  'staff.block',
  'staff.create',
  'staff.permissions.manage',
  'staff.read',
  'staff.update',
  'storefront.manage',
  'support.close',
  'support.read',
  'support.respond',
  'testing.manage',
  'users.read',
  'warehouse.dispatch',
  'warehouse.packing',
  'warehouse.picking',
  'warehouse.receiving',
  'warehouse.returns',
];

const EXPECTED_GROUPS: StaffCapabilityGroupKey[] = [
  'staff',
  'system',
  'catalog',
  'warehouse',
  'orders',
  'returns',
  'finance',
  'sellers',
  'auctions',
  'support',
  'analytics',
];

describe('Staff Capability Presentation Config (EMP.1C2A)', () => {
  it('contains exactly 86 capabilities', () => {
    expect(STAFF_CAPABILITIES.length).toBe(86);
    expect(getAllCapabilityKeys().length).toBe(86);
  });

  it('contains no duplicate keys', () => {
    const keys = STAFF_CAPABILITIES.map((c) => c.key);
    const uniqueKeys = new Set(keys);
    expect(uniqueKeys.size).toBe(86);
  });

  it('matches canonical backend registry capabilities exactly', () => {
    const configKeys = getAllCapabilityKeys().slice().sort();
    const expectedKeysSorted = EXPECTED_CAPABILITIES.slice().sort();
    expect(configKeys).toEqual(expectedKeysSorted);
  });

  it('defines exactly 11 deterministic groups with non-empty Russian labels', () => {
    expect(STAFF_CAPABILITY_GROUPS.length).toBe(11);
    const groupKeys = STAFF_CAPABILITY_GROUPS.map((g) => g.key);
    expect(groupKeys).toEqual(EXPECTED_GROUPS);

    for (const group of STAFF_CAPABILITY_GROUPS) {
      expect(group.title).toBeTruthy();
      expect(group.title.trim().length).toBeGreaterThan(0);
    }
  });

  it('assigns every capability to exactly one valid group', () => {
    const validGroupKeys = new Set(EXPECTED_GROUPS);
    for (const cap of STAFF_CAPABILITIES) {
      expect(validGroupKeys.has(cap.group)).toBe(true);
    }
  });

  it('has non-empty Russian titles and descriptions for all capabilities', () => {
    for (const cap of STAFF_CAPABILITIES) {
      expect(cap.title).toBeTruthy();
      expect(cap.title.trim().length).toBeGreaterThan(0);
      expect(cap.description).toBeTruthy();
      expect(cap.description!.trim().length).toBeGreaterThan(0);
    }
  });

  it('marks staff.permissions.manage as critical', () => {
    const managePerm = getCapabilityDefinition('staff.permissions.manage');
    expect(managePerm).toBeDefined();
    expect(managePerm?.critical).toBe(true);
    expect(managePerm?.group).toBe('staff');
  });

  it('marks other key high-privilege capabilities as critical', () => {
    const criticalKeys = [
      'staff.permissions.manage',
      'roles.manage',
      'settings.manage',
      'commission.manage',
      'testing.manage',
      'auctions.manage_settings',
      'payouts.approve',
    ];

    for (const key of criticalKeys) {
      const cap = getCapabilityDefinition(key);
      expect(cap).toBeDefined();
      expect(cap?.critical).toBe(true);
    }
  });

  it('correctly filters capabilities by group', () => {
    let totalFromGroups = 0;
    for (const groupKey of EXPECTED_GROUPS) {
      const groupCaps = getCapabilitiesByGroup(groupKey);
      expect(groupCaps.length).toBeGreaterThan(0);
      for (const cap of groupCaps) {
        expect(cap.group).toBe(groupKey);
      }
      totalFromGroups += groupCaps.length;
    }
    expect(totalFromGroups).toBe(86);
  });

  it('provides O(1) map access for all capabilities', () => {
    for (const key of EXPECTED_CAPABILITIES) {
      const def = STAFF_CAPABILITY_MAP[key];
      expect(def).toBeDefined();
      expect(def.key).toBe(key);
    }
  });
});
