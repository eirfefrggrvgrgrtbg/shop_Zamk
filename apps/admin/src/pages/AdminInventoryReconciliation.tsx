import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  ArrowLeft,
  ArrowRight,
  ScanLine,
  CheckCircle2,
  AlertTriangle,
  Check,
  PackageCheck
} from 'lucide-react';
import {
  getInventoryReconciliation,
  scanInventoryReconciliation,
  completeInventoryReconciliation,
  moveInventoryReconciliationToReview,
  cancelInventoryReconciliation,
  getInventoryReconciliationReview,
  getReconciliationResolutionPlan,
  resolveInventoryReconciliationCase,
  getAdminInventoryItem,
  type ReconciliationSession,
  type ReconciliationReview,
  type ReconciliationResolutionPlan,
  type ReconciliationResolutionCase,
  type ReconciliationResolutionAction,
  type ReplacementCandidate,
  type AdminInventoryView
} from '../api/adminInventory';
import { ApiError } from '@zamk/api-client/src/errors';
import {
  humanizeOrderStatus,
  humanizeShipmentStatus,
  humanizeReturnStatus,
  humanizeSupplyStatus
} from '../utils/statusMapper';

const formatHumanUnitStatus = (status?: string): string => {
  switch (status) {
    case 'shipped':
      return 'отгруженной';
    case 'expected':
      return 'ожидаемой';
    case 'damaged':
      return 'браком';
    case 'written_off':
      return 'списанной';
    case 'warehouse':
      return 'на складе';
    default:
      return 'в неопределённом статусе';
  }
};

const formatSnapshotStatus = (status?: string): string => {
  switch (status) {
    case 'warehouse':
      return 'На складе';
    case 'shipped':
      return 'Отгружена';
    case 'expected':
      return 'Ожидается';
    case 'damaged':
      return 'Брак';
    case 'written_off':
      return 'Списано';
    default:
      return status ? 'Статус не определён' : '—';
  }
};

const getSeverityBadge = (severity: string) => {
  switch (severity) {
    case 'critical':
      return {
        cardBorder: 'border-rose-200',
        headerBg: 'bg-rose-50 text-rose-900 border-rose-200',
        badgeBg: 'bg-rose-100 text-rose-800',
        label: 'Критично',
      };
    case 'high':
      return {
        cardBorder: 'border-orange-200',
        headerBg: 'bg-orange-50 text-orange-900 border-orange-200',
        badgeBg: 'bg-orange-100 text-orange-800',
        label: 'Высокий риск',
      };
    case 'warning':
      return {
        cardBorder: 'border-amber-200',
        headerBg: 'bg-amber-50 text-amber-900 border-amber-200',
        badgeBg: 'bg-amber-100 text-amber-800',
        label: 'Внимание',
      };
    case 'info':
    default:
      return {
        cardBorder: 'border-blue-200',
        headerBg: 'bg-blue-50 text-blue-900 border-blue-200',
        badgeBg: 'bg-blue-100 text-blue-800',
        label: 'Информация',
      };
  }
};

export function formatResolvedDiscrepanciesBanner(count: number): string {
  if (count <= 0) {
    return 'Инвентаризация завершена. Складской учёт не изменён.';
  }
  const rem10 = count % 10;
  const rem100 = count % 100;
  let word = 'расхождений';
  if (rem100 < 11 || rem100 > 19) {
    if (rem10 === 1) {
      word = 'расхождение';
    } else if (rem10 >= 2 && rem10 <= 4) {
      word = 'расхождения';
    }
  }
  return `Инвентаризация завершена. Исправлено ${count} ${word}.`;
}

export type ResolutionBucket = 'resolved' | 'manual_review' | 'action_required';

export function classifyResolutionBucket(c: ReconciliationResolutionCase): ResolutionBucket {
  if (c.resolution) {
    return 'resolved';
  }
  const isManual =
    c.caseType === 'shipped_found' ||
    c.severity === 'critical' ||
    c.allowedActions.some(a => a.safetyLevel === 'BLOCKED');

  if (isManual) {
    return 'manual_review';
  }
  return 'action_required';
}

export function AdminInventoryReconciliation() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement>(null);

  const [session, setSession] = useState<ReconciliationSession | null>(null);
  const [review, setReview] = useState<ReconciliationReview | null>(null);
  const [activeTab, setActiveTab] = useState<'review' | 'resolution'>('review');
  const [resolutionPlan, setResolutionPlan] = useState<ReconciliationResolutionPlan | null>(null);
  const [inventoryItem, setInventoryItem] = useState<AdminInventoryView | null>(null);

  const [loading, setLoading] = useState(true);
  const [scanCode, setScanCode] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);
  const [showCancelModal, setShowCancelModal] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // Resolution modal state
  const [resolveModal, setResolveModal] = useState<{
    caseUnitId: string;
    caseUnitCode: string;
    caseType: string;
    actionId: string;
    actionLabel: string;
    productTitle: string;
    variantName?: string;
    orderNumber?: string;
    replacementCandidates: ReplacementCandidate[];
    selectedReplacementUnitId: string;
    submitting: boolean;
    error: string | null;
  } | null>(null);

  const [lastScan, setLastScan] = useState<{
    code: string;
    classification: string;
    message?: string;
  } | null>(null);

  useEffect(() => {
    if (id) {
      getInventoryReconciliation(id)
        .then(data => {
          setSession(data);
          if (data.variantId) {
            try {
              const res = getAdminInventoryItem(data.variantId);
              if (res && typeof res.then === 'function') {
                res.then(setInventoryItem).catch(() => {});
              }
            } catch (_) {}
          }
          if (data.status === 'review' || data.status === 'completed' || data.status === 'cancelled') {
            loadReview(data.id);
          }
          if (data.status === 'completed') {
            loadResolutionPlan(data.id);
          }
          setLoading(false);
        })
        .catch(() => navigate('/inventory'));
    }
  }, [id, navigate]);

  const loadReview = async (sessionId: string) => {
    try {
      const data = await getInventoryReconciliationReview(sessionId);
      setReview(data);
    } catch (e) {
      console.error(e);
    }
  };

  const loadResolutionPlan = async (sessionId: string) => {
    try {
      const data = await getReconciliationResolutionPlan(sessionId);
      setResolutionPlan(data);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (session?.status === 'in_progress') {
      inputRef.current?.focus();
    }
  }, [session, isProcessing]);

  const handleScan = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!scanCode.trim() || isProcessing || !session) return;

    setIsProcessing(true);
    setActionError(null);
    const code = scanCode.trim();

    try {
      const res = await scanInventoryReconciliation(session.id, code);
      setSession(res.session);

      let msg = '';
      if (res.classification === 'wrong_variant') {
        const parts: string[] = [];
        if (res.unitContext?.productTitle) parts.push(res.unitContext.productTitle);
        if (res.unitContext?.size) parts.push(res.unitContext.size);
        if (res.unitContext?.color) parts.push(res.unitContext.color);
        const detail = parts.length > 0 ? parts.join(' · ') : 'Другой товар';
        msg = `Это другая позиция:\n${detail}`;
      } else if (res.classification === 'unexpected_found') {
        const statusStr = res.unitContext?.status || res.unit?.status;
        msg = `Система считает эту ZMU ${formatHumanUnitStatus(statusStr)}.`;
      } else if (res.classification === 'unknown_code') {
        msg = 'Код не распознан';
      } else if (res.classification === 'duplicate') {
        msg = 'Эта единица уже была отсканирована';
      }

      setLastScan({ code, classification: res.classification, message: msg });
      setScanCode('');
    } catch (err: any) {
      setLastScan({ code, classification: 'error', message: err.message || 'Ошибка сети' });
    } finally {
      setIsProcessing(false);
      setTimeout(() => inputRef.current?.focus(), 0);
    }
  };

  const handleMoveToReview = async () => {
    if (!session) return;
    try {
      setActionError(null);
      await moveInventoryReconciliationToReview(session.id);
      setSession(prev => prev ? { ...prev, status: 'review' } : null);
      await loadReview(session.id);
    } catch (e: any) {
      setActionError(e.message || "Ошибка перехода к проверке");
    }
  };

  const handleComplete = async () => {
    if (!session) return;
    try {
      setActionError(null);
      await completeInventoryReconciliation(session.id);
      setSession(prev => prev ? { ...prev, status: 'completed' } : null);
      await loadReview(session.id);
      await loadResolutionPlan(session.id);
    } catch (e: any) {
      setActionError(e.message || "Ошибка завершения проверки");
    }
  };


  const handleConfirmCancel = async () => {
    if (!session) return;
    try {
      setActionError(null);
      await cancelInventoryReconciliation(session.id);
      setShowCancelModal(false);
      navigate('/inventory');
    } catch (e: any) {
      setActionError(e.message || "Ошибка отмены");
      setShowCancelModal(false);
    }
  };

  const openResolveModal = (c: ReconciliationResolutionCase, action: ReconciliationResolutionAction) => {
    setResolveModal({
      caseUnitId: c.unitId,
      caseUnitCode: c.unitCode,
      caseType: c.caseType,
      actionId: action.id,
      actionLabel: action.label,
      productTitle: c.variant?.productTitle || session?.variantTitle || 'Товар',
      variantName: [c.variant?.size, c.variant?.color].filter(Boolean).join(' · '),
      orderNumber: c.historicalContext?.orderNumber,
      replacementCandidates: c.replacementCandidates || [],
      selectedReplacementUnitId: '', // NO auto-selection: must be an explicit worker choice
      submitting: false,
      error: null,
    });
  };

  const handleSubmitResolve = async () => {
    if (!resolveModal || !session) return;
    if (resolveModal.caseType === 'missing_live_allocated' && resolveModal.actionId === 'confirm_missing') {
      if (!resolveModal.selectedReplacementUnitId.trim()) {
        setResolveModal(m => m ? { ...m, error: 'Пожалуйста, выберите единицу на замену из списка' } : null);
        return;
      }
    }
    setResolveModal(m => m ? { ...m, submitting: true, error: null } : null);
    try {
      const updated = await resolveInventoryReconciliationCase(session.id, {
        unitId: resolveModal.caseUnitId,
        actionId: resolveModal.actionId,
        replacementUnitId: resolveModal.selectedReplacementUnitId ? resolveModal.selectedReplacementUnitId.trim() : undefined,
      });
      setResolutionPlan(updated);
      setResolveModal(null);
      // Refetch inventory and session review without manual reload
      if (session.variantId) {
        try {
          const res = getAdminInventoryItem(session.variantId);
          if (res && typeof res.then === 'function') {
            res.then(setInventoryItem).catch(() => {});
          }
        } catch (_) {}
      }
      loadReview(session.id);
      loadResolutionPlan(session.id);
    } catch (e: any) {
      let msg = e.message || 'Ошибка при выполнении действия';
      if ((e instanceof ApiError && e.status === 409) || e.status === 409 || e.code === 'conflict' || e.message?.includes('409') || e.message?.includes('Конфликт')) {
        msg = 'Состояние изменилось. Обновите данные и повторите действие.';
      } else if (e.status === 403) {
        msg = 'Недостаточно прав для выполнения действия.';
      }
      setResolveModal(m => m ? { ...m, submitting: false, error: msg } : null);
    }
  };

  if (loading || !session) return <div className="p-8">Загрузка...</div>;

  const remainingCount = Math.max(0, session.expectedCount - session.foundExpectedCount);
  const isCompletedOrReview = session.status === 'review' || session.status === 'completed' || session.status === 'cancelled';
  // Completed reconciliation missing count is immutable historical evidence:
  const missingCount = session.status === 'completed'
    ? Math.max(0, session.expectedCount - session.foundExpectedCount)
    : (review?.missing?.length ?? remainingCount);

  // Separate resolution summary dimension (mutually exclusive buckets):
  const resolutionCases = resolutionPlan?.cases || [];
  const totalDiscrepancies = resolutionCases.length;
  const resolvedDiscrepancies = resolutionPlan?.resolvedCasesCount ?? resolutionCases.filter(c => classifyResolutionBucket(c) === 'resolved').length;
  const manualCheckDiscrepancies = resolutionCases.filter(c => classifyResolutionBucket(c) === 'manual_review').length;
  const actionableDiscrepancies = resolutionCases.filter(c => classifyResolutionBucket(c) === 'action_required').length;

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col">
      <header className="bg-white border-b border-slate-200 px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Link to="/inventory" className="p-2 -ml-2 text-slate-400 hover:text-slate-600 rounded-full hover:bg-slate-100">
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div>
            <h1 className="text-xl font-bold text-slate-900">Инвентаризация</h1>
            <div className="text-sm text-slate-500 font-medium mt-0.5">
              {session.variantTitle} {session.variantSize && `· ${session.variantSize}`} {session.variantColor && `· ${session.variantColor}`}
            </div>
            <div className="text-xs text-slate-400 mt-1 font-mono">
              SKU {session.variantSKU}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <div className="px-3 py-1 rounded-full text-sm font-medium border">
            {session.status === 'in_progress' ? (
              <span className="text-indigo-600 border-indigo-200">В процессе</span>
            ) : session.status === 'review' ? (
              <span className="text-amber-600 border-amber-200">Проверка результатов</span>
            ) : session.status === 'completed' ? (
              <span className="text-emerald-600 border-emerald-200">Завершена</span>
            ) : (
              <span className="text-slate-600 border-slate-200">Отменена</span>
            )}
          </div>
          {session.status === 'in_progress' && (
            <button
              type="button"
              onClick={() => setShowCancelModal(true)}
              className="text-red-600 hover:text-red-800 text-sm font-medium px-3 py-1.5 cursor-pointer"
            >
              Отменить
            </button>
          )}
        </div>
      </header>

      {/* Session status banner */}
      {session.status === 'completed' && (
        <div className="bg-emerald-50 border-b border-emerald-200 px-6 py-3 flex items-center justify-between text-xs text-emerald-800">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 text-emerald-600" />
            <span className="font-semibold">
              {formatResolvedDiscrepanciesBanner(resolvedDiscrepancies)}
            </span>
          </div>
          <Link
            to="/inventory"
            className="text-emerald-700 hover:text-emerald-900 font-semibold underline"
          >
            Вернуться к остаткам
          </Link>
        </div>
      )}

      {session.status === 'cancelled' && (
        <div className="bg-slate-100 border-b border-slate-200 px-6 py-3 flex items-center justify-between text-xs text-slate-700">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-slate-500" />
            <span className="font-medium">Инвентаризация отменена. Данные сохранены в истории.</span>
          </div>
          <Link
            to="/inventory"
            className="text-slate-600 hover:text-slate-800 font-semibold underline"
          >
            Вернуться к остаткам
          </Link>
        </div>
      )}

      {actionError && (
        <div className="bg-rose-50 border-b border-rose-200 px-6 py-3 text-xs text-rose-800 font-medium">
          {actionError}
        </div>
      )}

      <main className="flex-1 flex p-6 gap-6 max-w-7xl mx-auto w-full">
        {/* Left Column - Scanner & Review Actions */}
        <div className="flex-1 flex flex-col gap-6">

          {session.status === 'in_progress' ? (
            <div className="bg-white rounded-xl border border-slate-200 p-8 shadow-sm">
              <form onSubmit={handleScan} className="flex flex-col items-center">
                <ScanLine className="w-16 h-16 text-indigo-100 mb-6" />
                <h2 className="text-2xl font-semibold text-slate-800 mb-2">Отсканируйте ZMU</h2>
                <p className="text-slate-500 mb-8 text-center max-w-sm">
                  Сканируйте все физически найденные единицы товара.
                  {session.accountingMode === 'mixed' && (
                    <span className="block mt-2 text-amber-600 font-medium text-xs">
                      Проверяются только физические ZMU. Остаток без ZMU ({session.legacyOnHand}) не входит в эту проверку.
                    </span>
                  )}
                </p>

                <div className="w-full max-w-md relative">
                  <input
                    ref={inputRef}
                    type="text"
                    value={scanCode}
                    onChange={(e) => setScanCode(e.target.value.toUpperCase())}
                    placeholder="Штрихкод ZMU..."
                    className="w-full text-center text-3xl font-mono p-4 pr-14 rounded-xl border-2 border-indigo-200 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/20 outline-none transition-all"
                    disabled={isProcessing}
                    autoFocus
                  />
                  <button
                    type="submit"
                    disabled={!scanCode.trim() || isProcessing}
                    aria-label="Отсканировать"
                    title="Отсканировать"
                    className="absolute right-3 top-1/2 -translate-y-1/2 p-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-700 disabled:bg-slate-200 text-white disabled:text-slate-400 transition-colors cursor-pointer disabled:cursor-not-allowed shadow-xs flex items-center justify-center"
                  >
                    <ArrowRight className="w-5 h-5" />
                  </button>
                </div>
              </form>

              {lastScan && (
                <div className={`mt-8 p-4 rounded-lg border flex items-start gap-3 ${
                  lastScan.classification === 'expected_found' ? 'bg-emerald-50 border-emerald-200 text-emerald-800' :
                  lastScan.classification === 'duplicate' ? 'bg-amber-50 border-amber-200 text-amber-800' :
                  lastScan.classification === 'unexpected_found' ? 'bg-blue-50 border-blue-200 text-blue-800' :
                  'bg-red-50 border-red-200 text-red-800'
                }`}>
                  {lastScan.classification === 'expected_found' ? <CheckCircle2 className="w-5 h-5 mt-0.5 shrink-0" /> :
                   lastScan.classification === 'duplicate' ? <Check className="w-5 h-5 mt-0.5 shrink-0" /> :
                   <AlertTriangle className="w-5 h-5 mt-0.5 shrink-0" />}

                  <div>
                    <div className="font-semibold text-lg mb-1">
                      {lastScan.classification === 'expected_found' ? 'Ожидаемая единица найдена' :
                       lastScan.classification === 'duplicate' ? 'Уже отсканирована' :
                       lastScan.classification === 'unexpected_found' ? 'Неожиданная единица' :
                       lastScan.classification === 'wrong_variant' ? 'Другой вариант товара' :
                       'Проблема со штрихкодом'}
                    </div>
                    <div className="font-mono text-sm opacity-90">{lastScan.code}</div>
                    {lastScan.message && (
                      <div className="text-sm mt-1 opacity-90 font-medium">{lastScan.message}</div>
                    )}
                  </div>
                </div>
              )}

              <div className="mt-12 pt-6 border-t flex justify-end">
                <button
                  type="button"
                  onClick={handleMoveToReview}
                  className="px-6 py-2.5 bg-slate-900 text-white rounded-lg font-medium hover:bg-slate-800 transition-colors cursor-pointer"
                >
                  Перейти к результатам
                </button>
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-slate-200 p-8 shadow-sm flex flex-col items-center">
              <PackageCheck className="w-16 h-16 text-indigo-600 mb-4" />
              <h2 className="text-2xl font-bold text-slate-800 mb-2">
                {session.status === 'completed' ? 'Результаты инвентаризации' :
                 session.status === 'cancelled' ? 'Отменённая инвентаризация' :
                 'Проверка результатов'}
              </h2>

              {review?.missing?.length === 0 && review?.unexpectedFound?.length === 0 && review?.changedDuringCount?.length === 0 ? (
                <div className="text-emerald-600 font-medium mb-8">✓ Расхождений не обнаружено</div>
              ) : resolvedDiscrepancies > 0 ? (
                <div className="text-emerald-800 bg-emerald-50 border border-emerald-200 px-4 py-2 rounded-lg font-medium mb-8 text-center text-sm">
                  Обнаружены расхождения.<br/>Часть расхождений уже исправлена.
                </div>
              ) : (
                <div className="text-amber-700 bg-amber-50 px-4 py-2 rounded-lg font-medium mb-8 text-center text-sm">
                  Обнаружены расхождения.<br/>Они зафиксированы, но складской учёт не изменён.
                </div>
              )}

              {session.status === 'review' && (
                <button
                  type="button"
                  onClick={handleComplete}
                  className="px-8 py-3 bg-indigo-600 text-white rounded-lg font-semibold hover:bg-indigo-700 transition-colors shadow-sm cursor-pointer"
                >
                  Завершить проверку
                </button>
              )}
            </div>
          )}

          {/* Tabs for Completed Session */}
          {session?.status === 'completed' && (
            <div className="flex gap-4 border-b border-slate-200 mb-4" role="tablist">
              <button
                type="button"
                role="tab"
                aria-selected={activeTab === 'review'}
                className={`pb-2 px-1 text-sm font-semibold ${activeTab === 'review' ? 'border-b-2 border-indigo-600 text-indigo-600' : 'text-slate-500 hover:text-slate-700'}`}
                onClick={() => setActiveTab('review')}
              >
                РЕЗУЛЬТАТЫ СВЕРКИ
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeTab === 'resolution'}
                className={`pb-2 px-1 text-sm font-semibold ${activeTab === 'resolution' ? 'border-b-2 border-indigo-600 text-indigo-600' : 'text-slate-500 hover:text-slate-700'}`}
                onClick={() => setActiveTab('resolution')}
              >
                РАЗБОР РАСХОЖДЕНИЙ
              </button>
            </div>
          )}

          {/* Resolution Plan */}
          {activeTab === 'resolution' && resolutionPlan && (
            <div className="flex flex-col gap-4">
              {resolutionPlan.cases.length === 0 ? (
                <div className="bg-emerald-50 rounded-xl p-6 border border-emerald-200 text-center">
                  <h3 className="text-emerald-900 font-bold mb-2">Расхождений не обнаружено</h3>
                  <p className="text-emerald-700 text-sm">Все ожидаемые единицы найдены, лишних не обнаружено.</p>
                </div>
              ) : (
                resolutionPlan.cases.map((c: ReconciliationResolutionCase) => {
                  const sev = getSeverityBadge(c.severity);
                  const variantDetails = [
                    c.variant?.productTitle,
                    c.variant?.size ? `Размер: ${c.variant.size}` : null,
                    c.variant?.color ? `Цвет: ${c.variant.color}` : null,
                    c.variant?.sku ? `SKU: ${c.variant.sku}` : null,
                  ].filter(Boolean).join(' · ');

                  const orderStatusLabel = humanizeOrderStatus(c.historicalContext?.orderStatus);
                  const shipmentStatusLabel = humanizeShipmentStatus(c.historicalContext?.shipmentStatus);
                  const returnStatusLabel = humanizeReturnStatus(c.historicalContext?.returnStatus);
                  const supplyStatusLabel = humanizeSupplyStatus(c.historicalContext?.supplyStatus);

                  const blockedActions = c.allowedActions?.filter(a => a.safetyLevel === 'BLOCKED') || [];
                  const executableActions = c.allowedActions?.filter(a => a.safetyLevel !== 'BLOCKED') || [];

                  return (
                    <div key={c.unitId} className={`bg-white rounded-xl border overflow-hidden ${c.resolution ? 'border-emerald-200' : sev.cardBorder}`}>
                      {/* Resolved header */}
                      {c.resolution ? (
                        <div className="px-4 py-3 border-b border-emerald-200 bg-emerald-50 flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <CheckCircle2 className="w-4 h-4 text-emerald-600 shrink-0" />
                            <span className="font-mono text-sm text-emerald-900">{c.unitCode}</span>
                            <span className="opacity-40 text-emerald-700">·</span>
                            <span className="text-sm text-emerald-900">Исправлено</span>
                          </div>
                          <span className="text-xs px-2.5 py-0.5 rounded-full bg-emerald-100 text-emerald-800 font-medium">Списано</span>
                        </div>
                      ) : (
                        <div className={`px-4 py-3 border-b font-semibold flex items-center justify-between ${sev.headerBg}`}>
                          <div className="flex items-center gap-2">
                            <span className="font-mono text-sm">{c.unitCode}</span>
                            <span className="opacity-40">·</span>
                            <span className="text-sm">{c.title}</span>
                          </div>
                          <span className={`text-xs px-2.5 py-0.5 rounded-full font-medium ${sev.badgeBg}`}>
                            {sev.label}
                          </span>
                        </div>
                      )}

                      <div className="p-4 space-y-3">
                        {variantDetails && (
                          <div className="text-xs text-slate-500 font-medium">
                            {variantDetails}
                          </div>
                        )}

                        {c.resolution ? (
                          <p className="text-emerald-700 text-sm">
                            Единица списана. Складской учёт скорректирован.
                            {c.resolution.replacementUnitCode && ` Замена: ${c.resolution.replacementUnitCode}.`}
                          </p>
                        ) : (
                          <>
                            <p className="text-slate-700 text-sm leading-relaxed">{c.explanation}</p>

                            {/* Blocked Informational Callout */}
                            {blockedActions.length > 0 && (
                              <div className="p-3 rounded-lg bg-amber-50/80 border border-amber-200 text-amber-900 text-xs flex items-start gap-2.5">
                                <AlertTriangle className="w-4 h-4 shrink-0 text-amber-600 mt-0.5" />
                                <div className="space-y-1">
                                  {blockedActions.map(action => (
                                    <div key={action.id}>
                                      <div className="font-semibold text-amber-950">{action.label}</div>
                                      {action.blockedReason && (
                                        <div className="text-amber-800 text-[11px] mt-0.5">{action.blockedReason}</div>
                                      )}
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}

                            {/* Structured context chips */}
                            {(c.historicalContext || c.currentAllocationCtx || c.lineageCtx) && (
                              <div className="flex flex-wrap gap-2 text-xs pt-1">
                                {c.historicalContext?.orderNumber && (
                                  <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 border border-slate-200 font-medium">
                                    Заказ {c.historicalContext.orderNumber}{orderStatusLabel ? ` · ${orderStatusLabel}` : ''}
                                  </span>
                                )}
                                {c.historicalContext?.shipmentStatus && (
                                  <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 border border-slate-200 font-medium">
                                    Отгрузка{shipmentStatusLabel ? ` · ${shipmentStatusLabel}` : ''}
                                  </span>
                                )}
                                {c.historicalContext?.returnStatus && (
                                  <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 border border-slate-200 font-medium">
                                    Возврат{returnStatusLabel ? ` · ${returnStatusLabel}` : ''}
                                  </span>
                                )}
                                {c.historicalContext?.supplyNumber && (
                                  <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 border border-slate-200 font-medium">
                                    Поставка {c.historicalContext.supplyNumber}{supplyStatusLabel ? ` · ${supplyStatusLabel}` : ''}
                                  </span>
                                )}
                              </div>
                            )}

                            {/* Executable Actions */}
                            {executableActions.length > 0 && (
                              <div className="pt-2 border-t border-slate-100 flex flex-wrap items-center gap-2">
                                {executableActions.map((action: ReconciliationResolutionAction) => {
                                  if (action.enabled && action.route) {
                                    return (
                                      <Link
                                        key={action.id}
                                        to={action.route}
                                        className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors shadow-xs ${
                                          action.safetyLevel === 'WORKFLOW_HANDOFF'
                                            ? 'bg-indigo-600 text-white hover:bg-indigo-700'
                                            : 'bg-white border border-slate-300 text-slate-700 hover:bg-slate-50'
                                        }`}
                                      >
                                        {action.label}
                                      </Link>
                                    );
                                  }
                                  if (action.enabled && action.safetyLevel === 'MUTATION_REQUIRES_CONFIRMATION') {
                                    // Active mutation button — opens confirmation modal
                                    return (
                                      <button
                                        key={action.id}
                                        type="button"
                                        onClick={() => openResolveModal(c, action)}
                                        className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-rose-600 text-white hover:bg-rose-700 transition-colors shadow-xs"
                                      >
                                        {action.label}
                                      </button>
                                    );
                                  }
                                  return (
                                    <div key={action.id} className="inline-flex items-center gap-1.5">
                                      <button
                                        type="button"
                                        disabled={true}
                                        title={action.blockedReason || 'Действие недоступно'}
                                        className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-slate-100 text-slate-400 border border-slate-200 cursor-not-allowed"
                                      >
                                        {action.label}
                                      </button>
                                      {action.blockedReason && (
                                        <span className="text-[11px] text-slate-400">
                                          ({action.blockedReason})
                                        </span>
                                      )}
                                    </div>
                                  );
                                })}
                              </div>
                            )}
                          </>
                        )}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}


          {/* Review Details */}
          {activeTab === 'review' && review && (
            <div className="flex flex-col gap-4">
              {review.missing.length > 0 && (
                <div className="bg-white rounded-xl border border-red-200 overflow-hidden">
                  <div className="bg-red-50 px-4 py-3 border-b border-red-200 font-semibold text-red-900 flex justify-between">
                    <span>НЕ НАЙДЕНЫ</span>
                    <span>{review.missing.length}</span>
                  </div>
                  <div className="divide-y">
                    {review.missing.map(m => (
                      <div key={m.unitId} className="px-4 py-2 font-mono text-sm text-slate-700">{m.unitCode}</div>
                    ))}
                  </div>
                </div>
              )}

              {review.unexpectedFound.length > 0 && (
                <div className="bg-white rounded-xl border border-blue-200 overflow-hidden">
                  <div className="bg-blue-50 px-4 py-3 border-b border-blue-200 font-semibold text-blue-900 flex justify-between">
                    <span>НЕОЖИДАННО НАЙДЕНЫ</span>
                    <span>{review.unexpectedFound.length}</span>
                  </div>
                  <div className="divide-y">
                    {review.unexpectedFound.map(m => (
                      <div key={m.unitId} className="px-4 py-2 text-sm flex justify-between">
                        <span className="font-mono text-slate-700">{m.unitCode}</span>
                        <span className="text-slate-500">Текущий статус: {formatSnapshotStatus(m.currentStatus)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {review.changedDuringCount.length > 0 && (
                <div className="bg-white rounded-xl border border-amber-200 overflow-hidden">
                  <div className="bg-amber-50 px-4 py-3 border-b border-amber-200 font-semibold text-amber-900 flex justify-between">
                    <span>ИЗМЕНИЛИСЬ ВО ВРЕМЯ ПРОВЕРКИ</span>
                    <span>{review.changedDuringCount.length}</span>
                  </div>
                  <div className="p-4 text-xs text-amber-800 border-b border-amber-100 bg-amber-50/50">
                    Состояние изменилось после начала проверки
                  </div>
                  <div className="divide-y">
                    {review.changedDuringCount.map(m => (
                      <div key={m.unitId} className="px-4 py-2 text-sm flex justify-between">
                        <span className="font-mono text-slate-700">{m.unitCode}</span>
                        <span className="text-slate-500 text-xs">Было при начале проверки: {formatSnapshotStatus(m.snapshotStatus)} → Сейчас: {formatSnapshotStatus(m.currentStatus)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {review.expectedFound.length > 0 && (
                <div className="bg-white rounded-xl border border-emerald-200 overflow-hidden">
                  <div className="bg-emerald-50 px-4 py-3 border-b border-emerald-200 font-semibold text-emerald-900 flex justify-between">
                    <span>НАЙДЕНЫ</span>
                    <span>{review.expectedFound.length}</span>
                  </div>
                  <div className="divide-y max-h-64 overflow-y-auto">
                    {review.expectedFound.map(m => (
                      <div key={m.unitId} className="px-4 py-2 font-mono text-sm text-slate-700">{m.unitCode}</div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

        </div>

        {/* Right Column - Stats */}
        <div className="w-80 shrink-0">
          <div className="bg-white rounded-xl border border-slate-200 p-6 sticky top-6 shadow-sm">
            <h3 className="text-sm font-bold text-slate-900 mb-4 uppercase tracking-wider">Прогресс</h3>

            <div className="space-y-4">
              <div>
                <div className="text-sm text-slate-500 mb-1">Ожидалось</div>
                <div className="text-3xl font-bold text-slate-900">{session.expectedCount}</div>
              </div>

              <div className="pt-4 border-t border-slate-100">
                <div className="text-sm text-slate-500 mb-1">
                  {isCompletedOrReview ? 'Найдено' : 'Найдено ожидаемых'}
                </div>
                <div className="text-2xl font-bold text-emerald-600">{session.foundExpectedCount}</div>
              </div>

              <div>
                <div className="text-sm text-slate-500 mb-1">
                  {isCompletedOrReview ? 'Не найдено' : 'Осталось найти'}
                </div>
                <div className={`text-2xl font-bold ${
                  isCompletedOrReview
                    ? (missingCount > 0 ? 'text-rose-600' : 'text-slate-300')
                    : (remainingCount > 0 ? 'text-amber-600' : 'text-slate-300')
                }`}>
                  {isCompletedOrReview ? missingCount : remainingCount}
                </div>
              </div>

              <div className="pt-4 border-t border-slate-100">
                <div className="text-sm text-slate-500 mb-1">Неожиданно найдено</div>
                <div className="text-xl font-bold text-blue-600">{session.unexpectedCount}</div>
              </div>

              <div>
                <div className="text-sm text-slate-500 mb-1">Ошибки сканирования</div>
                <div className={`text-xl font-bold ${session.problemsCount > 0 ? 'text-rose-600' : 'text-slate-300'}`}>
                  {session.problemsCount}
                </div>
              </div>
            </div>

            {/* Separate resolution progress dimension */}
            {session.status === 'completed' && resolutionPlan && (
              <div className="pt-6 border-t border-slate-200">
                <h3 className="text-sm font-bold text-slate-900 mb-4 uppercase tracking-wider">Разбор расхождений</h3>
                <div className="space-y-3 text-xs">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500">Всего расхождений</span>
                    <span className="text-sm font-bold text-slate-900">{totalDiscrepancies}</span>
                  </div>
                  <div className="flex items-center justify-between pt-2 border-t border-slate-100">
                    <span className="text-slate-500">Исправлено</span>
                    <span className="text-sm font-bold text-emerald-600">{resolvedDiscrepancies}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500">Требует ручной проверки</span>
                    <span className={`text-sm font-bold ${manualCheckDiscrepancies > 0 ? 'text-amber-600' : 'text-slate-400'}`}>
                      {manualCheckDiscrepancies}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500">Требует действий</span>
                    <span className={`text-sm font-bold ${actionableDiscrepancies > 0 ? 'text-rose-600' : 'text-slate-400'}`}>
                      {actionableDiscrepancies}
                    </span>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </main>

      {/* Resolve Confirmation Modal */}
      {resolveModal && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl border border-slate-200">
            <div className="flex items-center gap-3 mb-4">
              <div className={`p-2 rounded-lg ${resolveModal.actionId === 'close_stale_allocation' ? 'bg-amber-100 text-amber-700' : 'bg-rose-100 text-rose-700'}`}>
                {resolveModal.actionId === 'close_stale_allocation' ? (
                  <AlertTriangle className="w-5 h-5" />
                ) : (
                  <PackageCheck className="w-5 h-5" />
                )}
              </div>
              <div>
                <h3 className="text-base font-bold text-slate-900">{resolveModal.actionLabel}</h3>
                <div className="text-xs text-slate-500 font-medium">
                  {resolveModal.productTitle} {resolveModal.variantName && `· ${resolveModal.variantName}`}
                </div>
              </div>
            </div>

            {/* STALE ALLOCATION MODAL */}
            {resolveModal.actionId === 'close_stale_allocation' && (
              <div className="text-sm text-slate-600 mb-4 space-y-3">
                <div className="p-3 bg-amber-50 border border-amber-200 rounded-lg text-xs text-amber-900 leading-relaxed">
                  Старое зависшее назначение по завершённому или отменённому заказу будет освобождено.
                  Единица останется на складе и перейдёт в свободный остаток.
                </div>
                <div className="flex items-center justify-between text-xs py-1 border-b border-slate-100">
                  <span className="text-slate-500">ZMU единицы:</span>
                  <span className="font-mono font-semibold text-slate-800">{resolveModal.caseUnitCode}</span>
                </div>
              </div>
            )}

            {/* MISSING FREE MODAL (WITH -1 STOCK IMPACT) */}
            {resolveModal.actionId === 'confirm_missing' && resolveModal.caseType !== 'missing_live_allocated' && (() => {
              const currentTotal = inventoryItem?.totalStock ?? (session?.expectedCount ?? 0);
              const nextTotal = Math.max(0, currentTotal - 1);
              const currentPhysical = inventoryItem?.physical?.warehouse ?? (session?.expectedCount ?? 0);
              const nextPhysical = Math.max(0, currentPhysical - 1);
              const legacyOnHand = inventoryItem?.legacy?.onHand ?? Math.max(0, currentTotal - currentPhysical);

              return (
                <div className="text-sm text-slate-600 mb-4 space-y-3">
                  <div className="p-3 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-900 leading-relaxed">
                    Это действие подтверждает отсутствие единицы. Единица будет списана со склада со статусом «Списано». Отменить действие невозможно.
                  </div>

                  <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-2.5 text-xs">
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500">ZMU недостачи:</span>
                      <span className="font-mono font-semibold text-slate-800">{resolveModal.caseUnitCode}</span>
                    </div>

                    <div className="pt-2 border-t border-slate-200 space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-slate-700 font-semibold uppercase tracking-wider text-[11px]">ОБЩИЙ ОСТАТОК</span>
                        <span className="font-bold text-slate-800">
                          {currentTotal} → <span className="text-rose-700">{nextTotal}</span>
                        </span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-slate-600 font-medium uppercase tracking-wider text-[11px]">ФИЗИЧЕСКИЕ ZMU НА СКЛАДЕ</span>
                        <span className="font-medium text-slate-700">
                          {currentPhysical} → <span className="text-rose-700">{nextPhysical}</span>
                        </span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-slate-600 font-medium uppercase tracking-wider text-[11px]">БЕЗ ZMU</span>
                        <span className="font-medium text-slate-700">
                          {legacyOnHand} → {legacyOnHand}
                        </span>
                      </div>

                      <div className="flex items-center justify-between pt-1.5 border-t border-slate-200 text-slate-500">
                        <span>Изменение коммерческого остатка:</span>
                        <span className="text-[11px] font-bold px-1.5 py-0.5 rounded bg-rose-100 text-rose-800">-1</span>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })()}

            {/* MISSING LIVE ALLOCATED (WITH CANDIDATE SELECTOR & STOCK IMPACT) */}
            {resolveModal.actionId === 'confirm_missing' && resolveModal.caseType === 'missing_live_allocated' && (() => {
              const currentTotal = inventoryItem?.totalStock ?? (session?.expectedCount ?? 0);
              const nextTotal = Math.max(0, currentTotal - 1);
              const currentPhysical = inventoryItem?.physical?.warehouse ?? (session?.expectedCount ?? 0);
              const nextPhysical = Math.max(0, currentPhysical - 1);
              const legacyOnHand = inventoryItem?.legacy?.onHand ?? Math.max(0, currentTotal - currentPhysical);
              const currentReserved = inventoryItem?.reservedStock ?? 0;
              const selectedCandidate = resolveModal.replacementCandidates.find(c => c.unitId === resolveModal.selectedReplacementUnitId);
              const orderTarget = resolveModal.orderNumber ? `заказу ${resolveModal.orderNumber}` : 'активному заказу';

              return (
                <div className="text-sm text-slate-600 mb-4 space-y-3">
                  <div className="p-3 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-900 leading-relaxed space-y-1">
                    <p>Отсутствующая единица будет списана.</p>
                    <p>Выбранная свободная ZMU будет назначена {orderTarget}.</p>
                    <p>Количество зарезервированного товара для заказа не изменится.</p>
                  </div>

                  <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-2.5 text-xs">
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500">Отсутствующая ZMU:</span>
                      <span className="font-mono font-semibold text-slate-800">{resolveModal.caseUnitCode}</span>
                    </div>

                    <div className="pt-2 border-t border-slate-200 space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-slate-700 font-semibold uppercase tracking-wider text-[11px]">ОБЩИЙ ОСТАТОК</span>
                        <span className="font-bold text-slate-800">
                          {currentTotal} → <span className="text-rose-700">{nextTotal}</span>
                        </span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-slate-600 font-medium uppercase tracking-wider text-[11px]">ФИЗИЧЕСКИЕ ZMU НА СКЛАДЕ</span>
                        <span className="font-medium text-slate-700">
                          {currentPhysical} → <span className="text-rose-700">{nextPhysical}</span>
                        </span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-slate-600 font-medium uppercase tracking-wider text-[11px]">БЕЗ ZMU</span>
                        <span className="font-medium text-slate-700">
                          {legacyOnHand} → {legacyOnHand}
                        </span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-slate-600 font-medium uppercase tracking-wider text-[11px]">В РЕЗЕРВЕ</span>
                        <span className="font-medium text-slate-700">
                          {currentReserved} → {currentReserved}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                      Выберите единицу на замену (обязательно)
                    </label>
                    {resolveModal.replacementCandidates.length > 0 ? (
                      <select
                        value={resolveModal.selectedReplacementUnitId}
                        onChange={e => setResolveModal(m => m ? { ...m, selectedReplacementUnitId: e.target.value, error: null } : null)}
                        className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg font-mono focus:outline-none focus:ring-2 focus:ring-rose-500 bg-white"
                        autoFocus
                      >
                        <option value="">-- Выберите единицу из списка ({resolveModal.replacementCandidates.length} на складе) --</option>
                        {resolveModal.replacementCandidates.map(c => (
                          <option key={c.unitId} value={c.unitId}>
                            {c.unitCode} — Свободна
                          </option>
                        ))}
                      </select>
                    ) : (
                      <div className="p-2.5 bg-amber-50 border border-amber-200 rounded-lg text-xs text-amber-800">
                        На складе нет свободных единиц данной номенклатуры для автоматической замены.
                      </div>
                    )}
                  </div>

                  {selectedCandidate && (
                    <div className="p-3 bg-indigo-50 border border-indigo-200 rounded-lg text-center space-y-1">
                      <div className="text-[10px] font-bold uppercase tracking-wider text-indigo-700">ЗАМЕНА</div>
                      <div className="flex flex-col items-center justify-center font-mono font-semibold text-xs text-slate-800">
                        <span className="text-rose-700">{resolveModal.caseUnitCode}</span>
                        <span className="text-slate-400 text-sm">↓</span>
                        <span className="text-emerald-700">{selectedCandidate.unitCode}</span>
                      </div>
                    </div>
                  )}
                </div>
              );
            })()}

            {resolveModal.error && (
              <div className="p-2.5 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-700 mb-3 font-medium">
                {resolveModal.error}
              </div>
            )}

            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                disabled={resolveModal.submitting}
                onClick={() => setResolveModal(null)}
                className="px-4 py-2 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg cursor-pointer transition-colors disabled:opacity-50"
              >
                Отмена
              </button>
              <button
                type="button"
                disabled={
                  resolveModal.submitting ||
                  (resolveModal.caseType === 'missing_live_allocated' &&
                    resolveModal.actionId === 'confirm_missing' &&
                    !resolveModal.selectedReplacementUnitId.trim())
                }
                onClick={handleSubmitResolve}
                className={`px-4 py-2 text-xs font-semibold text-white rounded-lg cursor-pointer transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed ${
                  resolveModal.actionId === 'close_stale_allocation'
                    ? 'bg-amber-600 hover:bg-amber-700'
                    : 'bg-rose-600 hover:bg-rose-700'
                }`}
              >
                {resolveModal.submitting ? 'Выполняется...' : resolveModal.actionLabel}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Cancel Confirmation Modal */}
      {showCancelModal && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl border border-slate-200">
            <div className="flex items-center gap-3 text-rose-600 mb-3">
              <AlertTriangle className="w-6 h-6 shrink-0" />
              <h3 className="text-lg font-bold text-slate-900">Отмена инвентаризации</h3>
            </div>
            <p className="text-sm text-slate-600 mb-6">
              Инвентаризация будет отменена. Уже собранные данные сохранятся в истории.
            </p>
            <div className="flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => setShowCancelModal(false)}
                className="px-4 py-2 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg cursor-pointer transition-colors"
              >
                Вернуться к проверке
              </button>
              <button
                type="button"
                onClick={handleConfirmCancel}
                className="px-4 py-2 text-xs font-semibold text-white bg-rose-600 hover:bg-rose-700 rounded-lg cursor-pointer transition-colors shadow-sm"
              >
                Подтвердить отмену
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
