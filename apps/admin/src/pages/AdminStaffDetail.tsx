import { useState, useEffect, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  getStaffMember,
  patchStaffMemberProfile,
  listStaffRoles,
  getStaffMemberPermissions,
  updateStaffMemberPermissions,
  updateStaffStatus,
  resetStaffPassword,
} from '@zamk/api-client/src/admin';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import type { StaffMemberDetailResponse, StaffRoleWithPermissions, UpdateStaffMemberProfileRequest } from '@zamk/api-client/src/types';
import {
  STAFF_CAPABILITY_GROUPS,
  getCapabilitiesByGroup,
  getCapabilityDefinition,
} from '../config/staffCapabilities';
import {
  STAFF_SCREEN_ACCESS_RULES,
  getAccessTemplateDiff,
  isScreenVisibleWithPermissions,
} from '../config/staffWorkModules';
import {
  STAFF_ACCESS_SECTIONS,
  STAFF_ACCESS_SECTION_GROUPS,
  SECTION_MODE_LABELS,
  SectionMode,
  getSectionMode,
  applySectionMode,
  toggleSectionAction,
  getDraftChanges,
} from '../config/staffAccessSections';
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
  Lock,
  Eye,
  X,
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
  active: 'bg-emerald-50 text-emerald-700 border border-emerald-200',
  blocked: 'bg-rose-50 text-rose-700 border border-rose-200',
  archived: 'bg-gray-100 text-gray-700 border border-gray-200',
};

const MODE_BADGE_STYLE: Record<SectionMode, string> = {
  CLOSED: 'bg-gray-100 text-gray-500 border border-gray-200',
  VIEW: 'bg-blue-50 text-blue-700 border border-blue-200',
  WORK: 'bg-emerald-50 text-emerald-700 border border-emerald-200',
  CUSTOM: 'bg-indigo-50 text-indigo-700 border border-indigo-200',
};

function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '—';
  try {
    const d = new Date(dateStr);
    return d.toLocaleDateString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return dateStr;
  }
}

function getInitials(name?: string | null, email?: string | null): string {
  if (name && name.trim()) {
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return parts[0].slice(0, 2).toUpperCase();
  }
  if (email && email.trim()) {
    return email.slice(0, 2).toUpperCase();
  }
  return '??';
}


export function generatePassword(): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';
  const randomValues = new Uint32Array(12);
  globalThis.crypto.getRandomValues(randomValues);
  return Array.from(randomValues, (val) => chars[val % chars.length]).join('');
}

export function countUnicodeChars(val?: string | null): number {
  if (!val) return 0;
  return Array.from(val).length;
}

function humanizeAccountError(err: any): string {
  if (!err) return 'Не удалось выполнить действие. Попробуйте ещё раз.';
  const status = err.status || err.statusCode;
  const code = err.code || err.error?.code || err.data?.error?.code;

  if (code === 'last_owner') {
    return 'Невозможно заблокировать или архивировать последнего владельца платформы.';
  }
  if (code === 'last_permission_manager') {
    return 'Невозможно заблокировать сотрудника: это последний сотрудник с правом управления доступом.';
  }
  if (status === 403 || code === 'forbidden') {
    return 'У вас нет права выполнять это действие.';
  }
  if (status === 404 || code === 'not_found') {
    return 'Сотрудник не найден.';
  }
  if (err.message && typeof err.message === 'string' && !err.message.includes('Failed to fetch') && !err.message.includes('NetworkError')) {
    return err.message;
  }
  return 'Не удалось выполнить действие. Попробуйте ещё раз.';
}

const isScreenVisibleWithPerms = isScreenVisibleWithPermissions;

export function AdminStaffDetail() {
  const { userId } = useParams<{ userId: string }>();

  const [member, setMember] = useState<StaffMemberDetailResponse | null>(null);
  const [roles, setRoles] = useState<StaffRoleWithPermissions[]>([]);
  const [originalPermissions, setOriginalPermissions] = useState<string[]>([]);
  const [draftPermissions, setDraftPermissions] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [errorType, setErrorType] = useState<'none' | 'not_found' | 'forbidden' | 'generic'>('none');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { user: currentActor, reloadStaff, hasPermission } = useAdminAuth();
  const currentActorId = currentActor?.id || '';

  const canBlock = Boolean(hasPermission?.('staff.block'));
  const canResetPassword = Boolean(hasPermission?.('staff.update'));
  const canEditProfile = Boolean(hasPermission?.('staff.update'));

  // Profile Tab Editing states (EMP.1D2C)
  const [isProfileEditing, setIsProfileEditing] = useState(false);
  const [draftResponsibilities, setDraftResponsibilities] = useState('');
  const [draftWorkNote, setDraftWorkNote] = useState('');
  const [isProfileSaving, setIsProfileSaving] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileSuccessMessage, setProfileSuccessMessage] = useState<string | null>(null);

  // Account Tab & Modal states (EMP.1D1)
  const [isBlockModalOpen, setIsBlockModalOpen] = useState(false);
  const [isArchiveModalOpen, setIsArchiveModalOpen] = useState(false);
  const [isResetPasswordOpen, setIsResetPasswordOpen] = useState(false);
  const [temporaryPassword, setTemporaryPassword] = useState('');
  const [resetPasswordError, setResetPasswordError] = useState<string | null>(null);
  const [successPassword, setSuccessPassword] = useState<string | null>(null);
  const [isPasswordCopied, setIsPasswordCopied] = useState(false);
  const [isActionLoading, setIsActionLoading] = useState(false);
  const [accountActionError, setAccountActionError] = useState<string | null>(null);
  const [accountActionSuccess, setAccountActionSuccess] = useState<string | null>(null);

  // Persistence & Modal states (EMP.1C3C2R.2A)
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccessMessage, setSaveSuccessMessage] = useState<string | null>(null);
  const [isSelfRevokeConfirmOpen, setIsSelfRevokeConfirmOpen] = useState(false);
  const [isPermissionManagementForbidden, setIsPermissionManagementForbidden] = useState(false);

  // Top tabs: Профиль (default), Доступ, Учётная запись
  const [activeTab, setActiveTab] = useState<'profile' | 'access' | 'account'>('profile');

  // Access split workspace state
  const [selectedSectionKey, setSelectedSectionKey] = useState<string>('orders');
  const [sectionSearch, setSectionSearch] = useState('');

  // Modals
  const [isChangePreviewOpen, setIsChangePreviewOpen] = useState(false);
  const [isMenuPreviewOpen, setIsMenuPreviewOpen] = useState(false);

  // Template diff accordion in Access tab
  const [showTemplateDiff, setShowTemplateDiff] = useState(false);

  // Advanced C2B1 Capability viewer state (Access tab bottom)
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);
  const [advancedSearchQuery, setAdvancedSearchQuery] = useState('');
  const [advancedViewMode, setAdvancedViewMode] = useState<'assigned' | 'all'>('assigned');
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());

  // Technical system info in Account tab
  const [isSystemInfoOpen, setIsSystemInfoOpen] = useState(false);

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

      setIsPermissionManagementForbidden(false);

      try {
        const [memberRes, rolesRes] = await Promise.all([
          getStaffMember(userId),
          listStaffRoles(),
        ]);

        if (!memberRes) {
          setErrorType('not_found');
          setIsLoading(false);
          return;
        }

        setMember(memberRes);
        setRoles(rolesRes.items || []);
      } catch (err: any) {
        if (err?.status === 403 || err?.statusCode === 403) {
          setErrorType('forbidden');
        } else if (err?.status === 404 || err?.statusCode === 404) {
          setErrorType('not_found');
        } else {
          setErrorType('generic');
          setErrorMessage(err?.message || 'Не удалось загрузить данные сотрудника');
        }
        setIsLoading(false);
        return;
      }

      // Load permissions separately: 403 here must not destroy base employee profile
      try {
        const permsRes = await getStaffMemberPermissions(userId);
        const initialPerms = permsRes.permissions || [];
        setOriginalPermissions(initialPerms);
        setDraftPermissions(initialPerms);
      } catch (err: any) {
        if (err?.status === 403 || err?.statusCode === 403) {
          setIsPermissionManagementForbidden(true);
          setOriginalPermissions([]);
          setDraftPermissions([]);
        } else {
          setErrorMessage(err?.message || 'Не удалось загрузить права доступа');
        }
      } finally {
        setIsLoading(false);
      }
    }

    loadData();
  }, [userId]);

  const memberName = useMemo(() => {
    if (!member) return '';
    return member.name || (member as any).fullName || '';
  }, [member]);

  const memberStatus = useMemo(() => {
    if (!member) return 'active';
    return member.staffStatus || (member as any).status || 'active';
  }, [member]);

  const memberRoleId = useMemo(() => {
    if (!member) return '';
    return member.roleId || (member as any).staffRoleId || '';
  }, [member]);

  // Selected role / template
  const assignedRole = useMemo(() => {
    if (!member) return null;
    const targetId = member.roleId || (member as any).staffRoleId;
    return roles.find((r) => r.id === targetId) || null;
  }, [member, roles]);

  const assignedRoleName = useMemo(() => {
    if (!assignedRole) return 'Без роли';
    return (
      assignedRole.name ||
      (assignedRole.code ? ROLE_NAMES[assignedRole.code] : undefined) ||
      assignedRole.code ||
      'Без роли'
    );
  }, [assignedRole]);

  // Template diff based on current draft
  const templateDiff = useMemo(() => {
    const rolePerms = assignedRole?.permissions || [];
    return getAccessTemplateDiff(rolePerms, draftPermissions);
  }, [assignedRole, draftPermissions]);

  // Draft changes vs original
  const draftChanges = useMemo(() => {
    return getDraftChanges(originalPermissions, draftPermissions);
  }, [originalPermissions, draftPermissions]);

  // Selected access section
  const selectedSection = useMemo(() => {
    return (
      STAFF_ACCESS_SECTIONS.find((s) => s.key === selectedSectionKey) ||
      STAFF_ACCESS_SECTIONS[0]
    );
  }, [selectedSectionKey]);

  // Mode of selected section in current draft
  const currentSectionMode = useMemo(() => {
    return getSectionMode(selectedSection, draftPermissions);
  }, [selectedSection, draftPermissions]);

  // Filtered sections for left rail
  const filteredSectionsByGroup = useMemo(() => {
    const q = sectionSearch.trim().toLowerCase();
    return STAFF_ACCESS_SECTION_GROUPS.map((group) => {
      const sections = STAFF_ACCESS_SECTIONS.filter(
        (s) => s.group === group.key && (!q || s.title.toLowerCase().includes(q) || s.description?.toLowerCase().includes(q))
      );
      return {
        ...group,
        sections,
      };
    }).filter((g) => g.sections.length > 0);
  }, [sectionSearch]);

  // Profile dirty state and Unicode counters (EMP.1D2C)
  const isProfileDirty = useMemo(() => {
    if (!isProfileEditing || !member) return false;
    const currentResp = member.responsibilities || '';
    const currentNote = member.workNote || '';
    return draftResponsibilities !== currentResp || draftWorkNote !== currentNote;
  }, [isProfileEditing, member, draftResponsibilities, draftWorkNote]);

  const respCharCount = useMemo(() => countUnicodeChars(draftResponsibilities), [draftResponsibilities]);
  const workNoteCharCount = useMemo(() => countUnicodeChars(draftWorkNote), [draftWorkNote]);
  const isRespExceeded = respCharCount > 4000;
  const isWorkNoteExceeded = workNoteCharCount > 4000;
  const isProfileLimitExceeded = isRespExceeded || isWorkNoteExceeded;

  const handleStartProfileEdit = () => {
    setDraftResponsibilities(member?.responsibilities || '');
    setDraftWorkNote(member?.workNote || '');
    setProfileError(null);
    setProfileSuccessMessage(null);
    setIsProfileEditing(true);
  };

  const handleCancelProfileEdit = () => {
    setDraftResponsibilities(member?.responsibilities || '');
    setDraftWorkNote(member?.workNote || '');
    setProfileError(null);
    setIsProfileEditing(false);
  };

  const handleSaveProfile = async () => {
    if (isProfileSaving || !userId || !member) return;
    if (isProfileLimitExceeded) return;

    const trimmedDraftResp = draftResponsibilities.trim();
    const finalDraftResp = trimmedDraftResp === '' ? null : trimmedDraftResp;

    const trimmedCanonicalResp = (member.responsibilities ?? '').trim();
    const finalCanonicalResp = trimmedCanonicalResp === '' ? null : trimmedCanonicalResp;

    const respChanged = finalDraftResp !== finalCanonicalResp;

    const trimmedDraftNote = draftWorkNote.trim();
    const finalDraftNote = trimmedDraftNote === '' ? null : trimmedDraftNote;

    const trimmedCanonicalNote = (member.workNote ?? '').trim();
    const finalCanonicalNote = trimmedCanonicalNote === '' ? null : trimmedCanonicalNote;

    const noteChanged = finalDraftNote !== finalCanonicalNote;

    // If no effective changes after trim normalization, exit edit mode cleanly without PATCH
    if (!respChanged && !noteChanged) {
      setIsProfileEditing(false);
      setProfileError(null);
      return;
    }

    setIsProfileSaving(true);
    setProfileError(null);

    const payload: UpdateStaffMemberProfileRequest = {};
    if (respChanged) {
      payload.responsibilities = finalDraftResp;
    }
    if (noteChanged) {
      payload.workNote = finalDraftNote;
    }

    try {
      await patchStaffMemberProfile(userId, payload);
      const refreshed = await getStaffMember(userId);
      setMember(refreshed);
      setIsProfileEditing(false);
      setProfileSuccessMessage('Профиль сотрудника обновлён.');
    } catch (err: any) {
      const status = err?.status || err?.statusCode;
      const code = err?.code || err?.error?.code || (typeof err?.error === 'string' ? err.error : undefined);

      if (status === 403 || code === 'forbidden') {
        setProfileError('У вас нет права редактировать профиль сотрудника.');
      } else if (status === 404 || code === 'not_found') {
        setProfileError('Сотрудник не найден.');
      } else if (status === 400 || code === 'validation_error' || code === 'invalid_request') {
        setProfileError(err?.message || 'Проверьте введённые данные.');
      } else {
        setProfileError('Не удалось сохранить профиль. Попробуйте ещё раз.');
      }
    } finally {
      setIsProfileSaving(false);
    }
  };

  // Tab switching with leave protection
  const handleTabChange = (targetTab: 'profile' | 'access' | 'account') => {
    if (targetTab === activeTab) return;
    if (isProfileDirty) {
      const confirmed = window.confirm('Есть несохранённые изменения. Выйти без сохранения?');
      if (!confirmed) return;
      handleCancelProfileEdit();
    } else if (draftChanges.isDirty) {
      const confirmed = window.confirm('Есть несохранённые изменения. Выйти без сохранения?');
      if (!confirmed) return;
      handleCancelDraft();
    }
    setActiveTab(targetTab);
  };

  // Navigation leave protection
  const handleBackNavigation = (e: React.MouseEvent) => {
    if (draftChanges.isDirty || isProfileDirty) {
      const confirmed = window.confirm('Есть несохранённые изменения. Выйти без сохранения?');
      if (!confirmed) {
        e.preventDefault();
      }
    }
  };

  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (draftChanges.isDirty || isProfileDirty) {
        e.preventDefault();
        e.returnValue = 'Есть несохранённые изменения. Выйти без сохранения?';
        return 'Есть несохранённые изменения. Выйти без сохранения?';
      }
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload);
    };
  }, [draftChanges.isDirty, isProfileDirty]);

  // Handle section mode change
  const handleModeSelect = (mode: 'CLOSED' | 'VIEW' | 'WORK') => {
    if (saveSuccessMessage) setSaveSuccessMessage(null);
    if (saveError) setSaveError(null);
    const updated = applySectionMode(selectedSection, mode, draftPermissions);
    setDraftPermissions(updated);
  };

  // Handle single action toggle
  const handleActionToggle = (capability: string) => {
    if (saveSuccessMessage) setSaveSuccessMessage(null);
    if (saveError) setSaveError(null);
    const updated = toggleSectionAction(capability, draftPermissions);
    setDraftPermissions(updated);
  };

  // Cancel draft changes
  const handleCancelDraft = () => {
    setDraftPermissions(originalPermissions);
    setIsChangePreviewOpen(false);
    setSaveError(null);
  };

  // Execute canonical backend PUT persistence
  const executeSave = async () => {
    if (isSaving || !userId) return;
    setIsSaving(true);
    setSaveError(null);

    try {
      // Deterministically sorted full draftPermissions set
      const payload = [...draftPermissions].sort();
      const res = await updateStaffMemberPermissions(userId, { permissions: payload });
      const canonical = res.permissions || [];
      setOriginalPermissions(canonical);
      setDraftPermissions(canonical);
      setSaveSuccessMessage('Доступ сотрудника обновлён.');
      setSaveError(null);
      setIsChangePreviewOpen(false);
      setIsSelfRevokeConfirmOpen(false);

      const removedSelfManage =
        userId === currentActorId &&
        !canonical.includes('staff.permissions.manage');

      if (removedSelfManage) {
        setIsPermissionManagementForbidden(true);
      }

      if (reloadStaff) {
        try {
          await reloadStaff();
        } catch {
          // non-fatal
        }
      }
    } catch (err: any) {
      const status = err?.status || err?.statusCode;
      const code = err?.code || err?.error?.code || (typeof err?.error === 'string' ? err.error : undefined);

      if (status === 404 || code === 'not_found') {
        setErrorType('not_found');
        setIsChangePreviewOpen(false);
        setIsSelfRevokeConfirmOpen(false);
      } else if (status === 403 || code === 'forbidden') {
        setSaveError('У вас больше нет права изменять доступ сотрудников.');
        setIsPermissionManagementForbidden(true);
        setIsChangePreviewOpen(false);
        setIsSelfRevokeConfirmOpen(false);
        // DO NOT call getStaffMemberPermissions as actor lacks staff.permissions.manage
      } else if (status === 409 || code === 'last_permission_manager') {
        setSaveError('Невозможно отключить управление правами. В системе должен оставаться хотя бы один активный сотрудник с правом управления доступом.');
        setIsSelfRevokeConfirmOpen(false);
      } else if (status === 400 || code === 'validation_error') {
        setSaveError('Не удалось сохранить доступ. Проверьте выбранные действия.');
        setIsSelfRevokeConfirmOpen(false);
      } else {
        setSaveError('Не удалось сохранить изменения. Попробуйте ещё раз.');
        setIsSelfRevokeConfirmOpen(false);
      }
    } finally {
      setIsSaving(false);
    }
  };

  const handleSaveClick = () => {
    const isSelfRevoke =
      userId === currentActorId &&
      originalPermissions.includes('staff.permissions.manage') &&
      !draftPermissions.includes('staff.permissions.manage');

    if (isSelfRevoke) {
      setIsSelfRevokeConfirmOpen(true);
      return;
    }

    executeSave();
  };

  // Toggle group expansion in Advanced capability viewer
  const toggleGroupExpanded = (groupKey: string) => {
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


  // ---- Account Management Handlers (EMP.1D1) ----

  const copyPasswordToClipboard = async () => {
    if (!successPassword) return;
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(successPassword);
        setIsPasswordCopied(true);
        setTimeout(() => setIsPasswordCopied(false), 2000);
      }
    } catch {
      // Fallback or ignore
    }
  };

  const openResetPasswordModal = () => {
    setTemporaryPassword('');
    setResetPasswordError(null);
    setIsResetPasswordOpen(true);
  };

  const handleResetPasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!member || !userId) return;
    if (temporaryPassword.length < 8) {
      setResetPasswordError('Пароль должен содержать минимум 8 символов');
      return;
    }
    setIsActionLoading(true);
    setResetPasswordError(null);
    const pwdToSet = temporaryPassword;
    try {
      await resetStaffPassword(userId, { temporaryPassword: pwdToSet });
      setIsResetPasswordOpen(false);
      setTemporaryPassword('');
      setSuccessPassword(pwdToSet);
      setMember((prev) => (prev ? { ...prev, mustChangePassword: true } : prev));
    } catch (err: any) {
      setResetPasswordError(humanizeAccountError(err));
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleBlockConfirm = async () => {
    if (!member || !userId) return;
    setIsActionLoading(true);
    setAccountActionError(null);
    try {
      await updateStaffStatus(userId, { status: 'blocked' });
      setMember((prev) => (prev ? { ...prev, staffStatus: 'blocked', status: 'blocked' } : prev));
      setIsBlockModalOpen(false);
      setAccountActionSuccess('Сотрудник успешно заблокирован');
      setTimeout(() => setAccountActionSuccess(null), 4000);
    } catch (err: any) {
      setAccountActionError(humanizeAccountError(err));
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleArchiveConfirm = async () => {
    if (!member || !userId) return;
    setIsActionLoading(true);
    setAccountActionError(null);
    try {
      await updateStaffStatus(userId, { status: 'archived' });
      setMember((prev) => (prev ? { ...prev, staffStatus: 'archived', status: 'archived' } : prev));
      setIsArchiveModalOpen(false);
      setAccountActionSuccess('Сотрудник переведён в архив');
      setTimeout(() => setAccountActionSuccess(null), 4000);
    } catch (err: any) {
      setAccountActionError(humanizeAccountError(err));
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleUnblock = async () => {
    if (!member || !userId) return;
    setIsActionLoading(true);
    setAccountActionError(null);
    try {
      await updateStaffStatus(userId, { status: 'active' });
      setMember((prev) => (prev ? { ...prev, staffStatus: 'active', status: 'active' } : prev));
      setAccountActionSuccess('Сотрудник успешно разблокирован');
      setTimeout(() => setAccountActionSuccess(null), 4000);
    } catch (err: any) {
      setAccountActionError(humanizeAccountError(err));
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleRestoreFromArchive = async () => {
    if (!member || !userId) return;
    setIsActionLoading(true);
    setAccountActionError(null);
    try {
      await updateStaffStatus(userId, { status: 'active' });
      setMember((prev) => (prev ? { ...prev, staffStatus: 'active', status: 'active' } : prev));
      setAccountActionSuccess('Сотрудник успешно восстановлен из архива');
      setTimeout(() => setAccountActionSuccess(null), 4000);
    } catch (err: any) {
      setAccountActionError(humanizeAccountError(err));
    } finally {
      setIsActionLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="p-8 max-w-7xl mx-auto">
        <div className="animate-pulse space-y-6">
          <div className="h-6 w-32 bg-gray-200 rounded" />
          <div className="h-20 bg-gray-100 rounded-lg" />
          <div className="h-10 w-96 bg-gray-200 rounded" />
          <div className="h-96 bg-gray-50 rounded-lg" />
        </div>
      </div>
    );
  }

  if (errorType === 'forbidden') {
    return (
      <div className="p-8 max-w-7xl mx-auto" data-testid="error-forbidden">
        <div className="bg-amber-50 border border-amber-200 rounded-lg p-6 flex items-start gap-4">
          <AlertTriangle className="w-6 h-6 text-amber-600 shrink-0 mt-0.5" />
          <div>
            <h2 className="text-lg font-semibold text-amber-900">Доступ запрещен</h2>
            <p className="text-sm text-amber-800 mt-1">
              У вас нет прав для просмотра профилей сотрудников.
            </p>
            <Link
              to="/staff"
              className="inline-flex items-center gap-1 text-sm font-medium text-amber-900 underline mt-4"
            >
              <ArrowLeft className="w-4 h-4" /> Вернуться к списку
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (errorType === 'not_found' || !member) {
    return (
      <div className="p-8 max-w-7xl mx-auto" data-testid="error-not-found">
        <div className="bg-gray-50 border border-gray-200 rounded-lg p-6 flex items-start gap-4">
          <AlertCircle className="w-6 h-6 text-gray-500 shrink-0 mt-0.5" />
          <div>
            <h2 className="text-lg font-semibold text-gray-900">Сотрудник не найден</h2>
            <p className="text-sm text-gray-600 mt-1">
              Сотрудник с ID <span className="font-mono text-xs">{userId}</span> не существует или был удален.
            </p>
            <Link
              to="/staff"
              className="inline-flex items-center gap-1 text-sm font-medium text-gray-900 underline mt-4"
            >
              <ArrowLeft className="w-4 h-4" /> Вернуться к списку
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (errorType === 'generic') {
    return (
      <div className="p-8 max-w-7xl mx-auto" data-testid="error-generic">
        <div className="bg-rose-50 border border-rose-200 rounded-lg p-6 flex items-start gap-4">
          <AlertCircle className="w-6 h-6 text-rose-600 shrink-0 mt-0.5" />
          <div>
            <h2 className="text-lg font-semibold text-rose-900">Ошибка загрузки</h2>
            <p className="text-sm text-rose-700 mt-1">
              {errorMessage || 'Произошла непредвиденная ошибка при загрузке.'}
            </p>
            <Link
              to="/staff"
              className="inline-flex items-center gap-1 text-sm font-medium text-rose-900 underline mt-4"
            >
              <ArrowLeft className="w-4 h-4" /> Вернуться к списку
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const standardActions = selectedSection.actions.filter((a) => !a.isAdditional);
  const additionalActions = selectedSection.actions.filter((a) => a.isAdditional);

  return (
    <div className="p-6 max-w-[1600px] mx-auto space-y-6 pb-24 font-sans antialiased text-gray-900">
      {/* Breadcrumb Navigation */}
      <div>
        <Link
          to="/staff"
          onClick={handleBackNavigation}
          className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" /> К списку сотрудников
        </Link>
      </div>

      {/* Blocked Account Banner */}
      {memberStatus === 'blocked' && (
        <div
          data-testid="blocked-banner"
          className="bg-rose-50 border border-rose-200 text-rose-800 px-4 py-3 rounded-lg flex items-center gap-3 text-sm"
        >
          <Lock className="w-5 h-5 text-rose-600 shrink-0" />
          <div>
            <div>
              <span className="font-semibold">Учётная запись заблокирована.</span> Доступ к платформе
              приостановлен.
            </div>
            <div className="text-xs text-rose-700 mt-0.5">
              Настройки сохранятся, но доступ не будет действовать до активации учётной записи.
            </div>
          </div>
        </div>
      )}

      {/* Archived Account Banner */}
      {memberStatus === 'archived' && (
        <div
          data-testid="archived-banner"
          className="bg-gray-100 border border-gray-200 text-gray-800 px-4 py-3 rounded-lg flex items-center gap-3 text-sm"
        >
          <Lock className="w-5 h-5 text-gray-600 shrink-0" />
          <div>
            <div>
              <span className="font-semibold">Учётная запись в архиве.</span> Доступ к платформе
              приостановлен.
            </div>
            <div className="text-xs text-gray-600 mt-0.5">
              Настройки сохранятся, но доступ не будет действовать до активации учётной записи.
            </div>
          </div>
        </div>
      )}

      {/* Clean Compact Employee Header */}
      <div className="bg-white border border-gray-200 rounded-lg p-6 shadow-sm flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <div className="w-14 h-14 rounded-full bg-purple-100 text-purple-700 flex items-center justify-center font-semibold text-xl shrink-0">
            {getInitials(memberName, member.email)}
          </div>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-xl font-bold tracking-tight text-gray-900">
                {memberName || 'Без имени'}
              </h1>
              <span
                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  STAFF_STATUS_BADGE[memberStatus] || 'bg-gray-100 text-gray-700'
                }`}
              >
                {STAFF_STATUS_LABELS[memberStatus] || memberStatus}
              </span>
            </div>
            <div className="text-sm text-gray-500 mt-0.5">{member.email}</div>
          </div>
        </div>

        <div className="flex items-center gap-3 bg-gray-50 border border-gray-200 rounded-lg px-4 py-2.5">
          <Shield className="w-4 h-4 text-gray-500" />
          <div className="text-xs">
            <span className="text-gray-500">Шаблон доступа: </span>
            <span className="font-semibold text-gray-900">{assignedRoleName}</span>
          </div>
          {!isPermissionManagementForbidden && (
            <span
              className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                templateDiff.matches
                  ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                  : 'bg-indigo-50 text-indigo-700 border border-indigo-200'
              }`}
            >
              {templateDiff.matches ? 'По шаблону' : 'Индивидуально настроено'}
            </span>
          )}
        </div>
      </div>

      {/* Primary Tab Navigation */}
      <div className="border-b border-gray-200 flex gap-8">
        <button
          role="tab"
          aria-selected={activeTab === 'profile'}
          onClick={() => handleTabChange('profile')}
          className={`pb-3 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'profile'
              ? 'border-purple-600 text-purple-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          }`}
        >
          Профиль
        </button>
        <button
          role="tab"
          aria-selected={activeTab === 'access'}
          onClick={() => handleTabChange('access')}
          className={`pb-3 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'access'
              ? 'border-purple-600 text-purple-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          }`}
        >
          Доступ
        </button>
        <button
          role="tab"
          aria-selected={activeTab === 'account'}
          onClick={() => handleTabChange('account')}
          className={`pb-3 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'account'
              ? 'border-purple-600 text-purple-600'
              : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
          }`}
        >
          Учётная запись
        </button>
      </div>

      {/* TAB 1: ПРОФИЛЬ */}
      {activeTab === 'profile' && (
        <div className="space-y-6" data-testid="tab-profile-content">
          {/* Profile Success Banner */}
          {profileSuccessMessage && (
            <div
              data-testid="profile-success-banner"
              className="bg-emerald-50 border border-emerald-200 text-emerald-800 px-4 py-3 rounded-lg flex items-center justify-between text-sm shadow-sm"
            >
              <div className="flex items-center gap-2">
                <Check className="w-4 h-4 text-emerald-600 shrink-0" />
                <span>{profileSuccessMessage}</span>
              </div>
              <button
                type="button"
                onClick={() => setProfileSuccessMessage(null)}
                className="text-emerald-700 hover:text-emerald-900 p-1"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          )}

          {/* Profile Error Banner */}
          {profileError && (
            <div
              data-testid="profile-error-banner"
              className="bg-rose-50 border border-rose-200 text-rose-800 px-4 py-3 rounded-lg flex items-center justify-between text-sm shadow-sm"
            >
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-rose-600 shrink-0" />
                <span>{profileError}</span>
              </div>
              <button
                type="button"
                onClick={() => setProfileError(null)}
                className="text-rose-700 hover:text-rose-900 p-1"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          )}

          {/* Section 1: О сотруднике */}
          <div className="bg-white border border-gray-200 rounded-lg p-6 space-y-6 shadow-sm">
            <div>
              <h2 className="text-base font-semibold text-gray-900">О сотруднике</h2>
              <p className="text-xs text-gray-500 mt-0.5">
                Основные персональные данные и контактная информация
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-2 border-t border-gray-100">
              <div>
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">
                  ФИО
                </span>
                <span className="text-sm font-medium text-gray-900 mt-1 block">
                  {memberName || '—'}
                </span>
              </div>
              <div>
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">
                  Рабочий email
                </span>
                <span className="text-sm font-medium text-gray-900 mt-1 block">
                  {member.email}
                </span>
              </div>
              <div>
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">
                  Статус
                </span>
                <span className="text-sm font-medium text-gray-900 mt-1 block">
                  {STAFF_STATUS_LABELS[memberStatus] || memberStatus}
                </span>
              </div>
              <div>
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">
                  Дата создания
                </span>
                <span className="text-sm font-medium text-gray-900 mt-1 block">
                  {formatDate(member.createdAt)}
                </span>
              </div>
            </div>
          </div>

          {/* Editable sections: Обязанности and Рабочая заметка */}
          <div data-testid={isProfileEditing ? 'profile-edit-mode' : undefined} className="space-y-6">
            {/* Section 2: Обязанности */}
            <div className="bg-white border border-gray-200 rounded-lg p-6 space-y-4 shadow-sm">
              <div className="flex items-center justify-between">
                <h2 className="text-base font-semibold text-gray-900">Обязанности</h2>
              {!isProfileEditing && canEditProfile && (
                <button
                  type="button"
                  onClick={handleStartProfileEdit}
                  className="inline-flex items-center px-3.5 py-1.5 border border-gray-300 shadow-sm text-xs font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-purple-500"
                >
                  Редактировать
                </button>
              )}
            </div>

            {isProfileEditing ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs text-gray-500">
                    Опишите фактические обязанности сотрудника
                  </span>
                  <span
                    className={`text-xs ${
                      isRespExceeded ? 'text-rose-600 font-semibold' : 'text-gray-500'
                    }`}
                    data-testid="responsibilities-char-counter"
                  >
                    {respCharCount} / 4000
                  </span>
                </div>
                <textarea
                  data-testid="textarea-responsibilities"
                  rows={5}
                  value={draftResponsibilities}
                  onChange={(e) => setDraftResponsibilities(e.target.value)}
                  placeholder="Обязанности сотрудника..."
                  className={`w-full rounded-md shadow-sm text-sm border p-3 focus:outline-none focus:ring-2 ${
                    isRespExceeded
                      ? 'border-rose-300 focus:ring-rose-500 focus:border-rose-500'
                      : 'border-gray-300 focus:ring-purple-500 focus:border-purple-500'
                  }`}
                />
                {isRespExceeded && (
                  <p className="text-xs text-rose-600 mt-1">Превышен лимит в 4000 символов</p>
                )}
              </div>
            ) : (
              <div>
                {member.responsibilities && member.responsibilities.trim() ? (
                  <div
                    data-testid="display-responsibilities"
                    className="text-sm text-gray-900 whitespace-pre-wrap leading-relaxed bg-gray-50/50 p-4 rounded-lg border border-gray-100"
                  >
                    {member.responsibilities}
                  </div>
                ) : (
                  <div
                    data-testid="display-responsibilities-empty"
                    className="text-sm text-gray-500 italic bg-gray-50 rounded-lg p-4 border border-dashed border-gray-200"
                  >
                    Обязанности пока не указаны
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Section 3: Рабочая заметка */}
          <div className="bg-white border border-gray-200 rounded-lg p-6 space-y-4 shadow-sm">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <h2 className="text-base font-semibold text-gray-900">Рабочая заметка</h2>
                <span className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded border border-gray-200">
                  Внутренняя заметка
                </span>
              </div>
              {!isProfileEditing && canEditProfile && (
                <button
                  type="button"
                  onClick={handleStartProfileEdit}
                  className="inline-flex items-center px-3.5 py-1.5 border border-gray-300 shadow-sm text-xs font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-purple-500"
                >
                  Редактировать
                </button>
              )}
            </div>

            {isProfileEditing ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs text-gray-500">
                    Внутренняя административная информация о сотруднике
                  </span>
                  <span
                    className={`text-xs ${
                      isWorkNoteExceeded ? 'text-rose-600 font-semibold' : 'text-gray-500'
                    }`}
                    data-testid="work-note-char-counter"
                  >
                    {workNoteCharCount} / 4000
                  </span>
                </div>
                <textarea
                  data-testid="textarea-work-note"
                  rows={4}
                  value={draftWorkNote}
                  onChange={(e) => setDraftWorkNote(e.target.value)}
                  placeholder="Рабочая заметка..."
                  className={`w-full rounded-md shadow-sm text-sm border p-3 focus:outline-none focus:ring-2 ${
                    isWorkNoteExceeded
                      ? 'border-rose-300 focus:ring-rose-500 focus:border-rose-500'
                      : 'border-gray-300 focus:ring-purple-500 focus:border-purple-500'
                  }`}
                />
                {isWorkNoteExceeded && (
                  <p className="text-xs text-rose-600 mt-1">Превышен лимит в 4000 символов</p>
                )}
              </div>
            ) : (
              <div>
                {member.workNote && member.workNote.trim() ? (
                  <div
                    data-testid="display-work-note"
                    className="text-sm text-gray-900 whitespace-pre-wrap leading-relaxed bg-gray-50/50 p-4 rounded-lg border border-gray-100"
                  >
                    {member.workNote}
                  </div>
                ) : (
                  <div
                    data-testid="display-work-note-empty"
                    className="text-sm text-gray-500 italic bg-gray-50 rounded-lg p-4 border border-dashed border-gray-200"
                  >
                    Рабочая заметка пока не добавлена
                  </div>
                )}
              </div>
            )}

            {/* In Edit Mode: Save and Cancel buttons at the bottom of the editing section */}
            {isProfileEditing && (
              <div className="flex items-center justify-end gap-3 pt-4 border-t border-gray-100">
                <button
                  type="button"
                  onClick={handleCancelProfileEdit}
                  disabled={isProfileSaving}
                  className="px-4 py-2 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-purple-500 disabled:opacity-50"
                >
                  Отмена
                </button>
                <button
                  type="button"
                  onClick={handleSaveProfile}
                  disabled={isProfileSaving || isProfileLimitExceeded}
                  className="px-4 py-2 text-xs font-medium text-white bg-purple-600 border border-transparent rounded-md hover:bg-purple-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-purple-500 disabled:opacity-50 inline-flex items-center gap-2"
                >
                  {isProfileSaving && (
                    <div className="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  )}
                  Сохранить
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    )}

      {/* TAB 2: ДОСТУП */}
      {activeTab === 'access' && (
        <div className="space-y-6" data-testid="tab-access-content">
          {/* Save Success Banner */}
          {saveSuccessMessage && (
            <div
              data-testid="save-success-banner"
              className="bg-emerald-50 border border-emerald-200 text-emerald-800 px-4 py-3 rounded-lg flex items-center justify-between text-sm shadow-sm"
            >
              <div className="flex items-center gap-2">
                <Check className="w-4 h-4 text-emerald-600 shrink-0" />
                <span>{saveSuccessMessage}</span>
              </div>
              <button
                type="button"
                onClick={() => setSaveSuccessMessage(null)}
                className="text-emerald-700 hover:text-emerald-900 p-1"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          )}

          {/* Save Error Banner (shown on page when modal closed) */}
          {saveError && !isChangePreviewOpen && (
            <div
              data-testid="save-error-banner"
              className="bg-rose-50 border border-rose-200 text-rose-800 px-4 py-3 rounded-lg flex items-center justify-between text-sm shadow-sm"
            >
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-rose-600 shrink-0" />
                <span>{saveError}</span>
              </div>
              <button
                type="button"
                onClick={() => setSaveError(null)}
                className="text-rose-700 hover:text-rose-900 p-1"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          )}

          {/* Inactive Account Access Note */}
          {(memberStatus === 'blocked' || memberStatus === 'archived') && (
            <div
              data-testid="inactive-account-access-note"
              className="bg-amber-50/70 border border-amber-200 text-amber-800 px-4 py-2.5 rounded-lg text-xs flex items-center gap-2"
            >
              <AlertCircle className="w-4 h-4 text-amber-600 shrink-0" />
              <span>Настройки сохранятся, но доступ не будет действовать до активации учётной записи.</span>
            </div>
          )}

          {isPermissionManagementForbidden ? (
            <div
              data-testid="access-forbidden-panel"
              className="bg-gray-50 border border-gray-200 rounded-lg p-6 flex items-start gap-3 text-sm shadow-sm"
            >
              <ShieldAlert className="w-5 h-5 text-gray-400 shrink-0 mt-0.5" />
              <div>
                <div className="font-semibold text-gray-900">
                  У вас нет права управлять доступом сотрудников.
                </div>
                <div className="text-xs text-gray-500 mt-1">
                  Для изменения индивидуальных прав требуется право «Управление правами сотрудников».
                </div>
              </div>
            </div>
          ) : (
            <>
              {/* Template Strip */}
          <div className="bg-white border border-gray-200 rounded-lg p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-sm">
            <div className="flex items-center gap-3">
              <Shield className="w-5 h-5 text-gray-600 shrink-0" />
              <div>
                <div className="text-xs text-gray-500">Назначенный шаблон доступа</div>
                <div className="text-sm font-semibold text-gray-900 flex items-center gap-2">
                  <span>{assignedRoleName}</span>
                  <span
                    className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                      templateDiff.matches
                        ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                        : 'bg-indigo-50 text-indigo-700 border border-indigo-200'
                    }`}
                  >
                    {templateDiff.matches ? 'По шаблону' : 'Индивидуально настроено'}
                  </span>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-3">
              {!templateDiff.matches && (
                <button
                  type="button"
                  onClick={() => setShowTemplateDiff((prev) => !prev)}
                  className="text-xs text-purple-700 hover:text-purple-900 font-medium underline"
                >
                  {showTemplateDiff ? 'Скрыть отличия' : 'Показать отличия от шаблона'}
                </button>
              )}
              <button
                type="button"
                onClick={() => setIsMenuPreviewOpen(true)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-gray-300 text-xs font-medium text-gray-700 bg-white hover:bg-gray-50 transition-colors"
              >
                <Eye className="w-3.5 h-3.5 text-gray-500" />
                Какие разделы увидит сотрудник
              </button>
            </div>
          </div>

          {/* Collapsible Template Diff Viewer */}
          {showTemplateDiff && !templateDiff.matches && (
            <div
              data-testid="template-diff-panel"
              className="bg-indigo-50/50 border border-indigo-200 rounded-lg p-4 text-xs space-y-3"
            >
              <div className="font-semibold text-indigo-900">
                Отличия от шаблона «{assignedRoleName}»:
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <div className="font-medium text-emerald-800 mb-1">
                    Дополнительно выдано ({templateDiff.added.length}):
                  </div>
                  {templateDiff.added.length === 0 ? (
                    <div className="text-gray-500 italic">Нет добавлений</div>
                  ) : (
                    <ul className="space-y-1">
                      {templateDiff.added.map((cap) => (
                        <li key={cap} className="font-mono text-emerald-700">
                          + {cap}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
                <div>
                  <div className="font-medium text-rose-800 mb-1">
                    Отозвано из шаблона ({templateDiff.removed.length}):
                  </div>
                  {templateDiff.removed.length === 0 ? (
                    <div className="text-gray-500 italic">Нет ограничений</div>
                  ) : (
                    <ul className="space-y-1">
                      {templateDiff.removed.map((cap) => (
                        <li key={cap} className="font-mono text-rose-700">
                          − {cap}
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Desktop Split Access Workspace */}
          <div className="flex flex-col lg:flex-row gap-6 items-start" data-testid="split-access-editor">
            {/* LEFT RAIL: Real Admin Sections */}
            <div className="w-full lg:w-80 shrink-0 bg-white border border-gray-200 rounded-lg shadow-sm flex flex-col max-h-[780px]">
              <div className="p-3 border-b border-gray-200">
                <div className="relative">
                  <Search className="w-4 h-4 text-gray-400 absolute left-2.5 top-2.5" />
                  <input
                    type="text"
                    value={sectionSearch}
                    onChange={(e) => setSectionSearch(e.target.value)}
                    placeholder="Поиск по разделам..."
                    className="w-full pl-8 pr-3 py-1.5 text-xs border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-purple-500 focus:border-purple-500"
                  />
                </div>
              </div>

              <div className="overflow-y-auto p-2 space-y-4 flex-1">
                {filteredSectionsByGroup.map((group) => (
                  <div key={group.key} className="space-y-1">
                    <div className="text-[10px] font-semibold text-gray-400 uppercase tracking-wider px-2 pt-1">
                      {group.title}
                    </div>
                    {group.sections.map((section) => {
                      const mode = getSectionMode(section, draftPermissions);
                      const isSelected = selectedSectionKey === section.key;
                      return (
                        <button
                          key={section.key}
                          type="button"
                          data-testid={`section-item-${section.key}`}
                          onClick={() => setSelectedSectionKey(section.key)}
                          className={`w-full text-left px-3 py-2 rounded-lg text-xs flex items-center justify-between transition-colors ${
                            isSelected
                              ? 'bg-purple-50 text-purple-900 border border-purple-200 font-medium'
                              : 'hover:bg-gray-50 text-gray-700 border border-transparent'
                          }`}
                        >
                          <span className="truncate pr-2">{section.title}</span>
                          <span
                            className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium shrink-0 ${
                              MODE_BADGE_STYLE[mode]
                            }`}
                          >
                            {SECTION_MODE_LABELS[mode]}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                ))}
              </div>
            </div>

            {/* RIGHT PANEL: Settings for selected section */}
            <div className="flex-1 w-full bg-white border border-gray-200 rounded-lg p-6 shadow-sm space-y-6">
              {/* Section Header */}
              <div className="border-b border-gray-100 pb-4">
                <div className="flex items-center justify-between gap-4">
                  <h2 className="text-lg font-bold text-gray-900">{selectedSection.title}</h2>
                  {selectedSection.route && (
                    <span className="font-mono text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded border border-gray-200">
                      {selectedSection.route}
                    </span>
                  )}
                </div>
                {selectedSection.description && (
                  <p className="text-xs text-gray-500 mt-1">{selectedSection.description}</p>
                )}
              </div>

              {/* Access Mode Radios */}
              <div className="space-y-3">
                <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider">
                  Режим доступа
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                  {/* CLOSED */}
                  <label
                    className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                      currentSectionMode === 'CLOSED'
                        ? 'border-purple-600 bg-purple-50/40 text-purple-950'
                        : 'border-gray-200 hover:border-gray-300 text-gray-700'
                    }`}
                  >
                    <input
                      type="radio"
                      name={`section-mode-${selectedSection.key}`}
                      value="CLOSED"
                      checked={currentSectionMode === 'CLOSED'}
                      onChange={() => handleModeSelect('CLOSED')}
                      className="mt-0.5 text-purple-600 focus:ring-purple-500"
                    />
                    <div>
                      <div className="text-xs font-semibold">Закрыт</div>
                      <div className="text-[11px] text-gray-500">Запрет доступа к разделу</div>
                    </div>
                  </label>

                  {/* VIEW */}
                  {selectedSection.modes.view && (
                    <label
                      className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                        currentSectionMode === 'VIEW'
                          ? 'border-purple-600 bg-purple-50/40 text-purple-950'
                          : 'border-gray-200 hover:border-gray-300 text-gray-700'
                      }`}
                    >
                      <input
                        type="radio"
                        name={`section-mode-${selectedSection.key}`}
                        value="VIEW"
                        checked={currentSectionMode === 'VIEW'}
                        onChange={() => handleModeSelect('VIEW')}
                        className="mt-0.5 text-purple-600 focus:ring-purple-500"
                      />
                      <div>
                        <div className="text-xs font-semibold">Просмотр</div>
                        <div className="text-[11px] text-gray-500">Только чтение данных</div>
                      </div>
                    </label>
                  )}

                  {/* WORK */}
                  {selectedSection.modes.work && (
                    <label
                      className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                        currentSectionMode === 'WORK'
                          ? 'border-purple-600 bg-purple-50/40 text-purple-950'
                          : 'border-gray-200 hover:border-gray-300 text-gray-700'
                      }`}
                    >
                      <input
                        type="radio"
                        name={`section-mode-${selectedSection.key}`}
                        value="WORK"
                        checked={currentSectionMode === 'WORK'}
                        onChange={() => handleModeSelect('WORK')}
                        className="mt-0.5 text-purple-600 focus:ring-purple-500"
                      />
                      <div>
                        <div className="text-xs font-semibold">Работа</div>
                        <div className="text-[11px] text-gray-500">Стандартные рабочие действия</div>
                      </div>
                    </label>
                  )}

                  {/* CUSTOM (indicator) */}
                  {currentSectionMode === 'CUSTOM' && (
                    <div className="flex items-start gap-3 p-3 rounded-lg border border-indigo-300 bg-indigo-50/40 text-indigo-950 md:col-span-3">
                      <div className="w-2.5 h-2.5 rounded-full bg-indigo-600 mt-1 shrink-0" />
                      <div>
                        <div className="text-xs font-semibold">Настроено вручную</div>
                        <div className="text-[11px] text-gray-600">
                          Выбран индивидуальный набор действий или включены дополнительные операции.
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* Standard Actions */}
              <div className="space-y-3 pt-2">
                <div className="text-xs font-semibold text-gray-700 uppercase tracking-wider">
                  Доступные действия
                </div>
                <div className="space-y-2">
                  {standardActions.map((action) => {
                    const isChecked = draftPermissions.includes(action.capability);
                    return (
                      <label
                        key={action.capability}
                        className="flex items-start gap-3 p-2.5 rounded-lg hover:bg-gray-50 cursor-pointer border border-gray-100 transition-colors"
                      >
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => handleActionToggle(action.capability)}
                          className="mt-0.5 rounded text-purple-600 focus:ring-purple-500 border-gray-300"
                        />
                        <div className="flex-1">
                          <div className="text-xs font-medium text-gray-900 flex items-center gap-2">
                            <span>{action.title}</span>
                            {action.criticality === 'sensitive' && (
                              <span className="text-[10px] bg-amber-50 text-amber-700 px-1.5 py-0.2 rounded border border-amber-200">
                                чувствительное
                              </span>
                            )}
                          </div>
                          {action.description && (
                            <div className="text-[11px] text-gray-500 mt-0.5">
                              {action.description}
                            </div>
                          )}
                        </div>
                      </label>
                    );
                  })}
                </div>
              </div>

              {/* Additional / High-Risk Actions */}
              {additionalActions.length > 0 && (
                <div className="space-y-3 pt-2 border-t border-gray-100">
                  <div className="text-xs font-semibold text-rose-800 uppercase tracking-wider flex items-center gap-1.5">
                    <ShieldAlert className="w-3.5 h-3.5 text-rose-600" />
                    Дополнительные / критические действия
                  </div>
                  <div className="space-y-2">
                    {additionalActions.map((action) => {
                      const isChecked = draftPermissions.includes(action.capability);
                      return (
                        <label
                          key={action.capability}
                          className="flex items-start gap-3 p-2.5 rounded-lg bg-rose-50/30 hover:bg-rose-50/60 cursor-pointer border border-rose-200 transition-colors"
                        >
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => handleActionToggle(action.capability)}
                            className="mt-0.5 rounded text-rose-600 focus:ring-rose-500 border-rose-300"
                          />
                          <div className="flex-1">
                            <div className="text-xs font-medium text-gray-900 flex items-center gap-2">
                              <span>{action.title}</span>
                              <span className="text-[10px] bg-rose-100 text-rose-700 font-semibold px-1.5 py-0.2 rounded border border-rose-300">
                                критическое
                              </span>
                            </div>
                            {action.description && (
                              <div className="text-[11px] text-gray-500 mt-0.5">
                                {action.description}
                              </div>
                            )}
                          </div>
                        </label>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Secondary Collapsed Technical Capability Viewer (C2B1) */}
          <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
            <button
              type="button"
              onClick={() => setIsAdvancedOpen((prev) => !prev)}
              className="w-full px-6 py-4 flex items-center justify-between text-left hover:bg-gray-50 transition-colors border-b border-transparent"
            >
              <div className="flex items-center gap-2">
                {isAdvancedOpen ? (
                  <ChevronDown className="w-4 h-4 text-gray-500" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-gray-500" />
                )}
                <span className="text-sm font-semibold text-gray-900">Расширенные настройки</span>
                <span className="text-xs text-gray-500 ml-2">
                  (Технический аудит 86 атомарных прав платформы)
                </span>
              </div>
              <span className="text-xs text-gray-400">Только чтение</span>
            </button>

            {isAdvancedOpen && (
              <div className="p-6 border-t border-gray-200 space-y-6 bg-gray-50/50" data-testid="advanced-capabilities-panel">
                <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                  <div className="flex items-center gap-2 bg-white border border-gray-200 rounded-lg p-1">
                    <button
                      type="button"
                      onClick={() => setAdvancedViewMode('assigned')}
                      className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                        advancedViewMode === 'assigned'
                          ? 'bg-purple-600 text-white'
                          : 'text-gray-600 hover:text-gray-900'
                      }`}
                    >
                      Назначенные ({draftPermissions.length})
                    </button>
                    <button
                      type="button"
                      onClick={() => setAdvancedViewMode('all')}
                      className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                        advancedViewMode === 'all'
                          ? 'bg-purple-600 text-white'
                          : 'text-gray-600 hover:text-gray-900'
                      }`}
                    >
                      Все права (86)
                    </button>
                  </div>

                  <div className="relative w-full md:w-64">
                    <Search className="w-4 h-4 text-gray-400 absolute left-2.5 top-2.5" />
                    <input
                      type="text"
                      value={advancedSearchQuery}
                      onChange={(e) => setAdvancedSearchQuery(e.target.value)}
                      placeholder="Поиск по ключу или названию..."
                      className="w-full pl-8 pr-3 py-1.5 text-xs bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-purple-500"
                    />
                  </div>
                </div>

                {/* Capability Groups List */}
                <div className="space-y-3">
                  {STAFF_CAPABILITY_GROUPS.map((group) => {
                    const groupCaps = getCapabilitiesByGroup(group.key);
                    const q = advancedSearchQuery.trim().toLowerCase();
                    const filteredCaps = groupCaps.filter((c) => {
                      const matchesSearch =
                        !q ||
                        c.key.toLowerCase().includes(q) ||
                        c.title.toLowerCase().includes(q) ||
                        c.description?.toLowerCase().includes(q);
                      const matchesView =
                        advancedViewMode === 'all' || draftPermissions.includes(c.key);
                      return matchesSearch && matchesView;
                    });

                    if (filteredCaps.length === 0) return null;

                    const isExpanded = expandedGroups.has(group.key);

                    return (
                      <div
                        key={group.key}
                        className="bg-white border border-gray-200 rounded-lg overflow-hidden"
                      >
                        <button
                          type="button"
                          onClick={() => toggleGroupExpanded(group.key)}
                          className="w-full px-4 py-2.5 flex items-center justify-between text-left hover:bg-gray-50 transition-colors"
                        >
                          <div className="flex items-center gap-2">
                            {isExpanded ? (
                              <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
                            ) : (
                              <ChevronRight className="w-3.5 h-3.5 text-gray-400" />
                            )}
                            <span className="text-xs font-semibold text-gray-900">
                              {group.title}
                            </span>
                          </div>
                          <span className="text-[11px] text-gray-400">
                            {filteredCaps.length} прав
                          </span>
                        </button>

                        {isExpanded && (
                          <div className="px-4 pb-3 pt-1 border-t border-gray-100 space-y-2">
                            {filteredCaps.map((cap) => {
                              const isAssigned = draftPermissions.includes(cap.key);
                              return (
                                <div
                                  key={cap.key}
                                  className="flex items-start justify-between gap-4 py-1.5 border-b border-gray-50 last:border-0 text-xs"
                                >
                                  <div>
                                    <div className="flex items-center gap-2">
                                      <span className="font-medium text-gray-900">{cap.title}</span>
                                      <span className="font-mono text-[10px] text-gray-400 bg-gray-50 px-1.5 py-0.5 rounded border border-gray-200">
                                        {cap.key}
                                      </span>
                                      {cap.critical && (
                                        <span className="text-[10px] bg-rose-50 text-rose-700 px-1 rounded border border-rose-200">
                                          критическое
                                        </span>
                                      )}
                                    </div>
                                    {cap.description && (
                                      <div className="text-[11px] text-gray-500 mt-0.5">
                                        {cap.description}
                                      </div>
                                    )}
                                  </div>
                                  <span
                                    className={`text-[10px] font-medium px-1.5 py-0.5 rounded shrink-0 ${
                                      isAssigned
                                        ? 'bg-emerald-50 text-emerald-700'
                                        : 'bg-gray-100 text-gray-500'
                                    }`}
                                  >
                                    {isAssigned ? 'Выдано' : 'Не выдано'}
                                  </span>
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
            </>
          )}
        </div>
      )}

      {/* TAB 3: УЧЁТНАЯ ЗАПИСЬ */}
      {activeTab === 'account' && (
        <div className="space-y-6" data-testid="tab-account-content">
          {/* Top Section: Account Information */}
          <div className="bg-white border border-gray-200 rounded-lg p-6 space-y-4 shadow-sm">
            <div>
              <h2 className="text-base font-semibold text-gray-900">Учётная запись</h2>
              <p className="text-xs text-gray-500 mt-0.5">
                Параметры аутентификации и статус аккаунта сотрудника
              </p>
            </div>

            {/* Explanatory notice when blocked or archived */}
            {memberStatus === 'blocked' && (
              <div
                className="bg-rose-50 border border-rose-200 rounded-md p-3 text-xs text-rose-800 flex items-start gap-2.5"
                data-testid="account-blocked-explanation"
              >
                <Lock className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
                <span>
                  Учётная запись заблокирована. Настроенные права сохранены, но доступ к Admin не действует до разблокировки.
                </span>
              </div>
            )}
            {memberStatus === 'archived' && (
              <div
                className="bg-gray-100 border border-gray-200 rounded-md p-3 text-xs text-gray-800 flex items-start gap-2.5"
                data-testid="account-archived-explanation"
              >
                <Lock className="w-4 h-4 text-gray-600 shrink-0 mt-0.5" />
                <span>
                  Учётная запись находится в архиве. Индивидуальные права сохранены, но доступ к Admin закрыт.
                </span>
              </div>
            )}

            {/* Compact Information Rows */}
            <div className="divide-y divide-gray-100 pt-1 border-t border-gray-100 text-sm">
              <div className="py-3 flex items-center justify-between" data-testid="account-row-status">
                <span className="text-gray-500">Статус</span>
                <span
                  className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    STAFF_STATUS_BADGE[memberStatus] || 'bg-gray-100 text-gray-700'
                  }`}
                >
                  {STAFF_STATUS_LABELS[memberStatus] || memberStatus}
                </span>
              </div>
              <div className="py-3 flex items-center justify-between" data-testid="account-row-created-at">
                <span className="text-gray-500">Дата создания</span>
                <span className="font-medium text-gray-900">{formatDate(member.createdAt)}</span>
              </div>
              <div className="py-3 flex items-center justify-between" data-testid="account-row-must-change-password">
                <span className="text-gray-500">Смена пароля</span>
                <span className="font-medium text-gray-900">
                  {member.mustChangePassword ? 'Требуется' : 'Не требуется'}
                </span>
              </div>
            </div>
          </div>

          {/* Account Actions Section */}
          {(canResetPassword || canBlock) && (
            <div
              className="bg-white border border-gray-200 rounded-lg p-6 space-y-4 shadow-sm"
              data-testid="account-actions-section"
            >
              <div>
                <h3 className="text-sm font-semibold text-gray-900">Управление учётной записью</h3>
                <p className="text-xs text-gray-500 mt-0.5">
                  Действия со статусом доступа и аутентификацией сотрудника
                </p>
              </div>

              {accountActionError && (
                <div
                  className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-md flex items-center gap-2"
                  data-testid="account-action-error"
                >
                  <AlertCircle className="w-4 h-4 shrink-0 text-red-500" />
                  <span>{accountActionError}</span>
                </div>
              )}

              {accountActionSuccess && (
                <div
                  className="p-3 bg-emerald-50 border border-emerald-200 text-emerald-700 text-xs rounded-md flex items-center gap-2"
                  data-testid="account-action-success"
                >
                  <Check className="w-4 h-4 shrink-0 text-emerald-600" />
                  <span>{accountActionSuccess}</span>
                </div>
              )}

              <div className="flex items-center gap-3 flex-wrap pt-1">
                {/* Reset Password Button */}
                {canResetPassword && memberStatus !== 'archived' && (
                  <button
                    type="button"
                    onClick={openResetPasswordModal}
                    disabled={isActionLoading}
                    className="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
                  >
                    Сбросить пароль
                  </button>
                )}

                {/* Active: Block button */}
                {canBlock && memberStatus === 'active' && (
                  <button
                    type="button"
                    onClick={() => {
                      setAccountActionError(null);
                      setIsBlockModalOpen(true);
                    }}
                    disabled={isActionLoading}
                    className="inline-flex items-center px-4 py-2 border border-rose-300 rounded-md shadow-sm text-sm font-medium text-rose-700 bg-white hover:bg-rose-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50"
                  >
                    Заблокировать
                  </button>
                )}

                {/* Blocked: Unblock button */}
                {canBlock && memberStatus === 'blocked' && (
                  <button
                    type="button"
                    onClick={handleUnblock}
                    disabled={isActionLoading}
                    className="inline-flex items-center px-4 py-2 border border-emerald-300 rounded-md shadow-sm text-sm font-medium text-emerald-700 bg-white hover:bg-emerald-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-emerald-500 disabled:opacity-50"
                  >
                    Разблокировать
                  </button>
                )}

                {/* Archived: Restore button */}
                {canBlock && memberStatus === 'archived' && (
                  <button
                    type="button"
                    onClick={handleRestoreFromArchive}
                    disabled={isActionLoading}
                    className="inline-flex items-center px-4 py-2 border border-emerald-300 rounded-md shadow-sm text-sm font-medium text-emerald-700 bg-white hover:bg-emerald-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-emerald-500 disabled:opacity-50"
                  >
                    Восстановить из архива
                  </button>
                )}
              </div>
            </div>
          )}

          {/* Danger Zone: Archive */}
          {canBlock && memberStatus !== 'archived' && (
            <div
              className="bg-white border border-rose-200 rounded-lg p-6 space-y-4 shadow-sm"
              data-testid="danger-zone-section"
            >
              <div>
                <h3 className="text-sm font-semibold text-rose-900">Опасные действия</h3>
                <p className="text-xs text-gray-500 mt-1">
                  Сотрудник потеряет доступ к Admin. Индивидуальные права сохраняются.
                </p>
              </div>

              <div className="pt-1">
                <button
                  type="button"
                  onClick={() => {
                    setAccountActionError(null);
                    setIsArchiveModalOpen(true);
                  }}
                  disabled={isActionLoading}
                  className="inline-flex items-center px-4 py-2 border border-rose-300 rounded-md shadow-sm text-sm font-medium text-rose-700 bg-rose-50 hover:bg-rose-100 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-rose-500 disabled:opacity-50"
                >
                  Архивировать сотрудника
                </button>
              </div>
            </div>
          )}

          {/* Collapsed System Information */}
          <div className="bg-white border border-gray-200 rounded-lg shadow-sm overflow-hidden">
            <button
              type="button"
              onClick={() => setIsSystemInfoOpen((prev) => !prev)}
              className="w-full px-6 py-4 flex items-center justify-between text-left hover:bg-gray-50 transition-colors"
            >
              <div className="flex items-center gap-2">
                {isSystemInfoOpen ? (
                  <ChevronDown className="w-4 h-4 text-gray-500" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-gray-500" />
                )}
                <span className="text-sm font-semibold text-gray-900">Системная информация</span>
              </div>
              <span className="text-xs text-gray-400">Технические идентификаторы</span>
            </button>

            {isSystemInfoOpen && (
              <div className="px-6 pb-6 pt-2 border-t border-gray-100 space-y-4" data-testid="system-info-panel">
                <div>
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">
                    ID пользователя (userId)
                  </span>
                  <span className="font-mono text-xs text-gray-900 mt-1 block select-all">
                    {member.userId}
                  </span>
                </div>
                <div>
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">
                    Код роли (roleCode)
                  </span>
                  <span className="font-mono text-xs text-gray-900 mt-1 block">
                    {member.roleCode || memberRoleId || '—'}
                  </span>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Sticky Bottom Bar for Dirty State */}
      {draftChanges.isDirty && !isPermissionManagementForbidden && (
        <div
          data-testid="dirty-bottom-bar"
          className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200 px-6 py-3 shadow-lg z-30 flex items-center justify-between"
        >
          <div className="flex items-center gap-3">
            <span className="w-2.5 h-2.5 rounded-full bg-amber-500 animate-pulse" />
            <span className="text-xs md:text-sm font-medium text-gray-900">
              Есть несохранённые изменения:
            </span>
            <span className="text-xs font-semibold text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
              +{draftChanges.added.length}
            </span>
            <span className="text-xs font-semibold text-rose-700 bg-rose-50 px-2 py-0.5 rounded border border-rose-200">
              −{draftChanges.removed.length}
            </span>
            {draftChanges.criticalAdded.length > 0 && (
              <span className="text-xs font-semibold text-rose-800 bg-rose-100 px-2 py-0.5 rounded border border-rose-300">
                +{draftChanges.criticalAdded.length} критич.
              </span>
            )}
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={handleCancelDraft}
              className="px-4 py-2 border border-gray-300 rounded-lg text-xs font-medium text-gray-700 bg-white hover:bg-gray-50 transition-colors"
            >
              Отмена
            </button>
            <button
              type="button"
              onClick={() => setIsChangePreviewOpen(true)}
              className="px-4 py-2 bg-purple-600 text-white rounded-lg text-xs font-semibold hover:bg-purple-700 transition-colors shadow-sm"
            >
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* Read-Only Change Preview Modal */}
      {isChangePreviewOpen && (
        <div
          data-testid="change-preview-modal"
          className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4"
        >
          <div className="bg-white rounded-xl max-w-lg w-full p-6 shadow-xl space-y-5">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <div>
                <h3 className="text-base font-bold text-gray-900">Изменения доступа</h3>
                <p className="text-xs text-gray-500 mt-0.5">
                  Предпросмотр изменений прав доступа сотрудника
                </p>
              </div>
              <button
                type="button"
                disabled={isSaving}
                onClick={() => {
                  if (!isSaving) {
                    setSaveError(null);
                    setIsChangePreviewOpen(false);
                  }
                }}
                className="text-gray-400 hover:text-gray-600 p-1 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Save Error Alert in Preview Modal */}
            {saveError && (
              <div
                data-testid="save-error-alert"
                className="bg-rose-50 border border-rose-200 rounded-lg p-3 text-xs text-rose-800 flex items-start gap-2.5"
              >
                <AlertCircle className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
                <div>{saveError}</div>
              </div>
            )}

            {/* Critical Alert if critical capabilities altered */}
            {(draftChanges.criticalAdded.length > 0 ||
              draftChanges.criticalRemoved.length > 0) && (
              <div className="bg-rose-50 border border-rose-200 rounded-lg p-3 text-xs text-rose-900 flex items-start gap-2.5">
                <ShieldAlert className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
                <div>
                  <div className="font-semibold">Внимание: затрагиваются критические права!</div>
                  <div className="text-[11px] text-rose-800 mt-0.5">
                    Изменения включают права с повышенным уровнем риска.
                  </div>
                </div>
              </div>
            )}

            <div className="max-h-72 overflow-y-auto space-y-4 pr-1 text-xs">
              {draftChanges.added.length > 0 && (
                <div>
                  <div className="font-semibold text-emerald-800 mb-1.5 flex items-center gap-1">
                    <Check className="w-3.5 h-3.5 text-emerald-600" />
                    Добавится ({draftChanges.added.length}):
                  </div>
                  <ul className="space-y-1.5">
                    {draftChanges.added.map((cap) => {
                      const def = getCapabilityDefinition(cap);
                      return (
                        <li
                          key={cap}
                          className="p-2 rounded bg-emerald-50/50 border border-emerald-100 flex items-center justify-between"
                        >
                          <div>
                            <div className="font-medium text-gray-900">
                              + {def?.title || cap}
                            </div>
                            <div className="font-mono text-[10px] text-gray-400">{cap}</div>
                          </div>
                          {def?.critical && (
                            <span className="text-[9px] bg-rose-100 text-rose-700 px-1 py-0.5 rounded font-semibold">
                              критическое
                            </span>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                </div>
              )}

              {draftChanges.removed.length > 0 && (
                <div>
                  <div className="font-semibold text-rose-800 mb-1.5 flex items-center gap-1">
                    <X className="w-3.5 h-3.5 text-rose-600" />
                    Будет отключено ({draftChanges.removed.length}):
                  </div>
                  <ul className="space-y-1.5">
                    {draftChanges.removed.map((cap) => {
                      const def = getCapabilityDefinition(cap);
                      return (
                        <li
                          key={cap}
                          className="p-2 rounded bg-rose-50/50 border border-rose-100 flex items-center justify-between"
                        >
                          <div>
                            <div className="font-medium text-gray-900">
                              − {def?.title || cap}
                            </div>
                            <div className="font-mono text-[10px] text-gray-400">{cap}</div>
                          </div>
                          {def?.critical && (
                            <span className="text-[9px] bg-rose-100 text-rose-700 px-1 py-0.5 rounded font-semibold">
                              критическое
                            </span>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                </div>
              )}
            </div>

            <div className="border-t border-gray-100 pt-4 flex items-center justify-between gap-3">
              <button
                type="button"
                disabled={isSaving}
                onClick={() => {
                  setSaveError(null);
                  setIsChangePreviewOpen(false);
                }}
                className="px-3 py-1.5 border border-gray-300 rounded-lg text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Вернуться к настройке
              </button>
              <button
                type="button"
                disabled={isSaving}
                onClick={handleSaveClick}
                className="px-4 py-1.5 bg-purple-600 text-white rounded-lg text-xs font-semibold hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-sm"
              >
                {isSaving ? 'Сохранение...' : 'Сохранить изменения'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Self-Revoke Confirmation Modal */}
      {isSelfRevokeConfirmOpen && (
        <div
          data-testid="self-revoke-confirm-modal"
          className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4"
        >
          <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl space-y-4">
            <div className="flex items-start gap-3">
              <div className="p-2 bg-amber-100 rounded-full text-amber-600 shrink-0">
                <AlertTriangle className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-base font-bold text-gray-900">
                  Отключить себе управление доступом?
                </h3>
                <p className="text-xs text-gray-600 mt-2 leading-relaxed">
                  После сохранения вы больше не сможете изменять индивидуальные права сотрудников. Продолжить?
                </p>
              </div>
            </div>

            <div className="border-t border-gray-100 pt-4 flex items-center justify-end gap-3">
              <button
                type="button"
                disabled={isSaving}
                onClick={() => setIsSelfRevokeConfirmOpen(false)}
                className="px-4 py-2 border border-gray-300 rounded-lg text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                Отмена
              </button>
              <button
                type="button"
                disabled={isSaving}
                onClick={() => executeSave()}
                className="px-4 py-2 bg-rose-600 text-white rounded-lg text-xs font-semibold hover:bg-rose-700 disabled:opacity-50 transition-colors"
              >
                {isSaving ? 'Сохранение...' : 'Отключить и сохранить'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Menu Preview Modal */}
      {isMenuPreviewOpen && (
        <div
          data-testid="menu-preview-modal"
          className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4"
        >
          <div className="bg-white rounded-xl max-w-xl w-full p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <div>
                <h3 className="text-base font-bold text-gray-900">
                  Какие разделы увидит сотрудник
                </h3>
                <p className="text-xs text-gray-500 mt-0.5">
                  Предпросмотр пунктов меню на основе текущего черновика прав
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsMenuPreviewOpen(false)}
                className="text-gray-400 hover:text-gray-600 p-1"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="max-h-80 overflow-y-auto space-y-2 text-xs pr-1">
              {STAFF_SCREEN_ACCESS_RULES.map((rule) => {
                const visible = isScreenVisibleWithPerms(rule.visibility, draftPermissions);
                return (
                  <div
                    key={rule.key}
                    data-testid={`menu-rule-${rule.key}`}
                    className={`p-2.5 rounded-lg border flex items-center justify-between ${
                      visible
                        ? 'bg-emerald-50/40 border-emerald-200 text-gray-900'
                        : 'bg-gray-50 border-gray-200 text-gray-400 opacity-60'
                    }`}
                  >
                    <div className="flex items-center gap-2.5">
                      {visible ? (
                        <Check className="w-4 h-4 text-emerald-600 shrink-0" />
                      ) : (
                        <X className="w-4 h-4 text-gray-400 shrink-0" />
                      )}
                      <div>
                        <div className="font-medium">{rule.title}</div>
                        <div className="font-mono text-[10px] text-gray-400">{rule.route}</div>
                      </div>
                    </div>
                    <span
                      className={`text-[10px] font-semibold px-2 py-0.5 rounded ${
                        visible
                          ? 'bg-emerald-100 text-emerald-800'
                          : 'bg-gray-200 text-gray-600'
                      }`}
                    >
                      {visible ? 'Доступен' : 'Скрыт'}
                    </span>
                  </div>
                );
              })}
            </div>

            <div className="border-t border-gray-100 pt-3 flex justify-end">
              <button
                type="button"
                onClick={() => setIsMenuPreviewOpen(false)}
                className="px-4 py-1.5 bg-gray-900 text-white rounded-lg text-xs font-medium hover:bg-gray-800"
              >
                Закрыть
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Block Confirmation Modal */}
      {isBlockModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
          data-testid="block-confirm-modal"
        >
          <div className="bg-white rounded-lg p-6 w-full max-w-md shadow-xl space-y-4">
            <div className="flex items-center gap-3 text-rose-600">
              <ShieldAlert className="h-6 w-6 shrink-0" />
              <h2 className="text-lg font-bold text-gray-900">Заблокировать сотрудника?</h2>
            </div>
            <p className="text-sm text-gray-600">
              «{memberName || member?.email}» больше не сможет войти в Admin до разблокировки.
            </p>
            {accountActionError && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-md" data-testid="modal-error">
                {accountActionError}
              </div>
            )}
            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => {
                  setIsBlockModalOpen(false);
                  setAccountActionError(null);
                }}
                disabled={isActionLoading}
                className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleBlockConfirm}
                disabled={isActionLoading}
                className="px-4 py-2 bg-rose-600 text-white rounded-md text-sm font-medium hover:bg-rose-700 disabled:opacity-50"
              >
                {isActionLoading ? 'Блокировка...' : 'Заблокировать'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Archive Confirmation Modal */}
      {isArchiveModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
          data-testid="archive-confirm-modal"
        >
          <div className="bg-white rounded-lg p-6 w-full max-w-md shadow-xl space-y-4">
            <div className="flex items-center gap-3 text-rose-600">
              <ShieldAlert className="h-6 w-6 shrink-0" />
              <h2 className="text-lg font-bold text-gray-900">Архивировать сотрудника?</h2>
            </div>
            <p className="text-sm text-gray-600">
              Аккаунт будет переведён в архив и потеряет доступ к Admin.
            </p>
            {accountActionError && (
              <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-md" data-testid="modal-error">
                {accountActionError}
              </div>
            )}
            <div className="flex justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={() => {
                  setIsArchiveModalOpen(false);
                  setAccountActionError(null);
                }}
                disabled={isActionLoading}
                className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleArchiveConfirm}
                disabled={isActionLoading}
                className="px-4 py-2 bg-rose-600 text-white rounded-md text-sm font-medium hover:bg-rose-700 disabled:opacity-50"
              >
                {isActionLoading ? 'Архивация...' : 'Архивировать'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Reset Password Modal */}
      {isResetPasswordOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
          data-testid="reset-password-modal"
        >
          <div className="bg-white rounded-lg p-6 w-full max-w-sm shadow-xl">
            <h2 className="text-lg font-bold text-gray-900 mb-1">Сбросить пароль</h2>
            <p className="text-sm text-gray-500 mb-4">{member?.email}</p>
            {resetPasswordError && (
              <div className="mb-3 p-3 bg-red-50 text-red-700 text-xs rounded-md" data-testid="reset-password-error">
                {resetPasswordError}
              </div>
            )}
            <form onSubmit={handleResetPasswordSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Временный пароль *</label>
                <div className="mt-1 flex gap-2">
                  <input
                    required
                    type="text"
                    minLength={8}
                    value={temporaryPassword}
                    onChange={(e) => setTemporaryPassword(e.target.value)}
                    placeholder="Минимум 8 символов"
                    className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm focus:ring-indigo-500 focus:border-indigo-500"
                  />
                  <button
                    type="button"
                    onClick={() => setTemporaryPassword(generatePassword())}
                    className="px-3 py-2 border border-gray-300 rounded-md text-sm text-gray-600 hover:bg-gray-50"
                  >
                    Сгенерировать
                  </button>
                </div>
              </div>
              <div className="flex justify-end space-x-3 pt-2">
                <button
                  type="button"
                  onClick={() => {
                    setIsResetPasswordOpen(false);
                    setTemporaryPassword('');
                    setResetPasswordError(null);
                  }}
                  disabled={isActionLoading}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                >
                  Отмена
                </button>
                <button
                  type="submit"
                  disabled={isActionLoading || temporaryPassword.length < 8}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
                >
                  {isActionLoading ? 'Сброс...' : 'Сбросить пароль'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Password Success Modal */}
      {successPassword && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
          data-testid="password-success-modal"
        >
          <div className="bg-white rounded-lg p-6 w-full max-w-md shadow-xl">
            <div className="flex items-center text-emerald-600 mb-4">
              <Check className="h-7 w-7 mr-2" />
              <h2 className="text-lg font-bold text-gray-900">Новый временный пароль</h2>
            </div>
            <p className="text-sm text-gray-600 mb-1">
              Пользователь: <span className="font-medium">{member?.email}</span>
            </p>
            <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2 mb-3">
              Передайте пароль сотруднику безопасным способом. При первом входе он будет обязан сменить пароль.
            </p>
            <p className="text-xs font-medium text-gray-700 mb-2">
              Временный пароль (показывается только один раз):
            </p>
            <div className="bg-gray-100 p-3.5 rounded text-center mb-4 border border-gray-200 flex items-center justify-between">
              <code className="text-base font-mono font-bold text-gray-900 select-all flex-1 text-left">
                {successPassword}
              </code>
              <button
                type="button"
                onClick={copyPasswordToClipboard}
                className="ml-3 text-gray-500 hover:text-gray-700 p-1.5 rounded hover:bg-gray-200 transition-colors"
                title="Скопировать"
              >
                {isPasswordCopied ? (
                  <span className="text-xs text-emerald-600 font-medium">Скопировано</span>
                ) : (
                  <span className="text-xs text-indigo-600 font-medium">Копировать</span>
                )}
              </button>
            </div>
            <button
              type="button"
              onClick={() => setSuccessPassword(null)}
              className="w-full px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700"
            >
              Понятно
            </button>
          </div>
        </div>
      )}

    </div>
  );
}
