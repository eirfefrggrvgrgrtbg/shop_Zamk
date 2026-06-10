import { useState, useEffect } from 'react';
import { listStaffRoles } from '@zamk/api-client/src/admin';
import type { StaffRoleWithPermissions } from '@zamk/api-client/src/types';
import { AlertCircle, ChevronDown, ChevronRight, Shield } from 'lucide-react';

const ROLE_NAMES: Record<string, string> = {
  owner: 'Владелец',
  co_owner: 'Со-владелец',
  manager: 'Менеджер',
  finance_manager: 'Финансовый менеджер',
  moderator: 'Модератор',
  support: 'Поддержка',
  logistics: 'Логистика',
  analyst: 'Аналитик',
};

const PERMISSION_DOMAINS: { label: string; prefix: string[] }[] = [
  { label: 'Продавцы', prefix: ['sellers.'] },
  { label: 'Товары', prefix: ['products.', 'categories.', 'brands.'] },
  { label: 'Заказы', prefix: ['orders.'] },
  { label: 'Финансы', prefix: ['payments.', 'refunds.', 'payouts.'] },
  { label: 'Склад', prefix: ['inventory.', 'shipments.'] },
  { label: 'Модерация', prefix: ['reviews.', 'complaints.'] },
  { label: 'Сотрудники', prefix: ['staff.', 'roles.'] },
  { label: 'Поддержка', prefix: ['support.'] },
  { label: 'Система', prefix: ['settings.', 'audit.', 'storefront.', 'commission.', 'analytics.', 'exports.'] },
];

function groupPermissions(perms: string[]): Record<string, string[]> {
  const groups: Record<string, string[]> = {};
  for (const perm of perms) {
    const domain = PERMISSION_DOMAINS.find(d => d.prefix.some(p => perm.startsWith(p)));
    const label = domain?.label ?? 'Прочее';
    if (!groups[label]) groups[label] = [];
    groups[label].push(perm);
  }
  return groups;
}

function RoleCard({ role }: { role: StaffRoleWithPermissions }) {
  const [expanded, setExpanded] = useState(false);
  const grouped = groupPermissions(role.permissions);
  const domainCount = Object.keys(grouped).length;

  return (
    <div className="border border-gray-200 rounded-lg bg-white overflow-hidden">
      <button
        onClick={() => setExpanded(e => !e)}
        className="w-full flex items-center justify-between px-5 py-4 hover:bg-gray-50 transition-colors text-left"
      >
        <div className="flex items-center gap-3">
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-100 text-indigo-800">
            {ROLE_NAMES[role.code] ?? role.name}
          </span>
          <span className="text-sm text-gray-500 font-mono">{role.code}</span>
          {role.description && (
            <span className="hidden sm:block text-sm text-gray-400">{role.description}</span>
          )}
        </div>
        <div className="flex items-center gap-3 text-sm text-gray-500">
          <span>{role.permissions.length} разрешений · {domainCount} разделов</span>
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </div>
      </button>

      {expanded && (
        <div className="px-5 pb-5 border-t border-gray-100">
          {Object.keys(grouped).length === 0 ? (
            <p className="text-sm text-gray-400 pt-3">Нет разрешений</p>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 pt-4">
              {PERMISSION_DOMAINS.map(({ label }) => {
                const perms = grouped[label];
                if (!perms) return null;
                return (
                  <div key={label}>
                    <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">{label}</p>
                    <ul className="space-y-1">
                      {perms.map(p => (
                        <li key={p} className="text-xs font-mono text-gray-700 bg-gray-50 px-2 py-1 rounded">
                          {p}
                        </li>
                      ))}
                    </ul>
                  </div>
                );
              })}
              {grouped['Прочее'] && (
                <div>
                  <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">Прочее</p>
                  <ul className="space-y-1">
                    {grouped['Прочее'].map(p => (
                      <li key={p} className="text-xs font-mono text-gray-700 bg-gray-50 px-2 py-1 rounded">{p}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function AdminRoles() {
  const [roles, setRoles] = useState<StaffRoleWithPermissions[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const data = await listStaffRoles();
        setRoles(data.items ?? []);
      } catch (err: any) {
        setError(err.message || 'Не удалось загрузить роли');
      } finally {
        setIsLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Доступы и роли</h1>
          <p className="mt-1 text-sm text-gray-500">Системные роли и их разрешения (только просмотр)</p>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2 shrink-0" />
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-2 text-sm text-gray-500">Загрузка ролей...</p>
        </div>
      ) : roles.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Shield className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Роли не найдены</h3>
        </div>
      ) : (
        <div className="space-y-3">
          {roles.map(role => <RoleCard key={role.id} role={role} />)}
        </div>
      )}
    </div>
  );
}
