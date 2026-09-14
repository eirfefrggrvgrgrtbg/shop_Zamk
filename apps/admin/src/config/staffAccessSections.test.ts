import { describe, it, expect } from 'vitest';
import {
  STAFF_ACCESS_SECTIONS,
  ADVANCED_ONLY_CAPABILITIES,
  getSectionMode,
  applySectionMode,
  toggleSectionAction,
  getDraftChanges,
  getAccessSection,
} from './staffAccessSections';
import {
  getAllCapabilityKeys,
} from './staffCapabilities';
import { STAFF_SCREEN_ACCESS_RULES } from './staffWorkModules';

describe('Staff Access Sections Presentation Config', () => {
  const canonicalKeys = new Set(getAllCapabilityKeys());

  it('A. every editor section uses only canonical capabilities', () => {
    expect(canonicalKeys.size).toBe(86);

    for (const section of STAFF_ACCESS_SECTIONS) {
      for (const action of section.actions) {
        expect(
          canonicalKeys.has(action.capability),
          `Section ${section.key} action ${action.capability} is not a canonical capability`
        ).toBe(true);
      }

      if (section.modes.view) {
        for (const cap of section.modes.view) {
          expect(
            canonicalKeys.has(cap),
            `Section ${section.key} view mode cap ${cap} is not canonical`
          ).toBe(true);
        }
      }

      if (section.modes.work) {
        for (const cap of section.modes.work) {
          expect(
            canonicalKeys.has(cap),
            `Section ${section.key} work mode cap ${cap} is not canonical`
          ).toBe(true);
        }
      }
    }
  });

  it('B. no capability is accidentally mapped to two primary editable sections unless explicitly documented as shared/read dependency', () => {
    const capabilityOwners = new Map<string, string>();

    for (const section of STAFF_ACCESS_SECTIONS) {
      for (const action of section.actions) {
        const existingOwner = capabilityOwners.get(action.capability);
        expect(
          existingOwner,
          `Capability ${action.capability} is duplicated in sections ${existingOwner} and ${section.key}`
        ).toBeUndefined();
        capabilityOwners.set(action.capability, section.key);
      }
    }

    // 78 unique capabilities mapped to 22 sections
    expect(capabilityOwners.size).toBe(78);
  });

  it('C. critical capabilities are not silently included in generic WORK modes', () => {
    const sensitiveCriticalKeys = new Set([
      'staff.permissions.manage',
      'roles.manage',
      'settings.manage',
      'commission.manage',
      'testing.manage',
      'auctions.manage_settings',
      'payouts.approve',
    ]);

    for (const section of STAFF_ACCESS_SECTIONS) {
      if (section.modes.work) {
        for (const cap of section.modes.work) {
          expect(
            sensitiveCriticalKeys.has(cap),
            `Critical capability ${cap} was found in generic WORK mode for section ${section.key}`
          ).toBe(false);
        }
      }
    }
  });

  it('D. real route-backed sections correspond to current Admin routes/config', () => {
    const screenRuleRoutes = new Set(STAFF_SCREEN_ACCESS_RULES.map((r) => r.route));

    expect(STAFF_ACCESS_SECTIONS.length).toBe(22);

    for (const section of STAFF_ACCESS_SECTIONS) {
      expect(section.route).toBeDefined();
      expect(
        screenRuleRoutes.has(section.route!),
        `Section ${section.key} route ${section.route} does not match any STAFF_SCREEN_ACCESS_RULES`
      ).toBe(true);
    }
  });

  it('E. hidden/backend-only capabilities are preserved outside normal editor', () => {
    expect(ADVANCED_ONLY_CAPABILITIES.length).toBe(8);

    for (const cap of ADVANCED_ONLY_CAPABILITIES) {
      expect(canonicalKeys.has(cap)).toBe(true);
    }

    // Ensure sum of normal editor caps (78) + advanced only (8) == total canonical (86)
    const sectionCaps = new Set<string>();
    for (const s of STAFF_ACCESS_SECTIONS) {
      for (const a of s.actions) {
        sectionCaps.add(a.capability);
      }
    }

    expect(sectionCaps.size).toBe(78);
    for (const adv of ADVANCED_ONLY_CAPABILITIES) {
      expect(sectionCaps.has(adv)).toBe(false);
    }

    const totalTracked = new Set([...sectionCaps, ...ADVANCED_ONLY_CAPABILITIES]);
    expect(totalTracked.size).toBe(86);
  });

  describe('Section Mode Determination', () => {
    it('returns CLOSED when no section capabilities are present in draft', () => {
      const orders = getAccessSection('orders')!;
      expect(getSectionMode(orders, [])).toBe('CLOSED');
      expect(getSectionMode(orders, ['staff.read', 'security.read'])).toBe('CLOSED');
    });

    it('returns VIEW when draft matches exact view mode', () => {
      const orders = getAccessSection('orders')!;
      expect(getSectionMode(orders, ['orders.read'])).toBe('VIEW');
      expect(getSectionMode(orders, ['orders.read', 'staff.read'])).toBe('VIEW');
    });

    it('returns WORK when draft matches exact work mode', () => {
      const orders = getAccessSection('orders')!;
      expect(getSectionMode(orders, ['orders.read', 'orders.update_status'])).toBe('WORK');
    });

    it('returns CUSTOM when draft contains custom subset or additional actions', () => {
      const orders = getAccessSection('orders')!;
      expect(getSectionMode(orders, ['orders.update_status'])).toBe('CUSTOM');

      const staffSection = getAccessSection('staff')!;
      // Standard work + critical capability -> CUSTOM
      expect(
        getSectionMode(staffSection, [
          'staff.read',
          'staff.create',
          'staff.update',
          'staff.block',
          'staff.permissions.manage',
        ])
      ).toBe('CUSTOM');
    });
  });

  describe('applySectionMode and toggleSectionAction', () => {
    it('applies CLOSED mode by removing only section capabilities, preserving unrelated and advanced rights', () => {
      const orders = getAccessSection('orders')!;
      const draft = ['orders.read', 'orders.update_status', 'staff.read', 'testing.manage'];

      const result = applySectionMode(orders, 'CLOSED', draft);
      expect(result).toEqual(['staff.read', 'testing.manage']);
    });

    it('applies VIEW mode by setting only section view capabilities', () => {
      const orders = getAccessSection('orders')!;
      const draft = ['orders.update_status', 'staff.read', 'security.read'];

      const result = applySectionMode(orders, 'VIEW', draft);
      expect(result.sort()).toEqual(['orders.read', 'security.read', 'staff.read'].sort());
    });

    it('applies WORK mode by setting only section work capabilities', () => {
      const orders = getAccessSection('orders')!;
      const draft = ['orders.read', 'staff.read'];

      const result = applySectionMode(orders, 'WORK', draft);
      expect(result.sort()).toEqual(['orders.read', 'orders.update_status', 'staff.read'].sort());
    });

    it('toggles individual capability accurately', () => {
      const draft = ['orders.read', 'staff.read'];
      const toggledOn = toggleSectionAction('orders.update_status', draft);
      expect(toggledOn).toContain('orders.update_status');

      const toggledOff = toggleSectionAction('orders.read', toggledOn);
      expect(toggledOff).not.toContain('orders.read');
      expect(toggledOff).toContain('orders.update_status');
    });
  });

  describe('getDraftChanges', () => {
    it('computes added, removed, isDirty, and flags critical changes correctly', () => {
      const orig = ['orders.read', 'staff.read', 'staff.permissions.manage'];
      const draft = ['orders.read', 'payouts.approve'];

      const diff = getDraftChanges(orig, draft);
      expect(diff.isDirty).toBe(true);
      expect(diff.added).toEqual(['payouts.approve']);
      expect(diff.removed.sort()).toEqual(['staff.permissions.manage', 'staff.read'].sort());
      expect(diff.criticalAdded).toEqual(['payouts.approve']);
      expect(diff.criticalRemoved).toEqual(['staff.permissions.manage']);
    });

    it('reports not dirty when sets are identical', () => {
      const orig = ['orders.read', 'staff.read'];
      const draft = ['staff.read', 'orders.read'];

      const diff = getDraftChanges(orig, draft);
      expect(diff.isDirty).toBe(false);
      expect(diff.added).toEqual([]);
      expect(diff.removed).toEqual([]);
    });
  });
});
