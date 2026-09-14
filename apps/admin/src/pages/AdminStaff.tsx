import { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import {
  listStaffMembers,
  listStaffRoles,
  createStaffMember,
} from '@zamk/api-client/src/admin';
import type { StaffMemberView, StaffRoleWithPermissions } from '@zamk/api-client/src/types';
import { AlertCircle, Plus, CheckCircle2, Users, Copy } from 'lucide-react';
import { PermissionGuard } from '../components/PermissionGuard';

// ---- Constants ----

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
  archived: 'Архивирован',
};

const STAFF_STATUS_BADGE: Record<string, string> = {
  active: 'bg-green-100 text-green-800',
  blocked: 'bg-red-100 text-red-800',
  archived: 'bg-gray-100 text-gray-700',
};

function generatePassword(): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';
  return Array.from({ length: 12 }, () => chars[Math.floor(Math.random() * chars.length)]).join('');
}

// ---- Subcomponents ----

function PasswordSuccessModal({ email, roleCode, password, onClose }: {
  email: string;
  roleCode: string;
  password: string;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);

  const copyPassword = async () => {
    try {
      await navigator.clipboard.writeText(password);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white rounded-lg p-6 w-full max-w-md shadow-xl">
        <div className="flex items-center text-green-600 mb-4">
          <CheckCircle2 className="h-8 w-8 mr-2" />
          <h2 className="text-xl font-bold text-gray-900">Доступ создан</h2>
        </div>
        <p className="text-sm text-gray-600 mb-1">Пользователь: <span className="font-medium">{email}</span></p>
        <p className="text-sm text-gray-600 mb-3">
          Шаблон доступа: <span className="font-medium">{ROLE_NAMES[roleCode] ?? roleCode}</span>
        </p>
        <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2 mb-3">
          Передайте пароль пользователю надёжным способом. При первом входе он будет обязан сменить пароль.
        </p>
        <p className="text-sm font-medium text-gray-700 mb-2">
          Временный пароль (показывается только один раз):
        </p>
        <div className="bg-gray-100 p-4 rounded text-center mb-4 border border-gray-200 flex items-center justify-between">
          <code className="text-lg font-mono font-bold text-gray-900 select-all flex-1">{password}</code>
          <button onClick={copyPassword} className="ml-3 text-gray-400 hover:text-gray-600" title="Скопировать">
            {copied ? <CheckCircle2 className="h-5 w-5 text-green-500" /> : <Copy className="h-5 w-5" />}
          </button>
        </div>
        <button
          onClick={onClose}
          className="w-full px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700"
        >
          Понятно
        </button>
      </div>
    </div>
  );
}

// ---- Main Page ----

export function AdminStaff() {
  const [members, setMembers] = useState<StaffMemberView[]>([]);
  const [roles, setRoles] = useState<StaffRoleWithPermissions[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Create modal
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [newName, setNewName] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPhone, setNewPhone] = useState('');
  const [newRoleCode, setNewRoleCode] = useState('');
  const [newPassword, setNewPassword] = useState('');

  // Password success modal
  const [successPassword, setSuccessPassword] = useState<{ email: string; roleCode: string; password: string } | null>(null);

  const loadData = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const [membersRes, rolesRes] = await Promise.all([listStaffMembers(), listStaffRoles()]);
      setMembers(membersRes.items ?? []);
      setRoles(rolesRes.items ?? []);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить данные');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  // ---- Create handler ----

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsCreating(true);
    setCreateError(null);
    const localPassword = newPassword;
    try {
      const result = await createStaffMember({
        name: newName,
        email: newEmail,
        phone: newPhone || undefined,
        roleCode: newRoleCode,
        temporaryPassword: localPassword,
      });
      setNewName(''); setNewEmail(''); setNewPhone(''); setNewRoleCode(''); setNewPassword('');
      setIsCreateOpen(false);
      // Show password only from local variable — never from backend response
      setSuccessPassword({ email: result.email, roleCode: result.roleCode, password: localPassword });
      loadData();
    } catch (err: any) {
      setCreateError(err.message || 'Не удалось создать сотрудника');
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Сотрудники</h1>
          <p className="mt-1 text-sm text-gray-500">Справочник сотрудников платформы</p>
        </div>
        <PermissionGuard permission="staff.create">
          <button
            onClick={() => { setIsCreateOpen(true); setCreateError(null); }}
            className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
          >
            <Plus className="-ml-1 mr-2 h-5 w-5" />
            Создать доступ
          </button>
        </PermissionGuard>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-2 text-sm text-gray-500">Загрузка сотрудников...</p>
        </div>
      ) : members.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Users className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Сотрудников нет</h3>
          <p className="mt-1 text-sm text-gray-500">Создайте первый доступ сотрудника.</p>
        </div>
      ) : (
        <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Сотрудник</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Шаблон доступа</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Дата создания</th>
                <th className="relative px-6 py-3"><span className="sr-only">Открыть</span></th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {members.map((m) => {
                return (
                  <tr key={m.userId}>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div>
                        <Link
                          to={`/staff/${m.userId}`}
                          data-testid={`staff-link-${m.userId}`}
                          className="text-sm font-medium text-indigo-600 hover:text-indigo-900 hover:underline"
                        >
                          {m.name}
                        </Link>
                      </div>
                      <div className="text-xs text-gray-500">{m.email}</div>
                      {m.mustChangePassword && (
                        <span className="inline-block mt-1 text-xs text-amber-700 bg-amber-50 px-1.5 py-0.5 rounded">смена пароля</span>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-indigo-100 text-indigo-800">
                        {ROLE_NAMES[m.roleCode] ?? m.roleName}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${STAFF_STATUS_BADGE[m.staffStatus] ?? 'bg-gray-100 text-gray-700'}`}>
                        {STAFF_STATUS_LABELS[m.staffStatus] ?? m.staffStatus}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {new Date(m.createdAt).toLocaleDateString('ru-RU')}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <Link
                        to={`/staff/${m.userId}`}
                        data-testid={`staff-open-link-${m.userId}`}
                        className="text-xs font-medium text-indigo-600 hover:text-indigo-900 hover:underline inline-flex items-center gap-1"
                      >
                        Открыть &rarr;
                      </Link>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Staff Modal */}
      {isCreateOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md shadow-xl">
            <h2 className="text-xl font-bold mb-1">Создать доступ сотрудника</h2>
            <p className="text-sm text-gray-500 mb-4">После создания сотрудник получит временный пароль и должен будет его сменить при первом входе.</p>

            {createError && (
              <div className="mb-4 p-3 bg-red-50 text-red-700 text-sm rounded flex items-start">
                <AlertCircle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
                <span>{createError}</span>
              </div>
            )}

            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Имя *</label>
                <input required type="text" value={newName} onChange={e => setNewName(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Email *</label>
                <input required type="email" value={newEmail} onChange={e => setNewEmail(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Телефон</label>
                <input type="tel" value={newPhone} onChange={e => setNewPhone(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Шаблон доступа *</label>
                <select required value={newRoleCode} onChange={e => setNewRoleCode(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500">
                  <option value="">Выберите шаблон доступа</option>
                  {roles.map(r => (
                    <option key={r.code} value={r.code}>{ROLE_NAMES[r.code] ?? r.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Временный пароль *</label>
                <div className="mt-1 flex gap-2">
                  <input required type="text" minLength={8} value={newPassword} onChange={e => setNewPassword(e.target.value)}
                    placeholder="Минимум 8 символов"
                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
                  <button type="button" onClick={() => setNewPassword(generatePassword())}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-600 hover:bg-gray-50">
                    Сгенерировать
                  </button>
                </div>
                <p className="mt-1 text-xs text-gray-400">Пароль показывается один раз. Сохраните его перед закрытием окна.</p>
              </div>
              <div className="mt-5 flex justify-end space-x-3">
                <button type="button" onClick={() => setIsCreateOpen(false)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">
                  Отмена
                </button>
                <button type="submit" disabled={isCreating}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                  {isCreating ? 'Создание...' : 'Создать доступ'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Password success modal */}
      {successPassword && (
        <PasswordSuccessModal
          email={successPassword.email}
          roleCode={successPassword.roleCode}
          password={successPassword.password}
          onClose={() => setSuccessPassword(null)}
        />
      )}
    </div>
  );
}
