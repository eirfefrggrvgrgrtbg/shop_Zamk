import { useState } from 'react';
import {
  ShieldAlert,
  CheckCircle2,
  AlertCircle,
  Clock,
  Edit3,
  X,
} from 'lucide-react';
import {
  updateReturnResponsibility,
  getAdminReturnErrorMessage,
  getResponsibilityPartyLabel,
  getResponsibilityReasonLabel,
} from '../../api/adminReturns';
import type {
  ReturnResponsibilityAllocation,
  UpdateReturnResponsibilityRequest,
} from '../../api/adminReturns';
import { PermissionGuard } from '../PermissionGuard';
import { useAdminAuth } from '../../contexts/AdminAuthContext';

interface ReturnResponsibilityCardProps {
  returnId: string;
  allocations: ReturnResponsibilityAllocation[];
  returnStatus: string;
  onUpdated: () => Promise<void> | void;
}

interface ResponsibilityChoice {
  party: 'zamk' | 'carrier' | 'seller';
  reasonCode: string;
  title: string;
  helperText: string;
}

const CHOICES: ResponsibilityChoice[] = [
  {
    party: 'zamk',
    reasonCode: 'zamk_warehouse_damage',
    title: 'Ответственность ZAMK',
    helperText:
      'Товар повреждён на складе ZAMK. Продавцу будет начислена компенсация за отменённый доход согласно регламенту платформы.',
  },
  {
    party: 'carrier',
    reasonCode: 'carrier_damage',
    title: 'Ответственность перевозчика',
    helperText:
      'Товар повреждён при транспортировке службой доставки. Продавцу будет начислена компенсация через стандартный компенсационный механизм.',
  },
  {
    party: 'seller',
    reasonCode: 'seller_product_defect',
    title: 'Ответственность продавца',
    helperText:
      'Повреждение или брак возникли по вине продавца или товара. Компенсация от ZAMK продавцу не выплачивается.',
  },
];

const RESOLUTION_ELIGIBLE_RETURN_STATUSES = ['item_received', 'refunded', 'completed'];

function formatDateTime(dateStr?: string | null): string {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function ReturnResponsibilityCard({
  returnId,
  allocations,
  returnStatus,
  onUpdated,
}: ReturnResponsibilityCardProps) {
  const { hasPermission } = useAdminAuth();
  const canUpdateStatus = hasPermission('returns.update_status');

  // Track editing state per allocation ID
  const [editingAllocId, setEditingAllocId] = useState<string | null>(null);
  const [selectedParty, setSelectedParty] = useState<'zamk' | 'carrier' | 'seller' | null>(null);
  const [internalNote, setInternalNote] = useState('');
  const [isConfirmModalOpen, setIsConfirmModalOpen] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Return is physically received / inspected
  const isPhysicallyInspected = RESOLUTION_ELIGIBLE_RETURN_STATUSES.includes(returnStatus);

  if (!allocations || allocations.length === 0) {
    return null;
  }

  const handleStartEdit = (alloc: ReturnResponsibilityAllocation) => {
    setEditingAllocId(alloc.id);
    if (alloc.responsibleParty && (alloc.responsibleParty === 'zamk' || alloc.responsibleParty === 'carrier' || alloc.responsibleParty === 'seller')) {
      setSelectedParty(alloc.responsibleParty as 'zamk' | 'carrier' | 'seller');
    } else {
      setSelectedParty(null);
    }
    setInternalNote(alloc.internalNote || '');
    setError(null);
  };

  const handleCancelEdit = () => {
    setEditingAllocId(null);
    setSelectedParty(null);
    setInternalNote('');
    setError(null);
  };

  const handleOpenConfirm = (alloc: ReturnResponsibilityAllocation) => {
    setEditingAllocId(alloc.id);
    if (!selectedParty) return;
    setError(null);
    setIsConfirmModalOpen(true);
  };

  const handleCloseConfirm = () => {
    if (isSubmitting) return;
    setIsConfirmModalOpen(false);
  };

  const activeChoice = CHOICES.find((c) => c.party === selectedParty);
  const targetAlloc = (editingAllocId ? allocations.find((a) => a.id === editingAllocId) : null) || allocations[0];

  const handleConfirmSubmit = async () => {
    if (!targetAlloc || !activeChoice) return;

    try {
      setIsSubmitting(true);
      setError(null);

      const payload: UpdateReturnResponsibilityRequest = {
        responsibleParty: activeChoice.party,
        reasonCode: activeChoice.reasonCode,
      };

      const trimmedNote = internalNote.trim();
      if (trimmedNote) {
        payload.internalNote = trimmedNote;
      }

      await updateReturnResponsibility(returnId, targetAlloc.id, payload);

      setIsConfirmModalOpen(false);
      setEditingAllocId(null);
      setSelectedParty(null);
      setInternalNote('');

      // Reload canonical return state
      await onUpdated();
    } catch (err: unknown) {
      const msg = getAdminReturnErrorMessage(err, 'Не удалось сохранить финансовую ответственность.');
      setError(msg);
      setIsConfirmModalOpen(false);

      // In case of 404, refresh canonical state
      if (typeof err === 'object' && err !== null && 'status' in err && (err as { status: number }).status === 404) {
        await onUpdated();
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div data-testid="return-responsibility-card" className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 space-y-5">
      <div className="flex items-center justify-between border-b border-gray-100 pb-3">
        <div className="flex items-center space-x-2">
          <ShieldAlert className="h-5 w-5 text-gray-500" />
          <h2 className="text-base font-semibold text-gray-900">Финансовая ответственность</h2>
        </div>
      </div>

      {error && (
        <div data-testid="responsibility-error-banner" className="p-3.5 bg-red-50 border border-red-200 rounded-lg text-xs text-red-900 flex items-start space-x-2">
          <AlertCircle className="h-4 w-4 text-red-600 mt-0.5 flex-shrink-0" />
          <div>
            <span className="font-semibold block">Ошибка</span>
            <span>{error}</span>
          </div>
        </div>
      )}

      <div className="space-y-4">
        {allocations.map((alloc, idx) => {
          const isPending = alloc.status === 'pending';
          const isResolved = alloc.status === 'resolved';
          const isNotRequired = alloc.status === 'not_required';
          const isDamaged = alloc.legacyDisposition === 'damaged' || isPending || isResolved;

          // Eligibility rule:
          // Active resolution controls are eligible only when:
          // 1. Return is physically inspected (item_received / refunded / completed)
          // 2. Allocation is damaged / compensable (not clean restocked / not_required)
          const isResolutionEligible = isPhysicallyInspected && isDamaged && !isNotRequired;
          const isCurrentEditing = editingAllocId === alloc.id || (isPending && isResolutionEligible && editingAllocId === null);

          return (
            <div
              key={alloc.id}
              data-testid={`responsibility-allocation-${alloc.id}`}
              className="p-4 rounded-xl border border-gray-200 bg-gray-50/50 space-y-4 text-sm"
            >
              {allocations.length > 1 && (
                <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                  Позиция {idx + 1} (Ед. {alloc.quantity} шт.)
                </div>
              )}

              {/* Status Header */}
              <div className="flex items-center justify-between flex-wrap gap-2">
                <div className="flex items-center space-x-2">
                  <span className="text-xs text-gray-500 font-medium">Статус:</span>
                  {isPending ? (
                    <span
                      data-testid="responsibility-status-badge"
                      className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-50 text-amber-800 border border-amber-200"
                    >
                      <Clock className="h-3.5 w-3.5 mr-1 text-amber-600" />
                      Требуется решение
                    </span>
                  ) : isResolved ? (
                    <span
                      data-testid="responsibility-status-badge"
                      className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-green-50 text-green-800 border border-green-200"
                    >
                      <CheckCircle2 className="h-3.5 w-3.5 mr-1 text-green-600" />
                      Решение принято
                    </span>
                  ) : (
                    <span
                      data-testid="responsibility-status-badge"
                      className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-gray-100 text-gray-700 border border-gray-200"
                    >
                      Не требуется
                    </span>
                  )}
                </div>

                {/* Physical Fact (Read-Only) */}
                <div data-testid="physical-fact-display" className="text-xs text-gray-700 font-medium bg-white px-2.5 py-1 rounded-md border border-gray-200">
                  {alloc.legacyDisposition === 'damaged' || (!alloc.legacyDisposition && isDamaged) ? (
                    <span className="text-red-700 font-semibold">Повреждено: {alloc.quantity} шт.</span>
                  ) : (
                    <span className="text-gray-700">Принято на склад: {alloc.quantity} шт.</span>
                  )}
                </div>
              </div>

              {/* Clean / Non-eligible allocation without active controls */}
              {!isResolutionEligible && (
                <div className="text-xs text-gray-500 py-1">
                  {!isPhysicallyInspected ? (
                    <span>Приёмка на складе ещё не завершена. Определение финансовой ответственности станет доступно после приёмки.</span>
                  ) : isNotRequired ? (
                    <span>Товар принят на склад без повреждений. Финансовая ответственность не требуется.</span>
                  ) : (
                    <span>Определение финансовой ответственности для данной позиции не требуется.</span>
                  )}
                </div>
              )}

              {/* Resolved presentation (Read-only view when not editing) */}
              {isResolved && !isCurrentEditing && isResolutionEligible && (
                <div className="space-y-3 pt-1">
                  <div className="bg-white p-3.5 rounded-lg border border-gray-200 space-y-2">
                    <div className="flex items-center justify-between flex-wrap gap-1">
                      <span className="text-xs text-gray-500">Ответственная сторона:</span>
                      <span data-testid="resolved-party" className="text-sm font-bold text-gray-900">
                        {getResponsibilityPartyLabel(alloc.responsibleParty)}
                      </span>
                    </div>

                    <div className="flex items-center justify-between flex-wrap gap-1">
                      <span className="text-xs text-gray-500">Причина:</span>
                      <span data-testid="resolved-reason" className="text-xs font-medium text-gray-800">
                        {getResponsibilityReasonLabel(alloc.reasonCode)}
                      </span>
                    </div>

                    {alloc.decidedAt && (
                      <div className="flex items-center justify-between flex-wrap gap-1 text-xs text-gray-500 pt-1 border-t border-gray-100">
                        <span>Дата решения:</span>
                        <span data-testid="resolved-date" className="font-mono text-gray-700">
                          {formatDateTime(alloc.decidedAt)}
                        </span>
                      </div>
                    )}

                    {alloc.internalNote && (
                      <div className="pt-1 border-t border-gray-100 text-xs">
                        <span className="text-gray-500 font-medium block">Внутренний комментарий:</span>
                        <p data-testid="resolved-internal-note" className="text-gray-800 mt-0.5 whitespace-pre-wrap">
                          {alloc.internalNote}
                        </p>
                      </div>
                    )}
                  </div>

                  <PermissionGuard permission="returns.update_status">
                    <button
                      type="button"
                      data-testid="btn-change-responsibility"
                      onClick={() => handleStartEdit(alloc)}
                      disabled={isSubmitting}
                      className="inline-flex items-center px-3 py-1.5 bg-white hover:bg-gray-50 text-gray-700 text-xs font-medium rounded-lg border border-gray-300 shadow-sm transition-colors"
                    >
                      <Edit3 className="h-3.5 w-3.5 mr-1.5 text-gray-500" />
                      Изменить решение
                    </button>
                  </PermissionGuard>
                </div>
              )}

              {/* Active Decision Form (Pending or Editing) */}
              {isResolutionEligible && isCurrentEditing && (
                <div className="space-y-4 pt-1">
                  <p className="text-xs text-gray-600 leading-relaxed">
                    Склад зафиксировал повреждение товара при приёмке. Определите, на чьей стороне лежит финансовая ответственность за зафиксированный ущерб. Физический результат приёмки остаётся неизменным.
                  </p>

                  <div className="space-y-2.5">
                    {CHOICES.map((choice) => {
                      const isSelected = selectedParty === choice.party;
                      return (
                        <label
                          key={choice.party}
                          data-testid={`choice-${choice.party}`}
                          className={`block p-3.5 rounded-lg border cursor-pointer transition-all ${
                            isSelected
                              ? 'border-indigo-600 bg-indigo-50/60 ring-1 ring-indigo-600'
                              : 'border-gray-200 bg-white hover:border-gray-300'
                          }`}
                        >
                          <div className="flex items-start space-x-3">
                            <input
                              type="radio"
                              name={`responsibility-choice-${alloc.id}`}
                              value={choice.party}
                              checked={isSelected}
                              onChange={() => {
                                setEditingAllocId(alloc.id);
                                setSelectedParty(choice.party);
                              }}
                              disabled={!canUpdateStatus || isSubmitting}
                              className="mt-0.5 h-4 w-4 text-indigo-600 border-gray-300 focus:ring-indigo-500"
                            />
                            <div className="space-y-1">
                              <span className="text-sm font-semibold text-gray-900 block">
                                {choice.title}
                              </span>
                              <span className="text-xs text-gray-600 block leading-relaxed">
                                {choice.helperText}
                              </span>
                            </div>
                          </div>
                        </label>
                      );
                    })}
                  </div>

                  {/* Optional internal note */}
                  <div>
                    <label
                      htmlFor={`internal-note-${alloc.id}`}
                      className="block text-xs font-medium text-gray-700 mb-1"
                    >
                      Внутренний комментарий (не виден продавцу, опционально)
                    </label>
                    <textarea
                      id={`internal-note-${alloc.id}`}
                      data-testid="input-internal-note"
                      rows={2}
                      value={internalNote}
                      onChange={(e) => setInternalNote(e.target.value)}
                      disabled={!canUpdateStatus || isSubmitting}
                      placeholder="Укажите номер акта, детали инцидента или обоснование..."
                      className="w-full rounded-lg border border-gray-300 px-3 py-2 text-xs shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:bg-gray-50"
                    />
                  </div>

                  {/* Action buttons */}
                  <div className="flex items-center space-x-2 pt-1">
                    <PermissionGuard
                      permission="returns.update_status"
                      fallback={
                        <p className="text-xs text-gray-500">
                          У вас нет прав для изменения финансовой ответственности.
                        </p>
                      }
                    >
                      <button
                        type="button"
                        data-testid="btn-submit-responsibility"
                        onClick={() => handleOpenConfirm(alloc)}
                        disabled={!selectedParty || isSubmitting}
                        className="inline-flex items-center px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold rounded-lg shadow-sm transition-colors disabled:opacity-50"
                      >
                        Подтвердить решение
                      </button>
                    </PermissionGuard>

                    {/* If we were editing an already resolved allocation, allow canceling */}
                    {isResolved && (
                      <button
                        type="button"
                        data-testid="btn-cancel-edit"
                        onClick={handleCancelEdit}
                        disabled={isSubmitting}
                        className="inline-flex items-center px-3 py-2 bg-white hover:bg-gray-50 text-gray-700 text-xs font-medium rounded-lg border border-gray-300 shadow-sm transition-colors"
                      >
                        Отмена
                      </button>
                    )}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Confirmation Modal */}
      {isConfirmModalOpen && activeChoice && targetAlloc && (
        <div
          data-testid="responsibility-confirm-modal"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          role="dialog"
          aria-modal="true"
        >
          <div className="bg-white rounded-xl max-w-md w-full p-6 shadow-xl space-y-4 animate-in fade-in zoom-in-95 duration-150">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <h3 className="text-base font-bold text-gray-900">
                Подтверждение решения
              </h3>
              <button
                type="button"
                onClick={handleCloseConfirm}
                disabled={isSubmitting}
                className="text-gray-400 hover:text-gray-500 rounded-lg p-1"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="space-y-3 text-xs text-gray-700 leading-relaxed">
              <div className="p-3 bg-amber-50 border border-amber-200 rounded-lg text-amber-900 space-y-1">
                <div className="font-semibold">Обратите внимание:</div>
                <ul className="list-disc list-inside space-y-0.5">
                  <li>Физический результат приёмки не изменится (товар остаётся повреждённым).</li>
                  <li>
                    Будет зафиксирована финансовая ответственность:{' '}
                    <strong>{activeChoice.title}</strong> ({getResponsibilityReasonLabel(activeChoice.reasonCode)}).
                  </li>
                  {activeChoice.party === 'zamk' || activeChoice.party === 'carrier' ? (
                    <li>Продавцу автоматически может быть начислена компенсация за отменённый доход.</li>
                  ) : (
                    <li>Компенсация продавцу начислена не будет.</li>
                  )}
                </ul>
              </div>

              {targetAlloc.status === 'resolved' && (
                <div className="p-3 bg-blue-50 border border-blue-200 rounded-lg text-blue-900">
                  <span className="font-semibold block mb-0.5">Корректировка решения:</span>
                  <span>
                    Предыдущие финансовые проводки не перезаписываются: система автоматически выполнит перерасчёт и скорректирует баланс продавца. Новое решение станет актуальной истиной.
                  </span>
                </div>
              )}

              {internalNote.trim() && (
                <div className="pt-1">
                  <span className="font-medium text-gray-500 block">Внутренний комментарий:</span>
                  <p className="mt-0.5 p-2 bg-gray-50 rounded border border-gray-200 text-gray-800 italic whitespace-pre-wrap">
                    {internalNote.trim()}
                  </p>
                </div>
              )}
            </div>

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-gray-100">
              <button
                type="button"
                data-testid="btn-modal-cancel"
                onClick={handleCloseConfirm}
                disabled={isSubmitting}
                className="px-3 py-2 bg-white hover:bg-gray-50 text-gray-700 text-xs font-medium rounded-lg border border-gray-300 transition-colors"
              >
                Отмена
              </button>
              <button
                type="button"
                data-testid="btn-modal-confirm"
                onClick={handleConfirmSubmit}
                disabled={isSubmitting}
                className="inline-flex items-center px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold rounded-lg shadow-sm transition-colors disabled:opacity-50"
              >
                {isSubmitting ? (
                  <>
                    <div className="animate-spin rounded-full h-3.5 w-3.5 border-b-2 border-white mr-1.5" />
                    Сохранение...
                  </>
                ) : (
                  'Подтвердить'
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
