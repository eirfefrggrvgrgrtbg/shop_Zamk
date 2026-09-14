import { useState, useEffect, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  listStaffMembers,
  listStaffRoles,
  getStaffMemberPermissions,
} from '@zamk/api-client/src/admin';
import type { StaffMemberView, StaffRoleWithPermissions } from '@zamk/api-client/src/types';
import {
  STAFF_CAPABILITY_GROUPS,
  getCapabilitiesByGroup,
} from '../config/staffCapabilities';
import {
  ArrowLeft,
  AlertCircle,
  AlertTriangle,
  Search,
  Check,
  ChevronDown,
  ChevronRight,
  Shield,
  ShieldAlert,
} from 'lucide-react';

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

const STAFF_STATUS_LABELS: Record<string, string> = {
  active: 'Активен',
  blocked: 'Заблокирован',
  archived: 'В архиве',
};

const STAFF_STATUS_BADGE: Record<string, string> = {
  active: 'bg-green-100 text-green-800',
  blocked: 'bg-red-100 text-red-800',
  archived: 'bg-gray-100 text-gray-700',
};

function arePermissionSetsEqual(setA: string[], setB: string[]): boolean {
  const a = new Set(setA);
  const b = new Set(setB);
  if (a.size !== b.size) return false;
  for (const item of a) {
    if (!b.has(item)) return false;
  }
  return true;
}

function formatRightsCount(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 14) return `${n} прав`;
  if (mod10 === 1) return `${n} право`;
  if (mod10 >= 2 && mod10 <= 4) return `${n} права`;
  return `${n} прав`;
}

export function AdminStaffDetail() {
  const { userId } = useParams<{ userId: string }>();

  const [member, setMember] = useState<StaffMemberView | null>(null);
  const [roles, setRoles] = useState<StaffRoleWithPermissions[]>([]);
  const [directPermissions, setDirectPermissions] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [errorType, setErrorType] = useState<'none' | 'not_found' | 'forbidden' | 'generic'>('none');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [viewMode, setViewMode] = useState<'assigned' | 'all'>('assigned');
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());

  useEffect(() => {
    async function loadData() {
      if (!userId) {
        setErrorType('not_found');
        setIsLoading(false);
        return;
      }
      setIsLoading(true);
      setErrorType('none');
      setErrorMessage(null);

      try {
        const [membersRes, rolesRes, permsRes] = await Promise.all([
          listStaffMembers(),
          listStaffRoles(),
          getStaffMemberPermissions(userId),
        ]);

        const found = membersRes.items?.find((m) => m.userId === userId);
        if (!found) {
          setErrorType('not_found');
          setIsLoading(false);
          return;
        }

        setMember(found);
        setRoles(rolesRes.items || []);
        setDirectPermissions(permsRes.permissions || []);
      } catch (err: any) {
        if (err?.status === 403 || err?.code === 'forbidden') {
          setErrorType('forbidden');
        } else if (err?.status === 404 || err?.code === 'not_found') {
          setErrorType('not_found');
        } else {
          setErrorType('generic');
          setErrorMessage(err?.message || 'Не удалось загрузить данные сотрудника.');
        }
      } finally {
        setIsLoading(false);
      }
    }

    loadData();
  }, [userId]);

  const directSet = useMemo(() => new Set(directPermissions), [directPermissions]);

  const currentRole = useMemo(() => {
    if (!member) return null;
    return roles.find((r) => r.code === member.roleCode || r.id === member.roleId) || null;
  }, [member, roles]);

  const isPresetMatch = useMemo(() => {
    if (!currentRole) return false;
    return arePermissionSetsEqual(currentRole.permissions || [], directPermissions);
  }, [currentRole, directPermissions]);

  const normalizedQuery = searchQuery.trim().toLowerCase();

  const toggleGroup = (groupKey: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(groupKey)) {
        next.delete(groupKey);
      } else {
        next.add(groupKey);
      }
      return next;
    });
  };

  // Grouped data model
  const groupsData = useMemo(() => {
    return STAFF_CAPABILITY_GROUPS.map((group) => {
      const allInGroup = getCapabilitiesByGroup(group.key);
      const assignedInGroup = allInGroup.filter((c) => directSet.has(c.key));
      const selectedCount = assignedInGroup.length;

      // When search query is present, search all capabilities
      const searchMatches = normalizedQuery
        ? allInGroup.filter(
            (c) =>
              c.title.toLowerCase().includes(normalizedQuery) ||
              c.key.toLowerCase().includes(normalizedQuery) ||
              (c.description && c.description.toLowerCase().includes(normalizedQuery))
          )
        : [];

      return {
        group,
        allInGroup,
        assignedInGroup,
        selectedCount,
        searchMatches,
      };
    });
  }, [directSet, normalizedQuery]);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div>
          <Link
            to="/staff"
            className="inline-flex items-center text-sm font-medium text-indigo-600 hover:text-indigo-900"
          >
            <ArrowLeft className="mr-1 h-4 w-4" /> К сотрудникам
          </Link>
        </div>
        <div className="text-center py-16 bg-white rounded-lg shadow-sm border border-gray-200">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-3 text-sm text-gray-500">Загрузка сотрудника...</p>
        </div>
      </div>
    );
  }

  if (errorType === 'forbidden') {
    return (
      <div className="space-y-4">
        <div>
          <Link
            to="/staff"
            className="inline-flex items-center text-sm font-medium text-indigo-600 hover:text-indigo-900"
          >
            <ArrowLeft className="mr-1 h-4 w-4" /> К сотрудникам
          </Link>
        </div>
        <div className="p-6 bg-white rounded-lg shadow-sm border border-gray-200 text-center">
          <ShieldAlert className="mx-auto h-12 w-12 text-amber-500" />
          <h2 className="mt-3 text-lg font-bold text-gray-900">
            У вас нет прав для просмотра прав этого сотрудника.
          </h2>
          <p className="mt-1 text-sm text-gray-500">
            Для просмотра индивидуальных прав требуется разрешение staff.permissions.manage.
          </p>
          <div className="mt-4">
            <Link
              to="/staff"
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
            >
              Назад к сотрудникам
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (errorType === 'generic') {
    return (
      <div className="space-y-4">
        <div>
          <Link
            to="/staff"
            className="inline-flex items-center text-sm font-medium text-indigo-600 hover:text-indigo-900"
          >
            <ArrowLeft className="mr-1 h-4 w-4" /> К сотрудникам
          </Link>
        </div>
        <div className="p-6 bg-white rounded-lg shadow-sm border border-gray-200 text-center">
          <AlertCircle className="mx-auto h-12 w-12 text-red-500" />
          <h2 className="mt-3 text-lg font-bold text-gray-900">
            Не удалось загрузить данные сотрудника.
          </h2>
          {errorMessage && <p className="mt-1 text-sm text-red-600">{errorMessage}</p>}
          <div className="mt-4">
            <Link
              to="/staff"
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
            >
              Назад к сотрудникам
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (errorType === 'not_found' || !member) {
    return (
      <div className="space-y-4">
        <div>
          <Link
            to="/staff"
            className="inline-flex items-center text-sm font-medium text-indigo-600 hover:text-indigo-900"
          >
            <ArrowLeft className="mr-1 h-4 w-4" /> К сотрудникам
          </Link>
        </div>
        <div className="p-6 bg-white rounded-lg shadow-sm border border-gray-200 text-center">
          <AlertCircle className="mx-auto h-12 w-12 text-red-500" />
          <h2 className="mt-3 text-lg font-bold text-gray-900">Сотрудник не найден.</h2>
          <p className="mt-1 text-sm text-gray-500">
            Сотрудник с указанным идентификатором не существует или был удалён.
          </p>
          <div className="mt-4">
            <Link
              to="/staff"
              className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
            >
              Назад к сотрудникам
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const roleTitle = ROLE_NAMES[member.roleCode] ?? currentRole?.name ?? member.roleName;

  return (
    <div className="space-y-5">
      {/* Navigation link */}
      <div>
        <Link
          to="/staff"
          className="inline-flex items-center text-sm font-medium text-indigo-600 hover:text-indigo-900"
        >
          <ArrowLeft className="mr-1 h-4 w-4" /> К сотрудникам
        </Link>
      </div>

      {/* Compact Employee Header */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-xl font-bold text-gray-900">{member.name}</h1>
              <span
                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  STAFF_STATUS_BADGE[member.staffStatus] ?? 'bg-gray-100 text-gray-700'
                }`}
              >
                {STAFF_STATUS_LABELS[member.staffStatus] ?? member.staffStatus}
              </span>
              {member.mustChangePassword && (
                <span className="inline-block text-xs text-amber-700 bg-amber-50 border border-amber-200 px-2 py-0.5 rounded">
                  смена пароля
                </span>
              )}
            </div>
            <p className="text-sm text-gray-500 mt-0.5">{member.email}</p>
          </div>

          <div className="text-xs text-gray-400 sm:text-right">
            Дата создания: {new Date(member.createdAt).toLocaleDateString('ru-RU')}
          </div>
        </div>

        {/* Access summary bar */}
        <div className="mt-4 pt-3 border-t border-gray-100 flex flex-wrap items-center justify-between gap-2 text-sm">
          <div className="flex items-center gap-2.5 flex-wrap">
            <span className="text-gray-600">
              Шаблон: <strong className="font-semibold text-gray-900">{roleTitle}</strong>
            </span>
            {isPresetMatch ? (
              <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                По шаблону
              </span>
            ) : (
              <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">
                Индивидуально настроено
              </span>
            )}
            <span className="text-gray-400">·</span>
            <span className="font-semibold text-indigo-700 bg-indigo-50 px-2 py-0.5 rounded text-xs">
              {formatRightsCount(directPermissions.length)}
            </span>
          </div>

          <div className="text-xs text-gray-400">
            Фактический доступ определяется индивидуальными правами.
          </div>
        </div>

        {/* Owner safety note */}
        {member.roleCode === 'owner' && (
          <div className="mt-3 p-2.5 bg-gray-50 border border-gray-200 rounded text-xs text-gray-600 flex items-center">
            <Shield className="h-3.5 w-3.5 mr-2 text-gray-500 shrink-0" />
            <span>Владелец — системная роль с отдельными ограничениями безопасности.</span>
          </div>
        )}
      </div>

      {/* Blocked / Archived Alert Banner */}
      {member.staffStatus !== 'active' && (
        <div className="p-3 bg-amber-50 border border-amber-200 rounded-lg flex items-center text-sm text-amber-800">
          <AlertTriangle className="h-4 w-4 text-amber-600 mr-2.5 shrink-0" />
          <span>
            {member.staffStatus === 'blocked'
              ? 'Сотрудник заблокирован. Индивидуальные права сохранены, но не дают доступ до восстановления учётной записи.'
              : 'Сотрудник находится в архиве. Индивидуальные права сохранены, но не дают доступ до восстановления учётной записи.'}
          </span>
        </div>
      )}

      {/* Main Read-Only Section */}
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-5 space-y-4">
        {/* Section Header & Toolbar */}
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3 border-b border-gray-100 pb-3">
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-base font-bold text-gray-900">Индивидуальные права</h2>
              <span className="text-xs text-gray-500 font-normal">
                (Фактический доступ сотрудника)
              </span>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
            {/* View Mode Switch */}
            <div className="inline-flex rounded-md shadow-2xs bg-gray-100 p-0.5">
              <button
                type="button"
                onClick={() => setViewMode('assigned')}
                className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
                  viewMode === 'assigned'
                    ? 'bg-white text-gray-900 shadow-2xs font-semibold'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                Назначенные ({directPermissions.length})
              </button>
              <button
                type="button"
                onClick={() => setViewMode('all')}
                className={`px-3 py-1.5 text-xs font-medium rounded transition-colors ${
                  viewMode === 'all'
                    ? 'bg-white text-gray-900 shadow-2xs font-semibold'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                Все права (86)
              </button>
            </div>

            {/* Search Input */}
            <div className="relative w-full sm:w-60">
              <Search className="absolute left-2.5 top-2 h-3.5 w-3.5 text-gray-400" />
              <input
                type="text"
                placeholder="Поиск по правам..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-8 pr-3 py-1.5 border border-gray-300 rounded-md text-xs w-full focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500"
              />
            </div>
          </div>
        </div>

        {/* CONTENT RENDERING */}

        {/* 1. Active Search Query Mode */}
        {normalizedQuery ? (
          <div>
            <div className="mb-3 text-xs text-gray-500">
              Результаты поиска по запросу «<span className="font-semibold text-gray-800">{searchQuery}</span>»:
            </div>
            {groupsData.filter((g) => g.searchMatches.length > 0).length === 0 ? (
              <div className="text-center py-10 bg-gray-50 rounded-lg border border-dashed border-gray-300">
                <p className="text-sm text-gray-500">
                  Ничего не найдено по запросу «<span className="font-medium">{searchQuery}</span>»
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                {groupsData
                  .filter((g) => g.searchMatches.length > 0)
                  .map(({ group, allInGroup, searchMatches }) => (
                    <div
                      key={group.key}
                      className="border border-gray-200 rounded-lg p-4 bg-gray-50/40"
                    >
                      <div className="flex items-center justify-between border-b border-gray-200 pb-2 mb-2.5">
                        <h3 className="text-sm font-bold text-gray-900">{group.title}</h3>
                        <span className="text-xs text-gray-500">
                          {searchMatches.length} из {allInGroup.length}
                        </span>
                      </div>
                      <div className="divide-y divide-gray-100">
                        {searchMatches.map((cap) => {
                          const isSelected = directSet.has(cap.key);
                          return (
                            <div key={cap.key} className="py-2 flex items-start gap-2.5">
                              {isSelected ? (
                                <Check className="h-4 w-4 text-emerald-600 mt-0.5 shrink-0" />
                              ) : (
                                <span className="h-4 w-4 flex items-center justify-center text-gray-300 text-xs font-bold shrink-0">
                                  —
                                </span>
                              )}
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center justify-between gap-1">
                                  <span
                                    className={`text-xs ${
                                      isSelected
                                        ? 'font-semibold text-gray-900'
                                        : 'text-gray-500'
                                    }`}
                                  >
                                    {cap.title}
                                  </span>
                                  {cap.critical && isSelected && (
                                    <span className="text-[10px] font-medium bg-amber-100 text-amber-800 border border-amber-200 px-1.5 rounded">
                                      Критическое
                                    </span>
                                  )}
                                </div>
                                {cap.description && (
                                  <p className="text-[11px] text-gray-400 mt-0.5">
                                    {cap.description}
                                  </p>
                                )}
                                {cap.key === 'staff.permissions.manage' && isSelected && (
                                  <p className="text-[11px] text-amber-700 mt-0.5 font-medium">
                                    ⚠️ Позволяет изменять права доступа других сотрудников.
                                  </p>
                                )}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ))}
              </div>
            )}
          </div>
        ) : viewMode === 'assigned' ? (
          /* 2. Default "Назначенные" Mode */
          directPermissions.length === 0 ? (
            <div className="text-center py-8 bg-gray-50 rounded-lg border border-dashed border-gray-200 p-6">
              <Shield className="mx-auto h-8 w-8 text-gray-300 mb-2" />
              <p className="text-sm font-medium text-gray-700">
                У сотрудника нет индивидуальных прав доступа.
              </p>
              <p className="text-xs text-gray-400 mt-1">
                Все действия на платформе для данного сотрудника заблокированы.
              </p>
              <button
                type="button"
                onClick={() => setViewMode('all')}
                className="mt-3 text-xs text-indigo-600 hover:text-indigo-800 font-medium"
              >
                Посмотреть все доступные права →
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {groupsData
                .filter((g) => g.selectedCount > 0)
                .map(({ group, assignedInGroup, selectedCount }) => (
                  <div
                    key={group.key}
                    className="border border-gray-200 rounded-lg p-4 bg-gray-50/30"
                  >
                    <div className="flex items-center justify-between border-b border-gray-200 pb-2 mb-2">
                      <h3 className="text-sm font-bold text-gray-900">{group.title}</h3>
                      <span className="text-xs font-semibold text-indigo-700 bg-indigo-50 px-2 py-0.5 rounded">
                        {selectedCount}
                      </span>
                    </div>

                    <div className="divide-y divide-gray-100">
                      {assignedInGroup.map((cap) => (
                        <div key={cap.key} className="py-2 flex items-start gap-2.5">
                          <Check className="h-4 w-4 text-emerald-600 mt-0.5 shrink-0" />
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center justify-between gap-1">
                              <span className="text-xs font-medium text-gray-900">
                                {cap.title}
                              </span>
                              {cap.critical && (
                                <span className="text-[10px] font-medium bg-amber-100 text-amber-800 border border-amber-200 px-1.5 rounded">
                                  Критическое
                                </span>
                              )}
                            </div>
                            {cap.description && (
                              <p className="text-[11px] text-gray-500 mt-0.5">
                                {cap.description}
                              </p>
                            )}
                            {cap.key === 'staff.permissions.manage' && (
                              <p className="text-[11px] text-amber-700 mt-0.5 font-medium">
                                ⚠️ Позволяет изменять права доступа других сотрудников.
                              </p>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
            </div>
          )
        ) : (
          /* 3. "Все права" Mode (Collapsible Accordions) */
          <div className="space-y-2.5">
            {groupsData.map(({ group, allInGroup, selectedCount }) => {
              const isExpanded = expandedGroups.has(group.key);
              return (
                <div
                  key={group.key}
                  className="border border-gray-200 rounded-lg bg-white overflow-hidden"
                >
                  <button
                    type="button"
                    onClick={() => toggleGroup(group.key)}
                    className="w-full px-4 py-3 text-left flex items-center justify-between bg-gray-50/60 hover:bg-gray-100/60 transition-colors"
                  >
                    <div className="flex items-center gap-2">
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4 text-gray-500" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-gray-500" />
                      )}
                      <span className="text-sm font-semibold text-gray-900">{group.title}</span>
                    </div>

                    <div className="flex items-center gap-2">
                      <span
                        className={`text-xs px-2 py-0.5 rounded font-medium ${
                          selectedCount > 0
                            ? 'bg-indigo-50 text-indigo-700'
                            : 'bg-gray-100 text-gray-500'
                        }`}
                      >
                        {selectedCount} из {allInGroup.length}
                      </span>
                    </div>
                  </button>

                  {isExpanded && (
                    <div className="px-4 py-2.5 border-t border-gray-100 divide-y divide-gray-100 bg-white">
                      {allInGroup.map((cap) => {
                        const isSelected = directSet.has(cap.key);
                        return (
                          <div key={cap.key} className="py-2 flex items-start gap-2.5">
                            {isSelected ? (
                              <Check className="h-4 w-4 text-emerald-600 mt-0.5 shrink-0" />
                            ) : (
                              <span className="h-4 w-4 flex items-center justify-center text-gray-300 text-xs font-bold shrink-0">
                                —
                              </span>
                            )}
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center justify-between gap-1">
                                <span
                                  className={`text-xs ${
                                    isSelected
                                      ? 'font-semibold text-gray-900'
                                      : 'text-gray-400'
                                  }`}
                                >
                                  {cap.title}
                                </span>
                                {cap.critical && isSelected && (
                                  <span className="text-[10px] font-medium bg-amber-100 text-amber-800 border border-amber-200 px-1.5 rounded">
                                    Критическое
                                  </span>
                                )}
                              </div>
                              {cap.description && (
                                <p className="text-[11px] text-gray-400 mt-0.5">
                                  {cap.description}
                                </p>
                              )}
                              {cap.key === 'staff.permissions.manage' && isSelected && (
                                <p className="text-[11px] text-amber-700 mt-0.5 font-medium">
                                  ⚠️ Позволяет изменять права доступа других сотрудников.
                                </p>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
