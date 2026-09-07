import { useEffect, useRef, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  Barcode,
  CheckCircle2,
  AlertTriangle,
  AlertCircle,
  Package,
  Clock,
  Info,
  Layers,
} from 'lucide-react';
import { PermissionGuard } from '../components/PermissionGuard';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import {
  getAdminReturnReceivingState,
  startAdminReturnReceiving,
  scanAdminReturnUnit,
  inspectSerializedReturnUnit,
  inspectLegacyReturnItem,
  finalizeAdminReturnReceiving,
  getAdminReturnErrorMessage,
  getReturnReasonLabel,
  getReturnStatusLabel,
  getStatusBadgeClass,
} from '../api/adminReturns';
import type {
  AdminReturnReceivingState,
} from '@zamk/api-client/src/types';
import { playBeepSound } from '../utils/audio';

export function AdminReturnReceiving() {
  const { id } = useParams<{ id: string }>();
  const { hasPermission } = useAdminAuth();
  const canReceive = hasPermission('warehouse.returns');
  const canViewCustomerComment = hasPermission('returns.read');

  const [state, setState] = useState<AdminReturnReceivingState | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [infoMsg, setInfoMsg] = useState<string | null>(null);

  // Scanner state
  const [barcodeInput, setBarcodeInput] = useState('');
  const [isScanning, setIsScanning] = useState(false);
  const scannerInputRef = useRef<HTMLInputElement>(null);

  // Start receiving state
  const [isStarting, setIsStarting] = useState(false);

  // Serialized inspection draft state: unitId -> { disposition, inspectedCondition, isSaving }
  const [unitDrafts, setUnitDrafts] = useState<
    Record<string, { disposition: 'restock' | 'damaged' | 'reject' | ''; inspectedCondition: string; isSaving?: boolean }>
  >({});

  // Legacy inspection draft state: itemId -> { accepted: number; damaged: number; rejected: number; isSaving?: boolean; error?: string | null }
  const [legacyDrafts, setLegacyDrafts] = useState<
    Record<string, { accepted: number; damaged: number; rejected: number; isSaving?: boolean; error?: string | null }>
  >({});

  // Finalize modal state
  const [isFinalizeModalOpen, setIsFinalizeModalOpen] = useState(false);
  const [isFinalizing, setIsFinalizing] = useState(false);

  const fetchState = async (showLoading = true) => {
    if (!id) return;
    try {
      if (showLoading) setIsLoading(true);
      setError(null);
      const data = await getAdminReturnReceivingState(id);
      setState(data);

      // Initialize legacy drafts from loaded state
      const newLegacyDrafts: Record<string, { accepted: number; damaged: number; rejected: number }> = {};
      data.items.forEach((it) => {
        if (it.allocationMode === 'legacy') {
          newLegacyDrafts[it.returnItem.id] = {
            accepted: it.acceptedQuantity ?? it.returnItem.acceptedQuantity ?? 0,
            damaged: it.damagedQuantity ?? it.returnItem.damagedQuantity ?? 0,
            rejected: it.rejectedQuantity ?? it.returnItem.rejectedQuantity ?? 0,
          };
        }
      });
      setLegacyDrafts(newLegacyDrafts);

      // Initialize unit drafts from loaded scanned units
      const newUnitDrafts: Record<string, { disposition: 'restock' | 'damaged' | 'reject' | ''; inspectedCondition: string }> = {};
      data.items.forEach((it) => {
        it.scannedUnits.forEach((u) => {
          newUnitDrafts[u.id] = {
            disposition: (u.disposition as 'restock' | 'damaged' | 'reject') || '',
            inspectedCondition: u.inspectedCondition || '',
          };
        });
      });
      setUnitDrafts(newUnitDrafts);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось загрузить данные складской приёмки.'));
    } finally {
      if (showLoading) setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchState();
  }, [id]);

  useEffect(() => {
    if (state?.return.status === 'receiving' && canReceive) {
      scannerInputRef.current?.focus();
    }
  }, [state?.return.status, canReceive]);

  const handleStartReceiving = async () => {
    if (!id) return;
    try {
      setIsStarting(true);
      setError(null);
      setSuccessMsg(null);
      await startAdminReturnReceiving(id);
      await fetchState(false);
      setSuccessMsg('Складская приёмка успешно начата.');
      playBeepSound('success');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось начать приёмку возврата.'));
      playBeepSound('error');
    } finally {
      setIsStarting(false);
    }
  };

  const handleScanSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const code = barcodeInput.trim();
    if (!code || !id || isScanning || !canReceive) return;

    try {
      setIsScanning(true);
      setError(null);
      setSuccessMsg(null);
      setInfoMsg(null);

      const resp = await scanAdminReturnUnit(id, code);
      setBarcodeInput('');

      if (resp.alreadyScanned || resp.isDuplicate) {
        setInfoMsg(`Единица ${code} уже была отсканирована ранее.`);
        playBeepSound('success');
      } else {
        setSuccessMsg(`Единица ${code} успешно отсканирована.`);
        playBeepSound('success');
      }

      await fetchState(false);
      setTimeout(() => {
        scannerInputRef.current?.focus();
      }, 50);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось отсканировать единицу товара.'));
      playBeepSound('error');
    } finally {
      setIsScanning(false);
    }
  };

  const handleUpdateUnitDisposition = async (unitId: string, disposition: 'restock' | 'damaged' | 'reject') => {
    if (!id) return;
    const currentDraft = unitDrafts[unitId] || { disposition: '', inspectedCondition: '' };
    const newDraft = { ...currentDraft, disposition };
    setUnitDrafts((prev) => ({ ...prev, [unitId]: newDraft }));

    try {
      setError(null);
      await inspectSerializedReturnUnit(id, unitId, {
        disposition,
        inspectedCondition: newDraft.inspectedCondition ? newDraft.inspectedCondition.trim() : undefined,
      });
      await fetchState(false);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось сохранить решение по единице.'));
      playBeepSound('error');
    }
  };

  const handleSaveUnitCondition = async (unitId: string) => {
    if (!id) return;
    const currentDraft = unitDrafts[unitId];
    if (!currentDraft || !currentDraft.disposition) return;

    try {
      setError(null);
      await inspectSerializedReturnUnit(id, unitId, {
        disposition: currentDraft.disposition,
        inspectedCondition: currentDraft.inspectedCondition ? currentDraft.inspectedCondition.trim() : undefined,
      });
      await fetchState(false);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось сохранить примечание к состоянию.'));
    }
  };

  const handleSaveLegacyItem = async (itemId: string, requestedQty: number) => {
    if (!id) return;
    const draft = legacyDrafts[itemId];
    if (!draft) return;

    const accepted = Number(draft.accepted) || 0;
    const damaged = Number(draft.damaged) || 0;
    const rejected = Number(draft.rejected) || 0;

    if (accepted < 0 || damaged < 0 || rejected < 0) {
      setLegacyDrafts((prev) => ({
        ...prev,
        [itemId]: { ...draft, error: 'Количества не могут быть отрицательными.' },
      }));
      return;
    }

    const sum = accepted + damaged + rejected;
    if (sum > requestedQty) {
      setLegacyDrafts((prev) => ({
        ...prev,
        [itemId]: {
          ...draft,
          error: `Сумма количеств (${sum}) превышает запрошенное количество (${requestedQty}).`,
        },
      }));
      return;
    }

    try {
      setLegacyDrafts((prev) => ({ ...prev, [itemId]: { ...draft, isSaving: true, error: null } }));
      setError(null);
      await inspectLegacyReturnItem(id, itemId, {
        acceptedQuantity: accepted,
        damagedQuantity: damaged,
        rejectedQuantity: rejected,
      });
      await fetchState(false);
      setSuccessMsg('Количества для позиции успешно сохранены.');
    } catch (err: unknown) {
      const errMsg = getAdminReturnErrorMessage(err, 'Не удалось сохранить результаты проверки.');
      setLegacyDrafts((prev) => ({ ...prev, [itemId]: { ...draft, isSaving: false, error: errMsg } }));
      setError(errMsg);
      playBeepSound('error');
    } finally {
      setLegacyDrafts((prev) => {
        if (!prev[itemId]) return prev;
        return { ...prev, [itemId]: { ...prev[itemId], isSaving: false } };
      });
    }
  };

  const handleFinalize = async () => {
    if (!id || isFinalizing) return;
    try {
      setIsFinalizing(true);
      setError(null);
      setSuccessMsg(null);
      await finalizeAdminReturnReceiving(id);
      await fetchState(false);
      setIsFinalizeModalOpen(false);
      setSuccessMsg('Складская приёмка успешно завершена. Статус возврата обновлён.');
      playBeepSound('success');
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось завершить складскую приёмку.'));
      playBeepSound('error');
    } finally {
      setIsFinalizing(false);
    }
  };

  const formatTimestamp = (ts?: string | null) => {
    if (!ts) return '—';
    const d = new Date(ts);
    return d.toLocaleString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600"></div>
        <span className="ml-3 text-sm text-gray-500 font-medium">Загрузка данных складской приёмки...</span>
      </div>
    );
  }

  if (!state) {
    return (
      <div className="p-6 bg-white rounded-xl shadow-sm border border-gray-200 text-center space-y-3">
        <AlertTriangle className="h-10 w-10 text-amber-500 mx-auto" />
        <h3 className="text-base font-bold text-gray-900">Возврат не найден</h3>
        <p className="text-sm text-gray-500">Не удалось загрузить данные по возврату {id}.</p>
        <Link to="/returns" className="inline-flex items-center text-sm font-semibold text-indigo-600 hover:text-indigo-800">
          <ArrowLeft className="h-4 w-4 mr-1" />
          Вернуться к списку возвратов
        </Link>
      </div>
    );
  }

  const isApproved = state.return.status === 'approved';
  const isReceiving = state.return.status === 'receiving';
  const isCompleted = state.return.status === 'item_received' || state.return.status === 'completed' || state.return.status === 'refunded';

  // Calculate overall summary stats
  let totalRestock = 0;
  let totalDamaged = 0;
  let totalReject = 0;
  let totalNotReceived = 0;

  state.items.forEach((item) => {
    if (item.allocationMode === 'serialized') {
      item.scannedUnits.forEach((u) => {
        if (u.disposition === 'restock') totalRestock += 1;
        else if (u.disposition === 'damaged') totalDamaged += 1;
        else if (u.disposition === 'reject') totalReject += 1;
      });
      totalNotReceived += Math.max(0, item.requestedQuantity - item.scannedUnits.length);
    } else {
      totalRestock += item.acceptedQuantity;
      totalDamaged += item.damagedQuantity;
      totalReject += item.rejectedQuantity;
      totalNotReceived += item.notReceivedQuantity ?? Math.max(0, item.requestedQuantity - (item.acceptedQuantity + item.damagedQuantity + item.rejectedQuantity));
    }
  });

  const totalScannedUnits = state.items.reduce((acc, item) => {
    if (item.allocationMode === 'serialized') {
      return acc + item.scannedUnits.length;
    }
    return acc + (item.acceptedQuantity + item.damagedQuantity + item.rejectedQuantity);
  }, 0);

  const isZeroScanned = totalScannedUnits === 0 && state.totalRequested > 0;
  const isPartialScanned = totalScannedUnits > 0 && totalScannedUnits < state.totalRequested;

  return (
    <PermissionGuard permission={['returns.read', 'warehouse.returns']}>
      <div className="space-y-6 max-w-7xl mx-auto pb-16">
        {/* Navigation & Header */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-gray-200 pb-4">
          <div>
            <Link to="/returns" className="inline-flex items-center text-sm font-medium text-gray-500 hover:text-gray-700 mb-2">
              <ArrowLeft className="h-4 w-4 mr-1" />
              К списку возвратов
            </Link>
            <div className="flex items-center space-x-3">
              <h1 className="text-2xl font-bold text-gray-900">
                Складская приёмка возврата
              </h1>
              <span
                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${getStatusBadgeClass(
                  state.return.status
                )}`}
              >
                {getReturnStatusLabel(state.return.status)}
              </span>
            </div>
            <p className="mt-1 text-sm text-gray-500">
              Заказ <span className="font-semibold text-gray-900">{state.orderNumber || state.return.orderId}</span>
            </p>
          </div>

          {/* Start Receiving CTA in Header */}
          {isApproved && (
            <PermissionGuard permission="warehouse.returns">
              <button
                type="button"
                onClick={handleStartReceiving}
                disabled={isStarting}
                className="inline-flex items-center px-5 py-2.5 border border-transparent shadow-sm text-sm font-semibold rounded-lg text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 transition-colors"
              >
                <Package className="h-5 w-5 mr-2" />
                {isStarting ? 'Запуск приёмки...' : 'Начать приёмку'}
              </button>
            </PermissionGuard>
          )}
        </div>

        {/* Notifications */}
        {error && (
          <div className="p-4 bg-red-50 text-red-700 rounded-lg flex items-center shadow-sm border border-red-200">
            <AlertCircle className="h-5 w-5 mr-2 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}
        {successMsg && (
          <div className="p-4 bg-green-50 text-green-800 rounded-lg flex items-center shadow-sm border border-green-200">
            <CheckCircle2 className="h-5 w-5 mr-2 flex-shrink-0" />
            <span>{successMsg}</span>
          </div>
        )}
        {infoMsg && (
          <div className="p-4 bg-blue-50 text-blue-800 rounded-lg flex items-center shadow-sm border border-blue-200">
            <Info className="h-5 w-5 mr-2 flex-shrink-0" />
            <span>{infoMsg}</span>
          </div>
        )}

        {/* Summary Meta Cards */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div className="bg-white p-4 rounded-xl shadow-sm border border-gray-200">
            <span className="text-xs font-medium text-gray-500 uppercase">Всего запрошено</span>
            <p className="mt-1 text-2xl font-bold text-gray-900">{state.totalRequested} шт.</p>
          </div>
          <div className="bg-white p-4 rounded-xl shadow-sm border border-gray-200">
            <span className="text-xs font-medium text-gray-500 uppercase">Отсканировано / Принято</span>
            <p className="mt-1 text-2xl font-bold text-indigo-600">{state.totalScanned} шт.</p>
          </div>
          <div className="bg-white p-4 rounded-xl shadow-sm border border-gray-200">
            <span className="text-xs font-medium text-gray-500 uppercase">Сериализованные (ZMU)</span>
            <p className="mt-1 text-sm font-semibold text-gray-800">
              {state.serializedScanned} / {state.serializedRequested}
            </p>
            <span className="text-xs text-gray-400">Обычные (Legacy): {state.legacyRequested}</span>
          </div>
          <div className="bg-white p-4 rounded-xl shadow-sm border border-gray-200">
            <span className="text-xs font-medium text-gray-500 uppercase">Начало приёмки</span>
            <p className="mt-1 text-sm font-semibold text-gray-800 flex items-center">
              <Clock className="h-4 w-4 mr-1 text-gray-400" />
              {formatTimestamp(state.return.receivingStartedAt)}
            </p>
          </div>
        </div>

        {/* Reason / Customer Claim Box */}
        <div className="bg-white p-5 rounded-xl shadow-sm border border-gray-200 text-sm space-y-3">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 border-b border-gray-100 pb-3">
            <div>
              <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block">Причина возврата</span>
              <span className="font-semibold text-gray-900 text-base">{getReturnReasonLabel(state.return.reason)}</span>
            </div>
            {state.return.adminComment && (
              <div className="text-xs text-gray-600 bg-gray-50 px-3 py-1.5 rounded-lg border border-gray-200">
                <span className="font-medium text-gray-700">Решение модератора: </span>
                {state.return.adminComment}
              </div>
            )}
          </div>
          {canViewCustomerComment && (
            state.return.comment ? (
              <div>
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wider block mb-1">Комментарий покупателя</span>
                <div className="p-3.5 bg-gray-50 rounded-lg border border-gray-200 text-sm text-gray-800 whitespace-pre-wrap leading-relaxed">
                  {state.return.comment}
                </div>
              </div>
            ) : (
              <p className="text-xs text-gray-400 italic">Комментарий покупателя не указан</p>
            )
          )}
        </div>

        {/* Global Serialized Scanner Input (Active Receiving) */}
        {isReceiving && (
          <div className="bg-gradient-to-r from-indigo-50 to-blue-50 p-6 rounded-xl border border-indigo-200 shadow-sm">
            <div className="max-w-2xl mx-auto text-center space-y-3">
              <div className="inline-flex items-center justify-center p-3 bg-indigo-600 text-white rounded-full shadow-md">
                <Barcode className="h-6 w-6" />
              </div>
              <h2 className="text-lg font-bold text-gray-900">Сканирование единиц товара (ZMU)</h2>
              <p className="text-xs text-gray-600">
                Отсканируйте штрихкод ZMU физической единицы или введите код вручную и нажмите Enter.
              </p>

              <form onSubmit={handleScanSubmit} className="flex gap-2 mt-4">
                <div className="relative flex-1">
                  <input
                    ref={scannerInputRef}
                    type="text"
                    value={barcodeInput}
                    onChange={(e) => setBarcodeInput(e.target.value)}
                    placeholder="Например: ZMU-XUJBQQ5ADSW4BWTX"
                    disabled={isScanning || !canReceive}
                    className="block w-full pl-4 pr-10 py-3 text-base rounded-lg border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 font-mono"
                    autoFocus={canReceive}
                  />
                  <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                    <Barcode className="h-5 w-5 text-gray-400" />
                  </div>
                </div>
                <button
                  type="submit"
                  disabled={isScanning || !barcodeInput.trim() || !canReceive}
                  className="px-5 py-3 border border-transparent text-sm font-semibold rounded-lg shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 flex items-center transition-colors"
                >
                  {isScanning ? 'Поиск...' : 'Сканировать'}
                </button>
              </form>
              {!canReceive && (
                <div className="mt-3 text-sm text-amber-600 font-medium bg-amber-50 py-2 px-3 rounded-lg border border-amber-200">
                  Режим только для чтения: у вас нет прав на физическое сканирование на складе.
                </div>
              )}
            </div>
          </div>
        )}

        {/* Completed Read-only Banner */}
        {isCompleted && (
          <div className="bg-green-50 border border-green-200 rounded-xl p-5 shadow-sm flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="p-2 bg-green-600 text-white rounded-full">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <div>
                <h3 className="text-base font-bold text-green-900">Приёмка завершена</h3>
                <p className="text-xs text-green-700">
                  Складская обработка возврата завершена. Редактирование приёмки заблокировано.
                </p>
              </div>
            </div>
            <span className="text-xs font-semibold text-green-800 bg-green-200 px-3 py-1.5 rounded-md">
              Приёмка завершена
            </span>
          </div>
        )}

        {/* Return Items List */}
        <div className="space-y-6">
          <h2 className="text-lg font-bold text-gray-900 flex items-center">
            <Layers className="h-5 w-5 mr-2 text-indigo-600" />
            Позиции к приёмке ({state.items.length})
          </h2>

          {state.items.map((item, itemIdx) => {
            const isSerialized = item.allocationMode === 'serialized';
            const legDraft = legacyDrafts[item.returnItem.id] || { accepted: 0, damaged: 0, rejected: 0 };
            const legSum = Number(legDraft.accepted) + Number(legDraft.damaged) + Number(legDraft.rejected);
            const legNotReceived = Math.max(0, item.requestedQuantity - legSum);

            return (
              <div
                key={item.returnItem.id || itemIdx}
                className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden"
              >
                {/* Item Card Header */}
                <div className="bg-gray-50 px-6 py-4 border-b border-gray-200 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                  <div className="flex items-start space-x-4">
                    <div className="w-14 h-14 rounded-lg bg-gray-100 border border-gray-200 overflow-hidden flex-shrink-0 flex items-center justify-center">
                      {item.productImageUrl ? (
                        <img
                          src={item.productImageUrl}
                          alt={item.productTitle || ''}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <Package className="h-6 w-6 text-gray-400" />
                      )}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="font-semibold text-gray-900 text-base">
                          {item.productTitle || item.returnItem.productTitle || 'Товар'}
                        </h3>
                        <span
                          className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                            isSerialized ? 'bg-indigo-100 text-indigo-800' : 'bg-gray-100 text-gray-800'
                          }`}
                        >
                          {isSerialized ? 'Сериализованный (ZMU)' : 'Обычный (Legacy)'}
                        </span>
                      </div>
                      <div className="text-xs text-gray-500 mt-1 flex flex-wrap gap-x-4 gap-y-1">
                        {(item.variantSize || item.returnItem.variantSize) && (
                          <span>Размер: <span className="font-medium text-gray-800">{item.variantSize || item.returnItem.variantSize}</span></span>
                        )}
                        {(item.variantColor || item.returnItem.variantColor) && (
                          <span>Цвет: <span className="font-medium text-gray-800">{item.variantColor || item.returnItem.variantColor}</span></span>
                        )}
                        {(item.sku || item.returnItem.sku) && (
                          <span>Артикул: <span className="font-mono text-gray-800">{item.sku || item.returnItem.sku}</span></span>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Quantity Stats Badges */}
                  <div className="flex items-center space-x-2 text-xs flex-shrink-0">
                    <span className="bg-white border border-gray-200 px-2.5 py-1 rounded-md text-gray-700">
                      Запрошено: <strong className="text-gray-900">{item.requestedQuantity}</strong>
                    </span>
                    <span className="bg-white border border-gray-200 px-2.5 py-1 rounded-md text-indigo-700">
                      Отсканировано: <strong>{item.scannedQuantity}</strong>
                    </span>
                    <span className="bg-white border border-gray-200 px-2.5 py-1 rounded-md text-amber-700">
                      Не получено: <strong>{item.notReceivedQuantity ?? Math.max(0, item.requestedQuantity - item.scannedQuantity)}</strong>
                    </span>
                  </div>
                </div>

                {/* Item Card Body */}
                <div className="p-6 space-y-6">
                  {/* SERIALIZED SECTION */}
                  {isSerialized && (
                    <div className="space-y-4">
                      {/* Scanned Units List */}
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 mb-3 flex items-center">
                          <Barcode className="h-4 w-4 mr-1.5 text-gray-500" />
                          Отсканированные единицы ({item.scannedUnits.length} из {item.requestedQuantity})
                        </h4>

                        {item.scannedUnits.length === 0 ? (
                          <div className="text-center py-6 bg-gray-50 rounded-lg border border-dashed border-gray-200 text-xs text-gray-500">
                            Единицы ещё не отсканированы. Используйте поле сканера выше для сканирования ZMU.
                          </div>
                        ) : (
                          <div className="divide-y divide-gray-100 border border-gray-200 rounded-lg overflow-hidden">
                            {item.scannedUnits.map((unit) => {
                              const draft = unitDrafts[unit.id] || {
                                disposition: (unit.disposition as 'restock' | 'damaged' | 'reject') || '',
                                inspectedCondition: unit.inspectedCondition || '',
                              };
                              const hasDisposition = Boolean(unit.disposition);

                              return (
                                <div
                                  key={unit.id}
                                  className={`p-4 flex flex-col md:flex-row md:items-center md:justify-between gap-3 ${
                                    !hasDisposition ? 'bg-amber-50/50' : 'bg-white'
                                  }`}
                                >
                                  {/* Unit Details */}
                                  <div className="space-y-1">
                                    <div className="flex items-center space-x-2">
                                      <span className="font-mono font-bold text-sm text-gray-900">
                                        {unit.unitCode}
                                      </span>
                                      {!hasDisposition && (
                                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-amber-200 text-amber-900">
                                          Требуется решение
                                        </span>
                                      )}
                                    </div>
                                    <p className="text-xs text-gray-500">
                                      Отсканирован: {formatTimestamp(unit.scannedAt || unit.createdAt)}
                                    </p>
                                  </div>

                                  {/* Inspection / Disposition Controls */}
                                  <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3">
                                    {isReceiving && canReceive ? (
                                      <>
                                        {/* 3 Canonical Disposition Buttons */}
                                        <div className="inline-flex rounded-md shadow-sm" role="group">
                                          <button
                                            type="button"
                                            onClick={() => handleUpdateUnitDisposition(unit.id, 'restock')}
                                            className={`px-3 py-1.5 text-xs font-semibold rounded-l-md border ${
                                              draft.disposition === 'restock'
                                                ? 'bg-green-600 text-white border-green-600 shadow-inner'
                                                : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
                                            }`}
                                          >
                                            Вернуть в продажу
                                          </button>
                                          <button
                                            type="button"
                                            onClick={() => handleUpdateUnitDisposition(unit.id, 'damaged')}
                                            className={`px-3 py-1.5 text-xs font-semibold border-t border-b ${
                                              draft.disposition === 'damaged'
                                                ? 'bg-amber-600 text-white border-amber-600 shadow-inner'
                                                : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
                                            }`}
                                          >
                                            Повреждён
                                          </button>
                                          <button
                                            type="button"
                                            onClick={() => handleUpdateUnitDisposition(unit.id, 'reject')}
                                            className={`px-3 py-1.5 text-xs font-semibold rounded-r-md border ${
                                              draft.disposition === 'reject'
                                                ? 'bg-red-600 text-white border-red-600 shadow-inner'
                                                : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
                                            }`}
                                          >
                                            Отклонить возврат
                                          </button>
                                        </div>

                                        {/* Condition Note Input */}
                                        <div className="flex items-center space-x-1">
                                          <input
                                            type="text"
                                            placeholder="Примечание к состоянию"
                                            value={draft.inspectedCondition}
                                            onChange={(e) =>
                                              setUnitDrafts((prev) => ({
                                                ...prev,
                                                [unit.id]: {
                                                  ...draft,
                                                  inspectedCondition: e.target.value,
                                                },
                                              }))
                                            }
                                            onBlur={() => handleSaveUnitCondition(unit.id)}
                                            className="text-xs rounded border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 py-1.5 px-2.5 w-44"
                                          />
                                        </div>
                                      </>
                                    ) : (
                                      /* Read-Only Inspection Display */
                                      <div className="flex items-center space-x-3 text-xs">
                                        <span className="font-semibold text-gray-700">Решение:</span>
                                        {unit.disposition === 'restock' && (
                                          <span className="inline-flex items-center px-2.5 py-1 rounded font-semibold bg-green-100 text-green-800">
                                            Вернуть в продажу (На склад)
                                          </span>
                                        )}
                                        {unit.disposition === 'damaged' && (
                                          <span className="inline-flex items-center px-2.5 py-1 rounded font-semibold bg-amber-100 text-amber-800">
                                            Повреждён (Брак)
                                          </span>
                                        )}
                                        {unit.disposition === 'reject' && (
                                          <span className="inline-flex items-center px-2.5 py-1 rounded font-semibold bg-red-100 text-red-800">
                                            Отклонён
                                          </span>
                                        )}
                                        {!unit.disposition && (
                                          <span className="text-gray-400">Не указано</span>
                                        )}
                                        {unit.inspectedCondition && (
                                          <span className="text-gray-500 italic">
                                            «{unit.inspectedCondition}»
                                          </span>
                                        )}
                                      </div>
                                    )}
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>

                      {/* Outbound Shipped Allocations Reference */}
                      {item.outboundAllocations.length > 0 && (
                        <div className="pt-2">
                          <details className="text-xs text-gray-500">
                            <summary className="cursor-pointer font-medium text-indigo-600 hover:text-indigo-800">
                              Показать единицы, отправленные клиенту в заказе ({item.outboundAllocations.length})
                            </summary>
                            <div className="mt-2 flex flex-wrap gap-2">
                              {item.outboundAllocations.map((alloc) => (
                                <span
                                  key={alloc.unitCode || alloc.allocationId || alloc.id}
                                  className="inline-flex items-center px-2.5 py-1 rounded bg-gray-100 text-gray-800 font-mono text-xs border border-gray-200"
                                >
                                  {alloc.unitCode} ({alloc.unitStatus || 'shipped'})
                                </span>
                              ))}
                            </div>
                          </details>
                        </div>
                      )}
                    </div>
                  )}

                  {/* LEGACY SECTION */}
                  {!isSerialized && (
                    <div className="space-y-4">
                      {isReceiving && canReceive ? (
                        <div className="bg-gray-50 p-4 rounded-lg border border-gray-200 space-y-3">
                          <h4 className="text-sm font-semibold text-gray-900">
                            Количественная приёмка (Legacy)
                          </h4>

                          {legDraft.error && (
                            <div className="text-xs text-red-600 font-medium">{legDraft.error}</div>
                          )}

                          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                            <div>
                              <label className="block text-xs font-medium text-gray-700 mb-1">
                                Принято в продажу
                              </label>
                              <input
                                type="number"
                                min="0"
                                max={item.requestedQuantity}
                                value={legDraft.accepted}
                                onChange={(e) =>
                                  setLegacyDrafts((prev) => ({
                                    ...prev,
                                    [item.returnItem.id]: {
                                      ...legDraft,
                                      accepted: Math.max(0, parseInt(e.target.value, 10) || 0),
                                    },
                                  }))
                                }
                                className="block w-full text-sm rounded border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 py-1.5 px-2.5"
                              />
                            </div>
                            <div>
                              <label className="block text-xs font-medium text-gray-700 mb-1">
                                Повреждено
                              </label>
                              <input
                                type="number"
                                min="0"
                                max={item.requestedQuantity}
                                value={legDraft.damaged}
                                onChange={(e) =>
                                  setLegacyDrafts((prev) => ({
                                    ...prev,
                                    [item.returnItem.id]: {
                                      ...legDraft,
                                      damaged: Math.max(0, parseInt(e.target.value, 10) || 0),
                                    },
                                  }))
                                }
                                className="block w-full text-sm rounded border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 py-1.5 px-2.5"
                              />
                            </div>
                            <div>
                              <label className="block text-xs font-medium text-gray-700 mb-1">
                                Отклонено
                              </label>
                              <input
                                type="number"
                                min="0"
                                max={item.requestedQuantity}
                                value={legDraft.rejected}
                                onChange={(e) =>
                                  setLegacyDrafts((prev) => ({
                                    ...prev,
                                    [item.returnItem.id]: {
                                      ...legDraft,
                                      rejected: Math.max(0, parseInt(e.target.value, 10) || 0),
                                    },
                                  }))
                                }
                                className="block w-full text-sm rounded border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 py-1.5 px-2.5"
                              />
                            </div>
                            <div>
                              <label className="block text-xs font-medium text-gray-500 mb-1">
                                Не получено (расчёт)
                              </label>
                              <div className="bg-white border border-gray-200 rounded text-sm py-1.5 px-2.5 font-bold text-gray-800">
                                {legNotReceived}
                              </div>
                            </div>
                          </div>

                          <div className="flex justify-end pt-2">
                            <button
                              type="button"
                              onClick={() => handleSaveLegacyItem(item.returnItem.id, item.requestedQuantity)}
                              disabled={legDraft.isSaving}
                              className="px-3 py-1.5 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold rounded shadow-sm disabled:opacity-50"
                            >
                              {legDraft.isSaving ? 'Сохранение...' : 'Сохранить количества'}
                            </button>
                          </div>
                        </div>
                      ) : (
                        /* Read-Only Legacy Display */
                        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-gray-50 p-3 rounded-lg border border-gray-200 text-xs">
                          <div>
                            <span className="text-gray-500">Принято в продажу:</span>
                            <p className="font-bold text-green-700 text-sm">{item.acceptedQuantity} шт.</p>
                          </div>
                          <div>
                            <span className="text-gray-500">Повреждено:</span>
                            <p className="font-bold text-amber-700 text-sm">{item.damagedQuantity} шт.</p>
                          </div>
                          <div>
                            <span className="text-gray-500">Отклонено:</span>
                            <p className="font-bold text-red-700 text-sm">{item.rejectedQuantity} шт.</p>
                          </div>
                          <div>
                            <span className="text-gray-500">Не получено:</span>
                            <p className="font-bold text-gray-700 text-sm">{item.notReceivedQuantity ?? Math.max(0, item.requestedQuantity - (item.acceptedQuantity + item.damagedQuantity + item.rejectedQuantity))} шт.</p>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        {/* Finalize Receiving Action Bar (Active Receiving) */}
        {isReceiving && (
          <div
            className={`sticky bottom-4 border rounded-xl p-4 shadow-lg flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 z-10 ${
              isZeroScanned
                ? 'bg-amber-50/70 border-amber-300'
                : isPartialScanned
                ? 'bg-amber-50/50 border-amber-300'
                : 'bg-white border-gray-300'
            }`}
          >
            <div>
              <h3 className="text-sm font-bold text-gray-900">
                {!state.canFinalize
                  ? 'Ожидается решение по отсканированным единицам'
                  : isZeroScanned
                  ? 'Товары ещё не отсканированы'
                  : isPartialScanned
                  ? `Приёмка с расхождением (${totalScannedUnits} из ${state.totalRequested} шт.)`
                  : `Все единицы проверены (${state.totalRequested} из ${state.totalRequested} шт.)`}
              </h3>
              <p className="text-xs text-gray-500">
                {!state.canFinalize
                  ? 'Для завершения укажите решение (диспозицию) для всех отсканированных единиц и сохраните количества.'
                  : isZeroScanned
                  ? 'Отсканируйте ZMU-код единицы для приёмки. Если посылка пустая или товар утерян, можно зафиксировать недовоз.'
                  : isPartialScanned
                  ? `Отсканировано и проверено: ${totalScannedUnits} шт. Не получено: ${totalNotReceived} шт. Неполученные единицы не оприходуются и возврат средств за них блокируется.`
                  : 'Все запрашиваемые единицы отсканированы и проверены. Вы можете завершить приёмку.'}
              </p>
            </div>

            <PermissionGuard permission="warehouse.returns">
              {!state.canFinalize ? (
                <button
                  type="button"
                  disabled={true}
                  className="inline-flex items-center justify-center px-6 py-3 border border-transparent text-sm font-bold rounded-lg shadow-sm text-white bg-gray-300 cursor-not-allowed transition-colors"
                >
                  <CheckCircle2 className="h-5 w-5 mr-2" />
                  Завершить приёмку
                </button>
              ) : isZeroScanned ? (
                <button
                  type="button"
                  onClick={() => setIsFinalizeModalOpen(true)}
                  disabled={isFinalizing}
                  className="inline-flex items-center justify-center px-5 py-2.5 border border-amber-300 text-sm font-semibold rounded-lg shadow-sm text-amber-900 bg-amber-100 hover:bg-amber-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  <AlertTriangle className="h-4 w-4 mr-2 text-amber-700" />
                  Завершить без товара (0 из {state.totalRequested})
                </button>
              ) : isPartialScanned ? (
                <button
                  type="button"
                  onClick={() => setIsFinalizeModalOpen(true)}
                  disabled={isFinalizing}
                  className="inline-flex items-center justify-center px-5 py-2.5 border border-amber-500 text-sm font-bold rounded-lg shadow-sm text-white bg-amber-600 hover:bg-amber-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
                >
                  <AlertTriangle className="h-4 w-4 mr-2" />
                  Завершить с расхождением ({totalScannedUnits} из {state.totalRequested})
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => setIsFinalizeModalOpen(true)}
                  disabled={isFinalizing}
                  className="inline-flex items-center justify-center px-6 py-3 border border-transparent text-sm font-bold rounded-lg shadow-sm text-white bg-green-600 hover:bg-green-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
                >
                  <CheckCircle2 className="h-5 w-5 mr-2" />
                  Завершить приёмку
                </button>
              )}
            </PermissionGuard>
          </div>
        )}

        {/* Finalize Confirmation Modal */}
        {isFinalizeModalOpen && (
          <div className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
            <div className="bg-white rounded-xl max-w-lg w-full p-6 shadow-2xl space-y-4">
              <div className="flex items-center space-x-3 text-gray-900 border-b pb-3">
                <div
                  className={`p-2 rounded-full ${
                    isZeroScanned
                      ? 'bg-red-100 text-red-700'
                      : isPartialScanned
                      ? 'bg-amber-100 text-amber-700'
                      : 'bg-green-100 text-green-700'
                  }`}
                >
                  {isZeroScanned || isPartialScanned ? (
                    <AlertTriangle className="h-6 w-6" />
                  ) : (
                    <CheckCircle2 className="h-6 w-6" />
                  )}
                </div>
                <h3 className="text-lg font-bold">
                  {isZeroScanned
                    ? 'Подтверждение приёмки без товара (0 получено)'
                    : isPartialScanned
                    ? 'Подтверждение приёмки с расхождением'
                    : 'Подтверждение завершения приёмки'}
                </h3>
              </div>

              <p className="text-sm text-gray-600">
                {isZeroScanned ? (
                  <>
                    Вы подтверждаете завершение складской приёмки для заказа{' '}
                    <strong className="text-gray-900">{state.orderNumber || state.return.id}</strong> без фактически
                    полученного товара.
                  </>
                ) : (
                  <>
                    Вы собираетесь окончательно завершить складскую приёмку для заказа{' '}
                    <strong className="text-gray-900">{state.orderNumber || state.return.id}</strong>.
                  </>
                )}
              </p>

              {/* Summary Breakdown */}
              <div className="bg-gray-50 p-3 rounded-lg border border-gray-200 text-xs space-y-1.5">
                <div className="flex justify-between">
                  <span className="text-gray-600">Вернуть на склад (в продажу):</span>
                  <strong className="text-green-700">{totalRestock} шт.</strong>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-600">Повреждено / списано:</span>
                  <strong className="text-amber-700">{totalDamaged} шт.</strong>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-600">Отклонено:</span>
                  <strong className="text-red-700">{totalReject} шт.</strong>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-600">Не получено:</span>
                  <strong className="text-gray-700">{totalNotReceived} шт.</strong>
                </div>
              </div>

              {isZeroScanned ? (
                <div className="p-3 bg-red-50 text-red-800 rounded-lg text-xs flex items-start space-x-2 border border-red-200">
                  <AlertTriangle className="h-4 w-4 text-red-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <strong className="block font-semibold mb-0.5">Внимание: ни один товар не был принят!</strong>
                    <span>
                      Все запрашиваемые единицы ({state.totalRequested} шт.) будут зафиксированы как не полученные на склад.
                      Товары не поступят на склад, а возврат денежных средств покупателю будет полностью заблокирован.
                      Используйте это действие только в случае утери отправления или пустой посылки.
                    </span>
                  </div>
                </div>
              ) : isPartialScanned ? (
                <div className="p-3 bg-amber-50 text-amber-800 rounded-lg text-xs flex items-start space-x-2 border border-amber-200">
                  <AlertTriangle className="h-4 w-4 text-amber-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <strong className="block font-semibold mb-0.5">Внимание: приёмка с неполным количеством!</strong>
                    <span>
                      Принято {totalScannedUnits} из {state.totalRequested} шт. Неполученные единицы ({totalNotReceived} шт.)
                      не будут оприходованы на склад. Возврат средств за неполученные товары производиться не будет.
                    </span>
                  </div>
                </div>
              ) : (
                <div className="p-3 bg-amber-50 text-amber-800 rounded-lg text-xs flex items-start space-x-2">
                  <AlertTriangle className="h-4 w-4 text-amber-600 flex-shrink-0 mt-0.5" />
                  <span>
                    Это действие необратимо. Статус возврата перейдёт в «Товар принят на складе». Физические остатки на
                    складе будут обновлены. Неполученные товары не оприходуются и не подлежат возврату средств. Возврат
                    средств покупателю выполняется отдельным шагом.
                  </span>
                </div>
              )}

              <div className="flex justify-end space-x-3 pt-3 border-t">
                <button
                  type="button"
                  onClick={() => setIsFinalizeModalOpen(false)}
                  disabled={isFinalizing}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg"
                >
                  Отмена
                </button>
                <button
                  type="button"
                  onClick={handleFinalize}
                  disabled={isFinalizing}
                  className={`px-4 py-2 text-sm font-bold text-white rounded-lg shadow-sm disabled:opacity-50 flex items-center ${
                    isZeroScanned
                      ? 'bg-red-600 hover:bg-red-700'
                      : isPartialScanned
                      ? 'bg-amber-600 hover:bg-amber-700'
                      : 'bg-green-600 hover:bg-green-700'
                  }`}
                >
                  {isFinalizing
                    ? 'Завершение...'
                    : isZeroScanned
                    ? 'Подтвердить отсутствие товара и завершить'
                    : isPartialScanned
                    ? 'Подтвердить расхождение и завершить'
                    : 'Подтвердить и завершить'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </PermissionGuard>
  );
}
