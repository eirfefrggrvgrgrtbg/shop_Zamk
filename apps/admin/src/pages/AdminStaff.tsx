import { useState, useEffect, useCallback } from 'react';
import {
  listStaffMembers,
  listStaffRoles,
  createStaffMember,
  updateStaffRole,
  updateStaffStatus,
  resetStaffPassword,
} from '@zamk/api-client/src/admin';
import type { StaffMemberView, StaffRoleWithPermissions } from '@zamk/api-client/src/types';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import { AlertCircle, Plus, CheckCircle2, Users, RefreshCw, Copy } from 'lucide-react';

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
          Роль: <span className="font-medium">{ROLE_NAMES[roleCode] ?? roleCode}</span>
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
  const { isOwner, user: currentUser } = useAdminAuth();

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

  // Change role modal
  const [changeRoleTarget, setChangeRoleTarget] = useState<StaffMemberView | null>(null);
  const [changeRoleCode, setChangeRoleCode] = useState('');
  const [isChangingRole, setIsChangingRole] = useState(false);
  const [changeRoleError, setChangeRoleError] = useState<string | null>(null);

  // Reset password modal
  const [resetTarget, setResetTarget] = useState<StaffMemberView | null>(null);
  const [resetPassword, setResetPasswordValue] = useState('');
  const [isResetting, setIsResetting] = useState(false);
  const [resetError, setResetError] = useState<string | null>(null);

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

  // ---- Status handler ----

  const handleStatusChange = async (member: StaffMemberView, newStatus: string) => {
    const statusLabels: Record<string, string> = {
      blocked: 'заблокирован',
      archived: 'архивирован',
    };
    const confirmMsg = newStatus === 'blocked' || newStatus === 'archived'
      ? `Вы уверены, что хотите перевести «${member.name}» в статус «${statusLabels[newStatus] ?? newStatus}»?`
      : null;

    if (confirmMsg && !window.confirm(confirmMsg)) return;

    try {
      await updateStaffStatus(member.userId, { status: newStatus as any });
      setMembers(prev => prev.map(m => m.userId === member.userId ? { ...m, staffStatus: newStatus } : m));
    } catch (err: any) {
      alert(err.message || 'Не удалось обновить статус');
      loadData();
    }
  };

  // ---- Change role handler ----

  const openChangeRole = (member: StaffMemberView) => {
    setChangeRoleTarget(member);
    setChangeRoleCode(member.roleCode);
    setChangeRoleError(null);
  };

  const handleChangeRole = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!changeRoleTarget) return;
    setIsChangingRole(true);
    setChangeRoleError(null);
    try {
      await updateStaffRole(changeRoleTarget.userId, { roleCode: changeRoleCode });
      setChangeRoleTarget(null);
      loadData();
    } catch (err: any) {
      setChangeRoleError(err.message || 'Не удалось изменить роль');
    } finally {
      setIsChangingRole(false);
    }
  };

  // ---- Reset password handler ----

  const openResetPassword = (member: StaffMemberView) => {
    setResetTarget(member);
    setResetPasswordValue('');
    setResetError(null);
  };

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resetTarget) return;
    setIsResetting(true);
    setResetError(null);
    const localPwd = resetPassword;
    try {
      await resetStaffPassword(resetTarget.userId, { temporaryPassword: localPwd });
      const target = resetTarget;
      setResetTarget(null);
      setResetPasswordValue('');
      // Show password one time from local variable
      setSuccessPassword({ email: target.email, roleCode: target.roleCode, password: localPwd });
    } catch (err: any) {
      setResetError(err.message || 'Не удалось сбросить пароль');
    } finally {
      setIsResetting(false);
    }
  };

  // ---- Helpers ----

  const canModifyOwner = isOwner();

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Сотрудники</h1>
          <p className="mt-1 text-sm text-gray-500">Управление доступом сотрудников платформы</p>
        </div>
        <button
          onClick={() => { setIsCreateOpen(true); setCreateError(null); }}
          className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
        >
          <Plus className="-ml-1 mr-2 h-5 w-5" />
          Создать доступ
        </button>
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
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Имя / Email</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Роль</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Статус</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Дата создания</th>
                <th className="relative px-6 py-3"><span className="sr-only">Действия</span></th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {members.map((m) => {
                const isCurrentUser = m.userId === currentUser?.id;
                const isOwnerRow = m.roleCode === 'owner';
                const canModify = isOwnerRow ? canModifyOwner : true;

                return (
                  <tr key={m.userId}>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900">{m.name}</div>
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
                      <div className="flex items-center justify-end space-x-2">
                        {canModify && (
                          <button
                            onClick={() => openChangeRole(m)}
                            className="text-indigo-600 hover:text-indigo-900"
                          >
                            Сменить роль
                          </button>
                        )}
                        {m.staffStatus !== 'active' && !isCurrentUser && canModify && (
                          <button onClick={() => handleStatusChange(m, 'active')} className="text-green-600 hover:text-green-900">
                            Восстановить
                          </button>
                        )}
                        {m.staffStatus === 'active' && !isCurrentUser && canModify && (
                          <button onClick={() => handleStatusChange(m, 'blocked')} className="text-red-600 hover:text-red-900">
                            Заблокировать
                          </button>
                        )}
                        {m.staffStatus !== 'archived' && !isCurrentUser && canModify && (
                          <button onClick={() => handleStatusChange(m, 'archived')} className="text-gray-500 hover:text-gray-800">
                            Архивировать
                          </button>
                        )}
                        <button onClick={() => openResetPassword(m)} className="text-gray-500 hover:text-gray-800 flex items-center gap-1">
                          <RefreshCw className="h-3.5 w-3.5" />
                          Пароль
                        </button>
                      </div>
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
                <label className="block text-sm font-medium text-gray-700">Роль *</label>
                <select required value={newRoleCode} onChange={e => setNewRoleCode(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500">
                  <option value="">Выберите роль</option>
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

      {/* Change Role Modal */}
      {changeRoleTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-sm shadow-xl">
            <h2 className="text-lg font-bold mb-4">Сменить роль</h2>
            {changeRoleError && (
              <div className="mb-3 p-3 bg-red-50 text-red-700 text-sm rounded">{changeRoleError}</div>
            )}
            <form onSubmit={handleChangeRole} className="space-y-4">
              <div>
                <p className="text-sm text-gray-600">
                  Текущая роль: <span className="font-medium">{ROLE_NAMES[changeRoleTarget.roleCode] ?? changeRoleTarget.roleName}</span>
                </p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Новая роль</label>
                <select required value={changeRoleCode} onChange={e => setChangeRoleCode(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500">
                  {roles.map(r => (
                    <option key={r.code} value={r.code}>{ROLE_NAMES[r.code] ?? r.name}</option>
                  ))}
                </select>
              </div>
              <div className="flex justify-end space-x-3">
                <button type="button" onClick={() => setChangeRoleTarget(null)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">
                  Отмена
                </button>
                <button type="submit" disabled={isChangingRole}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                  {isChangingRole ? 'Сохранение...' : 'Сохранить'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Reset Password Modal */}
      {resetTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-sm shadow-xl">
            <h2 className="text-lg font-bold mb-1">Сбросить пароль</h2>
            <p className="text-sm text-gray-500 mb-4">{resetTarget.email}</p>
            {resetError && (
              <div className="mb-3 p-3 bg-red-50 text-red-700 text-sm rounded">{resetError}</div>
            )}
            <form onSubmit={handleResetPassword} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Временный пароль *</label>
                <div className="mt-1 flex gap-2">
                  <input required type="text" minLength={8} value={resetPassword} onChange={e => setResetPasswordValue(e.target.value)}
                    placeholder="Минимум 8 символов"
                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-indigo-500 focus:border-indigo-500" />
                  <button type="button" onClick={() => setResetPasswordValue(generatePassword())}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-600 hover:bg-gray-50">
                    Сгенерировать
                  </button>
                </div>
              </div>
              <div className="flex justify-end space-x-3">
                <button type="button" onClick={() => setResetTarget(null)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">
                  Отмена
                </button>
                <button type="submit" disabled={isResetting}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                  {isResetting ? 'Сброс...' : 'Сбросить пароль'}
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
