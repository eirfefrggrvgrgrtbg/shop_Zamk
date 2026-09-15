import React, { useState, useRef, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  PackageSearch,
  ArrowRight,
  XCircle,
  CheckCircle2,
  Box,
  Store,
  RefreshCw,
  Check,
  AlertTriangle,
  Clock,
  ShieldAlert,
} from 'lucide-react';
import {
  resolvePhysicalUnit,
  processFoundUnit,
  finalizeSupplyReceivingSession,
  ResolvedPhysicalUnit,
  ProcessFoundUnitResponse,
} from '@zamk/api-client/src/admin';
import { playBeepSound } from '../utils/audio';
import { normalizeScannerCode } from '../utils/scanner';
import { useAdminAuth } from '../contexts/AdminAuthContext';

export function AdminFreeScanner() {
  const { hasPermission } = useAdminAuth();
  const canReceive = hasPermission('inventory.receipt');

  const [searchParams] = useSearchParams();
  const initialCode = (searchParams.get('q') || searchParams.get('code') || searchParams.get('unitCode') || '').trim();
  const [unitCode, setUnitCode] = useState(initialCode);
  const [isDamaged, setIsDamaged] = useState(false);

  const [loading, setLoading] = useState(false);
  const [isActioning, setIsActioning] = useState(false);
  const [finalizing, setFinalizing] = useState(false);
  const [isFinalized, setIsFinalized] = useState(false);

  const [error, setError] = useState<string | null>(null);
  const [actionSuccessMessage, setActionSuccessMessage] = useState<string | null>(null);

  const [resolvedUnit, setResolvedUnit] = useState<ResolvedPhysicalUnit | null>(null);
  const [actionResult, setActionResult] = useState<ProcessFoundUnitResponse | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (initialCode) {
      setUnitCode(initialCode);
    }
  }, [initialCode]);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleScan = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = normalizeScannerCode(unitCode);
    if (!code) return;

    setLoading(true);
    setError(null);
    setActionSuccessMessage(null);
    setIsFinalized(false);
    setActionResult(null);

    try {
      // Step 1: NON-MUTATING Resolve
      const data = await resolvePhysicalUnit(code);
      setResolvedUnit(data);
      setIsDamaged(false);
      playBeepSound('success');
    } catch (err: any) {
      if (err?.code === 'unit_not_found' || err?.code === 'UNIT_NOT_FOUND' || err?.status === 404) {
        setError('Физическая единица с таким кодом не найдена.');
      } else {
        setError(err?.message || 'Ошибка при поиске единицы');
      }
      setResolvedUnit(null);
      playBeepSound('error');
    } finally {
      setLoading(false);
      setUnitCode('');
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  };

  const handleReceiveAction = async () => {
    if (!resolvedUnit || isActioning || !canReceive) return;

    setIsActioning(true);
    setError(null);
    setActionSuccessMessage(null);

    try {
      // Step 2: Explicit Receiving Mutation
      const data = await processFoundUnit({
        unitCode: resolvedUnit.unitCode,
        condition: isDamaged ? 'damaged' : 'ok',
      });
      setActionResult(data);
      setActionSuccessMessage(
        isDamaged ? 'Единица принята на склад как брак.' : 'Единица успешно принята на склад.'
      );
      playBeepSound('success');

      // Refresh non-mutating resolved view to show new state
      try {
        const refreshed = await resolvePhysicalUnit(resolvedUnit.unitCode);
        setResolvedUnit(refreshed);
      } catch (_) {
        // Fallback to local status update if refresh fails
        setResolvedUnit({
          ...resolvedUnit,
          unitStatus: isDamaged ? 'damaged' : 'warehouse',
          recommendedAction: isDamaged ? 'already_damaged' : 'already_in_warehouse',
        });
      }
    } catch (err: any) {
      setError(err?.message || 'Ошибка при выполнении доприёмки');
      playBeepSound('error');
    } finally {
      setIsActioning(false);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  };

  const handleFinalize = async () => {
    const sessionId = actionResult?.receivingSessionId || resolvedUnit?.receivingState?.activeReceivingSessionId;
    if (!sessionId || finalizing) return;

    setFinalizing(true);
    setError(null);

    try {
      await finalizeSupplyReceivingSession(sessionId, {});
      setIsFinalized(true);
      playBeepSound('success');

      if (resolvedUnit?.unitCode) {
        try {
          const updated = await resolvePhysicalUnit(resolvedUnit.unitCode);
          setResolvedUnit(updated);
        } catch (_) {}
      }
    } catch (err: any) {
      setError(err?.message || 'Не удалось завершить доприёмку');
      playBeepSound('error');
    } finally {
      setFinalizing(false);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  };

  const handleReset = () => {
    setResolvedUnit(null);
    setActionResult(null);
    setError(null);
    setActionSuccessMessage(null);
    setIsFinalized(false);
    setUnitCode('');
    setIsDamaged(false);
    inputRef.current?.focus();
  };

  const getStatusDisplay = (status: string) => {
    switch (status) {
      case 'expected':
        return {
          label: 'Ожидается на приёмке',
          desc: 'Единица числится в поставке, но ещё не оприходована на склад ZAMK.',
          type: 'warning',
          icon: <Clock className="w-5 h-5 text-amber-600" />,
        };
      case 'warehouse':
        return {
          label: 'Уже на складе',
          desc: 'Этот товар уже оприходован и находится на складе.',
          type: 'success',
          icon: <CheckCircle2 className="w-5 h-5 text-emerald-600" />,
        };
      case 'damaged':
        return {
          label: 'Повреждено / Брак',
          desc: 'Эта единица уже отмечена как брак.',
          type: 'error',
          icon: <XCircle className="w-5 h-5 text-rose-600" />,
        };
      case 'shipped':
        return {
          label: 'Отгружено',
          desc: 'Эта единица уже отгружена со склада покупателю.',
          type: 'neutral',
          icon: <ArrowRight className="w-5 h-5 text-slate-600" />,
        };
      case 'written_off':
        return {
          label: 'Списано',
          desc: 'Эта единица списана со склада.',
          type: 'neutral',
          icon: <XCircle className="w-5 h-5 text-slate-600" />,
        };
      default:
        return {
          label: `Статус: ${status}`,
          desc: '',
          type: 'neutral',
          icon: null,
        };
    }
  };

  return (
    <div className="max-w-4xl mx-auto py-8 px-4">
      <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden mb-6">
        <div className="p-6 border-b border-slate-200 bg-slate-50 flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-indigo-100 flex items-center justify-center text-indigo-600">
            <PackageSearch className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-900">Свободный сканер ZMU</h1>
            <p className="text-sm text-slate-500">
              Идентификация физической единицы без привязки к сессии. Сканирование только определяет статус товара.
            </p>
          </div>
        </div>

        <div className="p-6">
          <form onSubmit={handleScan} className="space-y-4">
            <div className="flex gap-3">
              <div className="relative flex-1">
                <input
                  ref={inputRef}
                  type="text"
                  value={unitCode}
                  onChange={(e) => setUnitCode(e.target.value.toUpperCase())}
                  placeholder="Отсканируйте ZMU физической единицы"
                  className="w-full pl-4 pr-12 py-4 text-xl font-mono uppercase bg-slate-50 border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 transition-shadow"
                  disabled={loading || isActioning || finalizing}
                  autoFocus
                />
                {loading && (
                  <div className="absolute right-4 top-1/2 -translate-y-1/2">
                    <RefreshCw className="w-6 h-6 text-slate-400 animate-spin" />
                  </div>
                )}
              </div>
              <button
                type="submit"
                disabled={!unitCode.trim() || loading || isActioning || finalizing}
                className="px-8 py-4 bg-indigo-600 hover:bg-indigo-700 text-white font-medium rounded-lg disabled:opacity-50 transition-colors"
              >
                Найти
              </button>
            </div>
          </form>

          {error && (
            <div className="mt-4 p-4 bg-red-50 text-red-700 rounded-lg flex items-center gap-2">
              <XCircle className="w-5 h-5 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}
        </div>
      </div>

      {resolvedUnit && (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden space-y-6 p-6">
          <div className="flex justify-between items-start border-b border-slate-100 pb-4">
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-2xl font-bold font-mono text-slate-900">{resolvedUnit.unitCode}</h2>
                {(() => {
                  const statusInfo = getStatusDisplay(resolvedUnit.unitStatus);
                  const badgeColors =
                    statusInfo.type === 'success'
                      ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                      : statusInfo.type === 'warning'
                      ? 'bg-amber-50 text-amber-700 border-amber-200'
                      : statusInfo.type === 'error'
                      ? 'bg-rose-50 text-rose-700 border-rose-200'
                      : 'bg-slate-100 text-slate-700 border-slate-200';
                  return (
                    <span
                      data-testid="unit-status-badge"
                      className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold border ${badgeColors}`}
                    >
                      {statusInfo.icon}
                      <span>{statusInfo.label}</span>
                    </span>
                  );
                })()}
              </div>
              <div className="text-slate-600 font-medium text-base mt-1">
                {resolvedUnit.product?.title || 'Товарная единица'}
              </div>
            </div>
            <button
              onClick={handleReset}
              className="text-sm text-slate-500 hover:text-slate-700 px-3 py-1.5 rounded-lg border border-slate-200 hover:bg-slate-50 transition-colors"
            >
              Очистить
            </button>
          </div>

          {/* Unit Details & Origin */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-slate-50 rounded-lg p-4 border border-slate-100">
              <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Детали товара</h3>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <dt className="text-slate-500">SKU продавца</dt>
                  <dd className="font-mono text-slate-900">{resolvedUnit.variant?.sellerSku || '—'}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-slate-500">Штрихкод</dt>
                  <dd className="font-mono text-slate-900">{resolvedUnit.variant?.barcode || '—'}</dd>
                </div>
                {(resolvedUnit.variant?.color || resolvedUnit.variant?.size) && (
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Вариант</dt>
                    <dd className="text-slate-900">
                      {[resolvedUnit.variant?.color, resolvedUnit.variant?.size].filter(Boolean).join(' / ')}
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            <div className="bg-slate-50 rounded-lg p-4 border border-slate-100">
              <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Происхождение</h3>
              <dl className="space-y-2 text-sm">
                <div className="flex justify-between items-center">
                  <dt className="text-slate-500">Поставка</dt>
                  <dd className="font-medium text-slate-900 flex items-center gap-1.5">
                    <Box className="w-4 h-4 text-slate-400" /> {resolvedUnit.origin?.supplyNumber}
                  </dd>
                </div>
                <div className="flex justify-between items-center">
                  <dt className="text-slate-500">Продавец</dt>
                  <dd className="font-medium text-slate-900 flex items-center gap-1.5">
                    <Store className="w-4 h-4 text-slate-400" /> {resolvedUnit.origin?.sellerName || '—'}
                  </dd>
                </div>
                {resolvedUnit.origin?.boxNumber && (
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Коробка</dt>
                    <dd className="text-slate-900">{resolvedUnit.origin.boxNumber}</dd>
                  </div>
                )}
              </dl>
            </div>
          </div>

          {/* Success Banner */}
          {actionSuccessMessage && (
            <div className="p-4 bg-emerald-50 border border-emerald-200 text-emerald-800 rounded-xl flex items-center gap-3 text-sm font-medium">
              <CheckCircle2 className="w-5 h-5 text-emerald-600 shrink-0" />
              <span>{actionSuccessMessage}</span>
            </div>
          )}

          {isFinalized && (
            <div className="p-4 bg-emerald-100 border border-emerald-300 text-emerald-800 rounded-xl flex items-center gap-2 font-medium">
              <Check className="w-5 h-5 flex-shrink-0" />
              <span>Доприёмка завершена. Поставка обновлена.</span>
            </div>
          )}

          {/* Action section depending on status */}
          {resolvedUnit.unitStatus === 'expected' ? (
            <div className="bg-amber-50/70 border border-amber-200 rounded-xl p-6 space-y-4">
              <div className="flex items-start gap-3">
                <AlertTriangle className="w-6 h-6 text-amber-600 shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <h4 className="font-bold text-base text-gray-900">Товар ожидает приёмки</h4>
                  <p className="text-xs text-amber-900 leading-relaxed">
                    Эта товарная единица числится в поставке <strong>{resolvedUnit.origin?.supplyNumber}</strong>,
                    но ещё не оприходована на склад. Для включения товара в складской остаток выполните доприёмку.
                  </p>
                </div>
              </div>

              <div className="pt-3 border-t border-amber-200/60 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <label className="inline-flex items-center gap-2 text-sm font-medium text-slate-700 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={isDamaged}
                    onChange={(e) => setIsDamaged(e.target.checked)}
                    className="w-4 h-4 text-red-600 rounded border-slate-300 focus:ring-red-500"
                    disabled={isActioning}
                  />
                  <span className={isDamaged ? 'text-red-700 font-semibold' : 'text-slate-600'}>
                    Найденный товар — брак
                  </span>
                </label>

                <div className="flex items-center gap-3">
                  {!canReceive && (
                    <span className="text-xs text-rose-600 font-medium flex items-center gap-1">
                      <ShieldAlert className="w-4 h-4" />
                      Требуются права inventory.receipt
                    </span>
                  )}
                  <button
                    onClick={handleReceiveAction}
                    disabled={!canReceive || isActioning}
                    className="px-6 py-2.5 bg-amber-600 hover:bg-amber-700 disabled:bg-gray-200 disabled:text-gray-400 text-white font-bold text-sm rounded-xl transition-all shadow-sm flex items-center gap-2"
                  >
                    {isActioning ? (
                      <>
                        <RefreshCw className="w-4 h-4 animate-spin" />
                        <span>Приёмка...</span>
                      </>
                    ) : (
                      <span>Допринять на склад</span>
                    )}
                  </button>
                </div>
              </div>

              {actionResult && actionResult.sessionRemaining === 0 && !isFinalized && (
                <div className="pt-4 border-t border-amber-200/60 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                  <span className="font-medium text-xs text-amber-900">
                    Все ожидаемые единицы в текущей сессии доприёмки приняты.
                  </span>
                  <button
                    onClick={handleFinalize}
                    disabled={finalizing}
                    className="px-4 py-2 bg-emerald-600 hover:bg-emerald-700 text-white font-medium text-xs rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
                  >
                    {finalizing && <RefreshCw className="w-3.5 h-3.5 animate-spin" />}
                    <span>Завершить доприёмку</span>
                  </button>
                </div>
              )}
            </div>
          ) : (
            <div className="bg-slate-50 border border-slate-200 rounded-xl p-5 text-sm text-slate-700">
              <div className="flex items-start gap-3">
                <div className="mt-0.5">{getStatusDisplay(resolvedUnit.unitStatus).icon}</div>
                <div className="space-y-1">
                  <h4 className="font-bold text-gray-900">{getStatusDisplay(resolvedUnit.unitStatus).label}</h4>
                  <p className="text-xs text-slate-600">{getStatusDisplay(resolvedUnit.unitStatus).desc}</p>
                  <p className="text-xs text-slate-500 pt-1">
                    Повторное оприходование не требуется. Физическая единица уже имеет актуальный статус.
                  </p>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
