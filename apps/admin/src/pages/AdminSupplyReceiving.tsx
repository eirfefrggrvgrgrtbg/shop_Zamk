import React, { useState, useRef, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  Truck,
  Box,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  ArrowLeft,
  RefreshCw,
  AlertCircle,
  RotateCcw,
  Tag,
  ShieldCheck,
  Building2,
  Calendar,
  Layers,
  Search,
} from 'lucide-react';
import {
  lookupSupplyByCode,
  markSupplyArrived,
  startSupplyReceivingSession,
  recordSupplyReceivingScan,
  recordSerializedReceivingScan,
  getSerializedReceivingScans,
  undoSerializedReceivingScan,
  finalizeSupplyReceivingSession,
  getSupplyReceivingQueue,
} from '@zamk/api-client/src/admin';
import type {
  SellerSupply,
  SupplyReceivingSession,
  SupplyReceivingQueueItem,
  SerializedRecentScan,
  SerializedScanResponse,
} from '@zamk/api-client/src/types';

import { playBeepSound } from '../utils/audio';
import { normalizeScannerCode } from '../utils/scanner';
import { useAdminAuth } from '../contexts/AdminAuthContext';

function mapReceivingError(err: any): string {
  const code = err?.error?.code || err?.code || '';
  const message = err?.error?.message || err?.message || '';

  switch (code) {
    case 'supply_not_found':
      return 'Поставка или грузоместо не найдено.';
    case 'supply_not_arrived':
      return 'Поставка ещё не прибыла на склад.';
    case 'supply_not_ready_for_receiving':
    case 'supply_invalid_status':
      return 'Поставка ещё не готова к приёмке.';
    case 'supply_already_completed':
      return 'Приёмка по этой поставке уже завершена.';
    case 'supply_cancelled':
      return 'Поставка отменена.';
    case 'no_expected_units_remain':
      return 'Все ожидаемые товарные единицы по этой поставке уже приняты.';
    case 'receiving_session_already_active':
      return 'Для этой поставки уже открыта приёмка.';
    case 'invalid_receiving_code':
      return 'Введите номер поставки, грузоместа или отсканируйте QR-код.';
    case 'unit_already_scanned':
      return 'Эта единица уже отсканирована в текущей сессии.';
    case 'unit_already_received':
      return 'Эта единица уже принята или забракована.';
    case 'unit_not_found':
      return 'Этикетка ZAMK не найдена.';
    case 'unit_not_in_supply':
      return 'Эта единица относится к другой поставке.';
    case 'serialized_unit_code_required':
      return 'Для этой поставки сканируйте уникальную этикетку ZMU.';
    case 'supply_unit_identity_mismatch':
      return 'Идентификаторы товарных единиц не совпадают с составом поставки.';
    case 'scan_not_found':
      return 'Скан не найден.';
    case 'scan_already_voided':
      return 'Этот скан уже был отменён.';
    case 'scan_not_in_session':
      return 'Скан не принадлежит этой сессии.';
    case 'receiving_session_finalized':
      return 'Сессия приёмки уже завершена.';
    case 'invalid_receiving_condition':
      return 'Недопустимое состояние товара (допустимо: ok или damaged).';
    case 'supply_not_serialized':
      return 'Эта поставка использует старую схему приёмки по ZMK.';
    default:
      if (message && !message.startsWith('HTTP Error')) {
        return message;
      }
      return 'Поставка или грузоместо не найдено.';
  }
}

function getStatusBadge(status: string) {
  switch (status) {
    case 'shipped_by_seller':
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
          <Truck className="w-3.5 h-3.5 mr-1" />
          Поставка в пути
        </span>
      );
    case 'arrived_at_zamk':
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
          Прибыла на склад
        </span>
      );
    case 'receiving':
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
          <ShieldCheck className="w-3.5 h-3.5 mr-1" />
          В процессе приёмки
        </span>
      );
    case 'completed_with_discrepancies':
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
          <AlertTriangle className="w-3.5 h-3.5 mr-1" />
          Приёмка завершена с расхождениями
        </span>
      );
    case 'completed':
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
          Приёмка завершена
        </span>
      );
    case 'cancelled':
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
          <AlertTriangle className="w-3.5 h-3.5 mr-1" />
          Отменена
        </span>
      );
    default:
      return (
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-slate-500/10 text-slate-400 border border-slate-500/20">
          <AlertCircle className="w-3.5 h-3.5 mr-1" />
          Не отправлена продавцом
        </span>
      );
  }
}

function formatOperationalTime(dateStr?: string): string {
  if (!dateStr) return '';
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '';
    const now = new Date();
    const isToday =
      d.getDate() === now.getDate() &&
      d.getMonth() === now.getMonth() &&
      d.getFullYear() === now.getFullYear();

    const timeStr = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
    if (isToday) {
      return `сегодня, ${timeStr}`;
    }
    const datePart = d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
    return `${datePart}, ${timeStr}`;
  } catch {
    return '';
  }
}

function formatCargoPlaces(count: number): string {
  const n = count || 0;
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 19) {
    return `${n} грузовых мест`;
  }
  if (mod10 === 1) {
    return `${n} грузовое место`;
  }
  if (mod10 >= 2 && mod10 <= 4) {
    return `${n} грузовых места`;
  }
  return `${n} грузовых мест`;
}

export function AdminSupplyReceiving() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { hasPermission } = useAdminAuth();
  const canReceive = hasPermission('inventory.receipt');
  const [dossier, setDossier] = useState<SellerSupply | null>(null);
  const [session, setSession] = useState<SupplyReceivingSession | null>(null);
  const [qrInput, setQrInput] = useState('');
  const [barcodeInput, setBarcodeInput] = useState('');
  const [isDamagedScan, setIsDamagedScan] = useState(false);

  const [recentScans, setRecentScans] = useState<SerializedRecentScan[]>([]);
  const [lastScannedItem, setLastScannedItem] = useState<SerializedScanResponse | null>(null);

  const [lookupLoading, setLookupLoading] = useState(false);
  const [arrivalLoading, setArrivalLoading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [undoLoading, setUndoLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const [queue, setQueue] = useState<SupplyReceivingQueueItem[]>([]);
  const [queueLoading, setQueueLoading] = useState(false);
  const [queueError, setQueueError] = useState<string | null>(null);
  const [openingSupplyId, setOpeningSupplyId] = useState<string | null>(null);

  const [isFinalized, setIsFinalized] = useState(false);
  const [showFinalizeModal, setShowFinalizeModal] = useState(false);

  const qrRef = useRef<HTMLInputElement>(null);
  const barcodeRef = useRef<HTMLInputElement>(null);

  const isSerialized = session?.receivingMode === 'serialized';

  // Derivations from dossier
  const remainingExpected = dossier?.items?.reduce((acc, i) => acc + (i.missingQuantity ?? 0), 0) ?? 0;
  const isAdditionalDossier = Boolean(
    dossier && (
      dossier.status === 'completed_with_discrepancies' ||
      (dossier.items && dossier.items.some((i) => (i.acceptedQuantity || 0) > 0 || (i.damagedQuantity || 0) > 0))
    )
  );
  const isAdditionalSession = Boolean(session && isAdditionalDossier);

  const fetchQueue = async () => {
    try {
      setQueueLoading(true);
      setQueueError(null);
      const items = await getSupplyReceivingQueue();
      setQueue(items || []);
    } catch (err: any) {
      setQueueError(err?.message || 'Не удалось загрузить очередь приёмки');
    } finally {
      setQueueLoading(false);
    }
  };

  useEffect(() => {
    fetchQueue();
  }, []);

  useEffect(() => {
    const qrParam = searchParams.get('qr');
    if (qrParam && !dossier && !session && !isFinalized) {
      setLookupLoading(true);
      lookupSupplyByCode(qrParam.trim())
        .then((data) => {
          setDossier(data);
          playBeepSound('success');
        })
        .catch((err) => {
          setError(mapReceivingError(err));
          playBeepSound('error');
        })
        .finally(() => {
          setLookupLoading(false);
        });
    }
  }, [searchParams]);

  useEffect(() => {
    if (!session && !dossier && !isFinalized) {
      qrRef.current?.focus();
    } else if (session && !isFinalized) {
      barcodeRef.current?.focus();
    }
  }, [session, dossier, isFinalized]);

  const handleSelectSupplyFromQueue = async (item: SupplyReceivingQueueItem) => {
    try {
      setOpeningSupplyId(item.supplyId);
      setError(null);
      setSuccessMessage(null);

      const lookupCode = item.supplyNumber || item.supplyId;
      const fullDossier = await lookupSupplyByCode(lookupCode);
      setDossier(fullDossier);

      if (canReceive) {
        const sess = await startSupplyReceivingSession(lookupCode);
        setSession(sess);
        if (sess.receivingMode === 'serialized') {
          await loadRecentScans(sess.id);
        }
      }
      playBeepSound('success');
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setOpeningSupplyId(null);
    }
  };

  const loadRecentScans = async (sessionId: string) => {
    try {
      const scans = await getSerializedReceivingScans(sessionId, 10);
      setRecentScans(scans || []);
    } catch (_) {
      // Ignored
    }
  };

  const handleLookup = async (e: React.FormEvent) => {
    e.preventDefault();
    const input = normalizeScannerCode(qrInput);
    if (!input) return;

    try {
      setLookupLoading(true);
      setError(null);
      setSuccessMessage(null);
      const data = await lookupSupplyByCode(input);
      setDossier(data);
      playBeepSound('success');
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setLookupLoading(false);
      setQrInput('');
    }
  };

  const handleMarkArrived = async () => {
    if (!dossier) return;
    try {
      setArrivalLoading(true);
      setError(null);
      setSuccessMessage(null);
      await markSupplyArrived(dossier.id);
      setDossier((prev) => (prev ? { ...prev, status: 'arrived_at_zamk', arrivedAt: new Date().toISOString() } : null));
      setSuccessMessage('Поставка отмечена как прибывшая на склад ZAMK.');
      playBeepSound('success');
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setArrivalLoading(false);
    }
  };

  const handleStartOrResumeSession = async () => {
    if (!dossier) return;
    const lookupCode = dossier.qrToken || dossier.supplyNumber || dossier.id;
    if (!lookupCode) return;

    try {
      setLoading(true);
      setError(null);
      setSuccessMessage(null);

      const data = await startSupplyReceivingSession(lookupCode);

      // Successfully obtained canonical session from backend
      setSession(data);
      setIsFinalized(false);
      setShowFinalizeModal(false);
      setLastScannedItem(null);
      setRecentScans([]);

      if (data.receivingMode === 'serialized') {
        await loadRecentScans(data.id);
      }
      playBeepSound('success');
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setLoading(false);
    }
  };

  const resetFlow = () => {
    setSession(null);
    setDossier(null);
    setRecentScans([]);
    setLastScannedItem(null);
    setIsFinalized(false);
    setShowFinalizeModal(false);
    setError(null);
    setSuccessMessage(null);
    setQrInput('');
    setBarcodeInput('');
    setIsDamagedScan(false);
    setSearchParams({});
    fetchQueue();
    setTimeout(() => qrRef.current?.focus(), 50);
  };

  const handleScanItem = async (e: React.FormEvent) => {
    e.preventDefault();
    const rawInput = normalizeScannerCode(barcodeInput);
    if (!rawInput || !session || !session.items) return;

    try {
      setLoading(true);
      setError(null);

      if (isSerialized) {
        const resp = await recordSerializedReceivingScan(session.id, {
          unitCode: rawInput,
          condition: isDamagedScan ? 'damaged' : 'ok',
        });
        setLastScannedItem(resp);

        // Reset damage flag only after successful scan
        if (isDamagedScan) {
          setIsDamagedScan(false);
        }

        // Update items locally
        setSession((prev) => {
          if (!prev || !prev.items) return prev;
          const newItems = prev.items.map((i) => {
            if (i.variantId === resp.productVariantId || (resp.variantBarcode && i.barcode === resp.variantBarcode)) {
              return {
                ...i,
                scannedQuantity: resp.condition === 'ok' ? i.scannedQuantity + 1 : i.scannedQuantity,
                damagedQuantity: resp.condition === 'damaged' ? i.damagedQuantity + 1 : i.damagedQuantity,
              };
            }
            return i;
          });
          return { ...prev, items: newItems };
        });

        await loadRecentScans(session.id);
        playBeepSound('success');
      } else {
        // Legacy aggregate scan
        const matchedItem = session.items.find(
          (i) => (i.barcode && i.barcode === rawInput) || i.sku === rawInput
        );
        if (!matchedItem || !matchedItem.variantId) {
          throw new Error('Штрихкод не найден в данной поставке');
        }

        await recordSupplyReceivingScan(session.id, {
          variantId: matchedItem.variantId,
          quantity: 1,
          isDamage: isDamagedScan,
        });

        if (isDamagedScan) {
          setIsDamagedScan(false);
        }

        setSession((prev) => {
          if (!prev || !prev.items) return prev;
          const newItems = prev.items.map((i) => {
            if (i.id === matchedItem.id) {
              return {
                ...i,
                scannedQuantity: isDamagedScan ? i.scannedQuantity : i.scannedQuantity + 1,
                damagedQuantity: isDamagedScan ? i.damagedQuantity + 1 : i.damagedQuantity,
              };
            }
            return i;
          });
          return { ...prev, items: newItems };
        });

        playBeepSound('success');
      }
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setLoading(false);
      setBarcodeInput('');
      setTimeout(() => barcodeRef.current?.focus(), 50);
    }
  };

  const handleUndoLastScan = async () => {
    if (!session || !isSerialized) return;
    const latestNonVoided = recentScans.find((s) => !s.voidedAt);
    if (!latestNonVoided) return;

    try {
      setUndoLoading(true);
      setError(null);

      await undoSerializedReceivingScan(session.id, latestNonVoided.scanId);

      const isDmg = latestNonVoided.condition === 'damaged';
      setSession((prev) => {
        if (!prev || !prev.items) return prev;
        const newItems = prev.items.map((i) => {
          if (
            (latestNonVoided.variantBarcode && i.barcode === latestNonVoided.variantBarcode) ||
            (latestNonVoided.sellerSku && (i.sku === latestNonVoided.sellerSku || i.barcode === latestNonVoided.sellerSku))
          ) {
            return {
              ...i,
              scannedQuantity: isDmg ? i.scannedQuantity : Math.max(0, i.scannedQuantity - 1),
              damagedQuantity: isDmg ? Math.max(0, i.damagedQuantity - 1) : i.damagedQuantity,
            };
          }
          return i;
        });
        return { ...prev, items: newItems };
      });

      if (lastScannedItem && lastScannedItem.scanId === latestNonVoided.scanId) {
        setLastScannedItem(null);
      }

      await loadRecentScans(session.id);
      playBeepSound('success');
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setUndoLoading(false);
      setTimeout(() => barcodeRef.current?.focus(), 50);
    }
  };

  const handleFinalizeConfirm = async () => {
    if (!session) return;
    try {
      setLoading(true);
      setError(null);

      await finalizeSupplyReceivingSession(session.id, {});

      setIsFinalized(true);
      setShowFinalizeModal(false);

      // Refetch canonical Supply state immediately without hard reload
      if (dossier) {
        const lookupCode = dossier.qrToken || dossier.supplyNumber || dossier.id;
        try {
          const updatedSupply = await lookupSupplyByCode(lookupCode);
          setDossier(updatedSupply);
        } catch (_) {
          // Ignore lookup failure
        }
      }
      playBeepSound('success');
    } catch (err: any) {
      setError(mapReceivingError(err));
      playBeepSound('error');
    } finally {
      setLoading(false);
    }
  };

  const totalExpected = session?.items?.reduce((acc, i) => acc + i.expectedQuantity, 0) || 0;
  const totalOk = session?.items?.reduce((acc, i) => acc + i.scannedQuantity, 0) || 0;
  const totalDamaged = session?.items?.reduce((acc, i) => acc + i.damagedQuantity, 0) || 0;
  const totalScanned = totalOk + totalDamaged;
  const totalRemaining = Math.max(0, totalExpected - totalScanned);
  const hasDiscrepancy = session?.items?.some((i) => i.expectedQuantity !== i.scannedQuantity + i.damagedQuantity);
  const latestNonVoidedScan = recentScans.find((s) => !s.voidedAt);
  const countArrived = queue.filter((q) => q.status === 'arrived_at_zamk').length;
  const countReceiving = queue.filter((q) => q.status === 'receiving').length;
  const totalRemainingInQueue = queue.reduce((sum, q) => sum + (q.remainingUnitsCount || 0), 0);
  const isSessionOrDossier = Boolean(dossier || session);

  return (
    <div className={`space-y-6 pb-20 ${isSessionOrDossier ? '' : 'max-w-5xl'}`}>
      <div className={isSessionOrDossier ? 'px-4 sm:px-6' : ''}>
        {isSessionOrDossier && !isFinalized && (
          <button
            type="button"
            onClick={resetFlow}
            className="inline-flex items-center text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors mb-3"
          >
            <ArrowLeft className="w-4 h-4 mr-1.5" />
            К очереди приёмки
          </button>
        )}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">
              {isAdditionalSession
                ? 'Доприёмка поставок (Additional Receiving)'
                : isSessionOrDossier
                ? 'Приемка поставок (Supplies)'
                : 'Приёмка поставок'}
            </h1>
            <p className="text-sm mt-1 text-gray-500">
              {isAdditionalSession
                ? 'Доприёмка оставшихся физических единиц по ZMU'
                : isSerialized
                ? 'Сериализованная приёмка физических единиц по ZMU'
                : isSessionOrDossier
                ? 'Сканирование QR поставок и штрихкодов товаров'
                : 'Поиск поставок и операционная очередь входящих поставок на склад ZAMK'}
            </p>
          </div>
        {session && (
          <div className="mt-3 sm:mt-0 flex items-center space-x-2">
            {isAdditionalSession ? (
              <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                <RotateCcw className="w-3.5 h-3.5 mr-1" />
                Доприёмка · ZMU
              </span>
            ) : isSerialized ? (
              <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                <ShieldCheck className="w-3.5 h-3.5 mr-1" />
                Сериализованная · ZMU
              </span>
            ) : (
              <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                <Tag className="w-3.5 h-3.5 mr-1" />
                Старая поставка · приёмка по ZMK
              </span>
            )}
          </div>
        )}
        </div>
      </div>

      {error && !isFinalized && (
        <div className={`p-4 rounded-xl flex items-center ${isSessionOrDossier ? 'mx-4 sm:mx-6 bg-rose-500/10 border border-rose-500/20 text-rose-200' : 'bg-rose-50 border border-rose-200 text-rose-800'}`}>
          <AlertTriangle className={`h-5 w-5 mr-3 flex-shrink-0 ${isSessionOrDossier ? 'text-rose-500' : 'text-rose-600'}`} />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {successMessage && !isFinalized && (
        <div className={`p-4 rounded-xl flex items-center ${isSessionOrDossier ? 'mx-4 sm:mx-6 bg-emerald-500/10 border border-emerald-500/20 text-emerald-200' : 'bg-emerald-50 border border-emerald-200 text-emerald-800'}`}>
          <CheckCircle2 className={`h-5 w-5 mr-3 flex-shrink-0 ${isSessionOrDossier ? 'text-emerald-500' : 'text-emerald-600'}`} />
          <span className="text-sm font-medium">{successMessage}</span>
        </div>
      )}

      {isFinalized && session ? (
        <div className="mx-4 sm:mx-6">
          <div className="bg-slate-800 border border-slate-700 rounded-xl p-8 max-w-4xl mx-auto shadow-2xl">
            <div className="text-center mb-8">
              <div className={`inline-flex items-center justify-center w-20 h-20 rounded-full ${
                dossier?.status === 'completed'
                  ? 'bg-emerald-500/10 text-emerald-500'
                  : 'bg-amber-500/10 text-amber-500'
              } mb-4`}>
                {dossier?.status === 'completed' ? (
                  <CheckCircle2 className="h-10 w-10 text-emerald-500" />
                ) : (
                  <AlertTriangle className="h-10 w-10 text-amber-500" />
                )}
              </div>
              <h2 className="text-3xl font-bold text-white mb-2">
                {dossier?.status === 'completed'
                  ? 'Приёмка завершена'
                  : 'Приёмка завершена с расхождениями'}
              </h2>
              <p className="text-slate-400">
                Сессия <span className="font-mono text-emerald-400">{session.id}</span> успешно закрыта.
              </p>
              {dossier?.status === 'completed_with_discrepancies' && remainingExpected > 0 && (
                <div className="mt-4 p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg max-w-md mx-auto text-amber-300 text-sm font-medium">
                  Осталось принять: <span className="font-bold text-white">{remainingExpected} шт.</span>
                </div>
              )}
            </div>

            <div className="bg-slate-900 border border-slate-700 rounded-xl overflow-hidden mb-8">
              <div className="px-6 py-4 border-b border-slate-700 bg-slate-800/50">
                <h3 className="font-medium text-white">Итоги сессии {hasDiscrepancy && '(в текущей сессии есть расхождения)'}</h3>
              </div>
              <div className="p-0 overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="border-b border-slate-700 bg-slate-900">
                      <th className="py-4 px-6 text-xs font-semibold text-slate-400 uppercase tracking-wider">SKU / Штрихкод</th>
                      <th className="py-4 px-6 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">Ожидалось</th>
                      <th className="py-4 px-6 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">Принято</th>
                      <th className="py-4 px-6 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">Брак</th>
                      <th className="py-4 px-6 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">Недостача</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {session.items?.map((item) => {
                      const missing = Math.max(0, item.expectedQuantity - item.scannedQuantity - item.damagedQuantity);
                      const hasRowDiscrepancy = missing > 0 || item.damagedQuantity > 0;
                      return (
                        <tr key={item.id} className={`hover:bg-slate-800/30 transition-colors ${hasRowDiscrepancy ? 'bg-rose-500/5' : ''}`}>
                          <td className="py-4 px-6 font-mono text-white text-sm">{item.barcode || item.sku}</td>
                          <td className="py-4 px-6 text-right font-medium text-slate-300">{item.expectedQuantity}</td>
                          <td className="py-4 px-6 text-right font-bold text-emerald-400">{item.scannedQuantity}</td>
                          <td className="py-4 px-6 text-right font-bold text-rose-400">{item.damagedQuantity}</td>
                          <td className="py-4 px-6 text-right font-bold text-orange-400">{missing}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="flex flex-col sm:flex-row gap-3">
              {dossier?.status === 'completed_with_discrepancies' && remainingExpected > 0 && (
                <button
                  onClick={handleStartOrResumeSession}
                  disabled={!canReceive || loading}
                  className="flex-1 bg-amber-600 hover:bg-amber-500 text-white font-medium py-3 px-4 rounded-lg flex items-center justify-center transition-colors shadow-lg"
                >
                  {loading ? <RefreshCw className="w-5 h-5 animate-spin mr-2" /> : <ArrowRight className="w-5 h-5 mr-2" />}
                  Доприёмка · осталось {remainingExpected}
                </button>
              )}
              <button
                onClick={resetFlow}
                className="flex-1 bg-slate-700 hover:bg-slate-600 text-white font-medium py-3 px-4 rounded-lg flex items-center justify-center transition-colors"
              >
                <RefreshCw className="w-5 h-5 mr-2" />
                Назад к приёмке поставок
              </button>
            </div>
          </div>
        </div>
      ) : !session ? (
        !dossier ? (
          <div className="space-y-6">
            {/* Search Toolbar */}
            <div className="bg-white border border-gray-200 rounded-xl p-4 sm:p-5 shadow-2xs">
              <form onSubmit={handleLookup} className="space-y-2">
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2.5">
                  <div className="relative flex-1">
                    <input
                      ref={qrRef}
                      type="text"
                      value={qrInput}
                      onChange={(e) => setQrInput(e.target.value)}
                      placeholder="SUP-номер, коробка или QR..."
                      className="w-full bg-gray-50 border border-gray-300 rounded-lg pl-3.5 pr-10 py-2.5 text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 focus:bg-white transition-colors"
                      disabled={lookupLoading}
                      autoFocus
                    />
                    {lookupLoading && (
                      <div className="absolute right-3 top-1/2 -translate-y-1/2">
                        <RefreshCw className="h-4 w-4 text-indigo-600 animate-spin" />
                      </div>
                    )}
                  </div>
                  <button
                    type="submit"
                    disabled={lookupLoading || !qrInput.trim()}
                    className="inline-flex items-center justify-center px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold rounded-lg transition-colors disabled:opacity-50 shadow-2xs shrink-0"
                  >
                    {lookupLoading ? (
                      <RefreshCw className="h-4 w-4 animate-spin mr-1.5" />
                    ) : (
                      <Search className="h-4 w-4 mr-1.5" />
                    )}
                    Найти
                  </button>
                </div>
                <p className="text-xs text-gray-500">
                  Отсканируйте QR, код коробки или введите номер поставки
                </p>
              </form>
            </div>

            {/* Receiving Queue Section */}
            <div className="space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                <div>
                  <h2 className="text-lg font-bold text-gray-900">
                    Ожидают приёмки {queue.length > 0 && <span className="text-gray-400 font-normal">· {queue.length}</span>}
                  </h2>
                  <p className="text-xs text-gray-500 mt-0.5">
                    Поставки, которые уже прибыли на склад и требуют действий
                  </p>
                </div>

                <div className="flex flex-wrap items-center gap-2 self-start sm:self-auto">
                  {queue.length > 0 && (
                    <div className="flex flex-wrap items-center gap-1.5 text-xs">
                      <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-emerald-50 text-emerald-700 border border-emerald-200 font-medium">
                        Ожидают начала {countArrived}
                      </span>
                      <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-indigo-50 text-indigo-700 border border-indigo-200 font-medium">
                        В процессе {countReceiving}
                      </span>
                      {totalRemainingInQueue > 0 && (
                        <span className="inline-flex items-center px-2.5 py-1 rounded-md bg-amber-50 text-amber-800 border border-amber-200 font-medium">
                          Осталось принять {totalRemainingInQueue} шт.
                        </span>
                      )}
                    </div>
                  )}

                  <button
                    type="button"
                    onClick={fetchQueue}
                    disabled={queueLoading}
                    className="inline-flex items-center px-3 py-1.5 text-xs font-medium text-gray-700 bg-white hover:bg-gray-50 border border-gray-200 rounded-lg transition-colors shadow-2xs disabled:opacity-50"
                  >
                    <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${queueLoading ? 'animate-spin' : ''}`} />
                    Обновить
                  </button>
                </div>
              </div>

              {queueError && (
                <div className="p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-rose-800 flex items-center justify-between shadow-2xs">
                  <div className="flex items-center space-x-2 text-xs font-medium">
                    <AlertCircle className="w-4 h-4 flex-shrink-0 text-rose-600" />
                    <span>{queueError}</span>
                  </div>
                  <button
                    type="button"
                    onClick={fetchQueue}
                    className="px-2.5 py-1 bg-rose-100 hover:bg-rose-200 rounded-md text-xs font-semibold text-rose-800 transition-colors"
                  >
                    Повторить
                  </button>
                </div>
              )}

              {queueLoading && queue.length === 0 ? (
                <div className="p-8 text-center bg-white rounded-xl border border-gray-200 shadow-2xs">
                  <RefreshCw className="w-5 h-5 animate-spin mx-auto text-indigo-600 mb-2" />
                  <p className="text-xs text-gray-500 font-medium">Загрузка очереди приёмки...</p>
                </div>
              ) : queue.length === 0 ? (
                <div className="p-8 text-center bg-white rounded-xl border border-gray-200 shadow-2xs space-y-1.5">
                  <h3 className="text-sm font-semibold text-gray-900">
                    Нет поставок, ожидающих приёмки
                  </h3>
                  <p className="text-xs text-gray-500 max-w-sm mx-auto">
                    Все прибывшие поставки обработаны, либо новые поставки ещё не поступили на склад.
                  </p>
                </div>
              ) : (
                <div className="space-y-3">
                  {queue.map((item) => {
                    const isArrived = item.status === 'arrived_at_zamk';
                    const isReceiving = item.status === 'receiving';
                    const isOpening = openingSupplyId === item.supplyId;

                    return (
                      <div
                        key={item.supplyId}
                        className="bg-white rounded-xl border border-gray-200 p-4 sm:p-5 hover:border-indigo-300 hover:shadow-xs transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-4"
                      >
                        <div className="space-y-1.5">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-mono font-bold text-base text-gray-900">
                              {item.supplyNumber || 'Поставка'}
                            </span>
                            {isArrived && (
                              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                                Ожидает приёмки
                              </span>
                            )}
                            {isReceiving && (
                              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                                Приёмка начата
                              </span>
                            )}
                          </div>

                          <div className="text-sm font-medium text-gray-800">
                            {item.sellerName || 'Продавец ZAMK'}
                          </div>

                          <div className="text-xs text-gray-500 space-y-0.5">
                            {isArrived ? (
                              <>
                                <div>
                                  {formatCargoPlaces(item.cargoPlacesCount)} · {item.expectedUnitsCount} единиц
                                </div>
                                {item.arrivedAt && (
                                  <div>Прибыла {formatOperationalTime(item.arrivedAt)}</div>
                                )}
                              </>
                            ) : (
                              <>
                                <div>
                                  Принято {item.acceptedUnitsCount} из {item.expectedUnitsCount} · осталось {item.remainingUnitsCount}
                                </div>
                                {item.receivingStartedAt && (
                                  <div>Начата {formatOperationalTime(item.receivingStartedAt)}</div>
                                )}
                              </>
                            )}
                          </div>
                        </div>

                        <div className="flex items-center sm:self-center shrink-0">
                          <button
                            type="button"
                            onClick={() => handleSelectSupplyFromQueue(item)}
                            disabled={!canReceive || isOpening}
                            className="inline-flex items-center justify-center px-4 py-2 text-xs font-semibold rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white transition-colors shadow-2xs disabled:opacity-50"
                          >
                            {isOpening ? (
                              <RefreshCw className="w-3.5 h-3.5 animate-spin mr-1.5" />
                            ) : (
                              <ArrowRight className="w-3.5 h-3.5 mr-1.5" />
                            )}
                            {isReceiving ? 'Продолжить приёмку' : 'Начать приёмку'}
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="mx-4 sm:mx-6 bg-slate-800 border border-slate-700 rounded-xl p-6 max-w-4xl mx-auto shadow-xl space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between pb-4 border-b border-slate-700 gap-3">
              <div>
                <div className="flex items-center space-x-3">
                  <h2 className="text-2xl font-bold font-mono text-white tracking-wide">
                    {dossier.supplyNumber || dossier.humanId || dossier.id}
                  </h2>
                  {getStatusBadge(dossier.status)}
                </div>
                <p className="text-sm text-slate-400 mt-1">
                  Карточка поставки перед началом физической приёмки на складе ZAMK
                </p>
              </div>
              <button
                onClick={resetFlow}
                className="self-start sm:self-auto text-slate-400 hover:text-white flex items-center text-sm px-3 py-1.5 border border-slate-700 rounded-md hover:bg-slate-700 transition-colors"
              >
                <ArrowLeft className="h-4 w-4 mr-2" />
                К очереди приёмки
              </button>
            </div>

            {/* Details Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="bg-slate-900/60 border border-slate-700/60 rounded-lg p-4 space-y-2.5">
                <div className="flex items-center text-sm text-slate-400">
                  <Building2 className="w-4 h-4 mr-2 text-slate-400 flex-shrink-0" />
                  <span className="text-slate-400 font-medium">Продавец:</span>
                  <span className="ml-2 font-semibold text-white truncate">{dossier.sellerName || 'Продавец ZAMK'}</span>
                </div>
                <div className="flex items-center text-sm text-slate-400">
                  <Truck className="w-4 h-4 mr-2 text-slate-400 flex-shrink-0" />
                  <span className="text-slate-400 font-medium">Доставка:</span>
                  <span className="ml-2 text-slate-200">
                    {dossier.carrierName
                      ? `${dossier.carrierName} (трек: ${dossier.trackingNumber || '—'})`
                      : dossier.handoffMethod === 'self_delivery'
                      ? 'Самопривоз на склад'
                      : 'Доставка транспортной компанией'}
                  </span>
                </div>
                {dossier.shippedAt && (
                  <div className="flex items-center text-sm text-slate-400">
                    <Calendar className="w-4 h-4 mr-2 text-slate-400 flex-shrink-0" />
                    <span className="text-slate-400 font-medium">Отправлена:</span>
                    <span className="ml-2 text-slate-200">{new Date(dossier.shippedAt).toLocaleString('ru-RU')}</span>
                  </div>
                )}
              </div>

              <div className="bg-slate-900/60 border border-slate-700/60 rounded-lg p-4 space-y-2.5">
                <div className="flex items-center text-sm text-slate-400">
                  <Box className="w-4 h-4 mr-2 text-slate-400 flex-shrink-0" />
                  <span className="text-slate-400 font-medium">Грузоместа (коробки):</span>
                  <span className="ml-2 font-mono text-slate-200">
                    {dossier.boxes && dossier.boxes.length > 0
                      ? dossier.boxes.map((b) => b.boxNumber).join(', ')
                      : `${dossier.totalExpectedBoxes || 1} шт.`}
                  </span>
                </div>
                <div className="flex items-center text-sm text-slate-400">
                  <Layers className="w-4 h-4 mr-2 text-slate-400 flex-shrink-0" />
                  <span className="text-slate-400 font-medium">Ожидается товаров:</span>
                  <span className="ml-2 font-semibold text-emerald-400">
                    {dossier.totalExpectedItems} шт.{' '}
                    <span className="font-normal text-slate-400">({dossier.skuCount || dossier.items?.length || 0} SKU)</span>
                  </span>
                </div>
                {dossier.arrivedAt && (
                  <div className="flex items-center text-sm text-slate-400">
                    <CheckCircle2 className="w-4 h-4 mr-2 text-emerald-400 flex-shrink-0" />
                    <span className="text-slate-400 font-medium">Прибыла на склад:</span>
                    <span className="ml-2 text-emerald-300">{new Date(dossier.arrivedAt).toLocaleString('ru-RU')}</span>
                  </div>
                )}
              </div>
            </div>

            {/* Items preview */}
            {dossier.items && dossier.items.length > 0 && (
              <div className="bg-slate-900/80 border border-slate-700/60 rounded-lg overflow-hidden">
                <div className="px-4 py-3 bg-slate-800/60 border-b border-slate-700 text-xs font-semibold uppercase tracking-wider text-slate-400 flex justify-between items-center">
                  <span>Состав поставки</span>
                  {isAdditionalDossier && (
                    <span className="text-amber-400 font-mono font-normal normal-case">
                      Осталось принять: {remainingExpected} шт.
                    </span>
                  )}
                </div>
                <div className="max-h-56 overflow-y-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-xs text-slate-400 bg-slate-900/50 border-b border-slate-800">
                      <tr>
                        <th className="py-2.5 px-4">Товар</th>
                        <th className="py-2.5 px-4 font-mono">Артикул / Штрихкод</th>
                        <th className="py-2.5 px-4 text-right">Заявлено</th>
                        {isAdditionalDossier && (
                          <>
                            <th className="py-2.5 px-4 text-right text-emerald-400">Принято</th>
                            <th className="py-2.5 px-4 text-right text-rose-400">Брак</th>
                            <th className="py-2.5 px-4 text-right text-amber-400">Осталось</th>
                          </>
                        )}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {dossier.items.map((item) => (
                        <tr key={item.id} className="hover:bg-slate-800/40">
                          <td className="py-2.5 px-4 text-slate-200">
                            <span className="font-medium text-white">{item.productTitle || item.sku}</span>
                            {(item.colorName || item.sizeName) && (
                              <span className="text-xs text-slate-400 ml-2">
                                {[item.colorName, item.sizeName].filter(Boolean).join(' / ')}
                              </span>
                            )}
                          </td>
                          <td className="py-2.5 px-4 font-mono text-xs text-slate-400">
                            {item.sellerSku || item.sku}
                            {item.barcode ? ` · ${item.barcode}` : ''}
                          </td>
                          <td className="py-2.5 px-4 text-right font-semibold text-slate-200">
                            {item.expectedQuantity} шт.
                          </td>
                          {isAdditionalDossier && (
                            <>
                              <td className="py-2.5 px-4 text-right font-bold text-emerald-400">
                                {item.acceptedQuantity || 0} шт.
                              </td>
                              <td className="py-2.5 px-4 text-right font-bold text-rose-400">
                                {item.damagedQuantity || 0} шт.
                              </td>
                              <td className="py-2.5 px-4 text-right font-bold text-amber-400">
                                {item.missingQuantity || 0} шт.
                              </td>
                            </>
                          )}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Action Decision Section */}
            {dossier.status === 'shipped_by_seller' && (
              <div className="bg-amber-500/10 border border-amber-500/20 rounded-xl p-5 text-left flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                  <h4 className="text-amber-300 font-semibold text-base flex items-center">
                    <Truck className="w-5 h-5 mr-2" /> Поставка в пути
                  </h4>
                  <p className="text-slate-300 text-sm mt-1">
                    Подтвердите физическое прибытие поставки на склад ZAMK.
                  </p>
                </div>
                <button
                  onClick={handleMarkArrived}
                  disabled={!canReceive || arrivalLoading}
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-medium px-5 py-3 rounded-lg flex items-center justify-center transition-colors flex-shrink-0 disabled:opacity-50 shadow-lg"
                >
                  {arrivalLoading ? (
                    <RefreshCw className="w-5 h-5 animate-spin mr-2" />
                  ) : (
                    <CheckCircle2 className="w-5 h-5 mr-2" />
                  )}
                  Поставка прибыла
                </button>
              </div>
            )}

            {dossier.status === 'arrived_at_zamk' && (
              <div className="bg-emerald-500/10 border border-emerald-500/20 rounded-xl p-5 text-left flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                  <h4 className="text-emerald-300 font-semibold text-base flex items-center">
                    <CheckCircle2 className="w-5 h-5 mr-2" /> Поставка готова к приёмке
                  </h4>
                  <p className="text-slate-300 text-sm mt-1">
                    Физическое прибытие подтверждено. Откройте сессию для начала сканирования.
                  </p>
                </div>
                <button
                  onClick={handleStartOrResumeSession}
                  disabled={!canReceive || loading}
                  className="bg-blue-600 hover:bg-blue-500 text-white font-medium px-6 py-3 rounded-lg flex items-center justify-center transition-colors flex-shrink-0 disabled:opacity-50 shadow-lg"
                >
                  {loading ? (
                    <RefreshCw className="w-5 h-5 animate-spin mr-2" />
                  ) : (
                    <ArrowRight className="w-5 h-5 mr-2" />
                  )}
                  Начать приёмку
                </button>
              </div>
            )}

            {dossier.status === 'receiving' && (
              <div className="bg-indigo-500/10 border border-indigo-500/20 rounded-xl p-5 text-left flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                  <h4 className="text-indigo-300 font-semibold text-base flex items-center">
                    <ShieldCheck className="w-5 h-5 mr-2" /> {isAdditionalDossier ? 'Открыта активная сессия доприёмки' : 'Открыта активная сессия приёмки'}
                  </h4>
                  <p className="text-slate-300 text-sm mt-1">
                    {isAdditionalDossier
                      ? 'Для этой поставки открыта сессия доприёмки. Вы можете продолжить сканирование товаров.'
                      : 'Для этой поставки уже начата приёмка. Вы можете продолжить сканирование товаров.'}
                  </p>
                </div>
                <button
                  onClick={handleStartOrResumeSession}
                  disabled={!canReceive || loading}
                  className="bg-indigo-600 hover:bg-indigo-500 text-white font-medium px-6 py-3 rounded-lg flex items-center justify-center transition-colors flex-shrink-0 disabled:opacity-50 shadow-lg"
                >
                  {loading ? (
                    <RefreshCw className="w-5 h-5 animate-spin mr-2" />
                  ) : (
                    <ArrowRight className="w-5 h-5 mr-2" />
                  )}
                  {isAdditionalDossier ? 'Продолжить доприёмку' : 'Продолжить приёмку'}
                </button>
              </div>
            )}

            {dossier.status === 'completed_with_discrepancies' && (
              remainingExpected > 0 ? (
                <div className="bg-amber-500/10 border border-amber-500/20 rounded-xl p-5 text-left flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                  <div>
                    <h4 className="text-amber-300 font-semibold text-base flex items-center">
                      <AlertTriangle className="w-5 h-5 mr-2" /> Приёмка завершена с расхождениями
                    </h4>
                    <p className="text-slate-300 text-sm mt-1">
                      Осталось принять: <span className="font-bold text-amber-400">{remainingExpected} шт.</span>
                    </p>
                  </div>
                  <button
                    onClick={handleStartOrResumeSession}
                    disabled={!canReceive || loading}
                    className="bg-amber-600 hover:bg-amber-500 text-white font-medium px-6 py-3 rounded-lg flex items-center justify-center transition-colors flex-shrink-0 disabled:opacity-50 shadow-lg"
                  >
                    {loading ? (
                      <RefreshCw className="w-5 h-5 animate-spin mr-2" />
                    ) : (
                      <ArrowRight className="w-5 h-5 mr-2" />
                    )}
                    Доприёмка · осталось {remainingExpected}
                  </button>
                </div>
              ) : (
                <div className="bg-slate-900 border border-slate-700 rounded-xl p-5 text-left">
                  <h4 className="text-amber-400 font-medium text-sm flex items-center">
                    <AlertTriangle className="w-4 h-4 mr-2" />
                    Приёмка завершена с расхождениями (зафиксирован брак).
                  </h4>
                  <p className="text-slate-400 text-xs mt-1">
                    Все ожидаемые товарные единицы обработаны. Доприёмка не требуется.
                  </p>
                </div>
              )
            )}

            {dossier.status === 'completed' && (
              <div className="bg-slate-900 border border-slate-700 rounded-xl p-5 text-left">
                <h4 className="text-emerald-400 font-medium text-sm flex items-center">
                  <CheckCircle2 className="w-4 h-4 mr-2" />
                  Приёмка по этой поставке уже завершена без расхождений.
                </h4>
              </div>
            )}

            {(dossier.status === 'ready_to_ship' || dossier.status === 'draft') && (
              <div className="bg-slate-900 border border-slate-700 rounded-xl p-5 text-left">
                <h4 className="text-slate-300 font-medium text-sm flex items-center">
                  <AlertCircle className="w-4 h-4 mr-2 text-slate-400" />
                  Поставка ещё не передана перевозчику.
                </h4>
                <p className="text-slate-400 text-xs mt-1">
                  Продавец ещё не отправил поставку на склад ZAMK. Приёмка станет доступна после отправки и прибытия.
                </p>
              </div>
            )}

            {dossier.status === 'cancelled' && (
              <div className="bg-rose-500/10 border border-rose-500/20 rounded-xl p-5 text-left">
                <h4 className="text-rose-400 font-medium text-sm flex items-center">
                  <AlertTriangle className="w-4 h-4 mr-2 text-rose-400" />
                  Поставка отменена.
                </h4>
              </div>
            )}
          </div>
        )
      ) : (
        <div className="mx-4 sm:mx-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-6">
            <div className="bg-slate-800 border border-slate-700 rounded-xl p-6 shadow-lg">
              <div className="flex justify-between items-center mb-6">
                <div>
                  <h2 className="text-xl font-medium text-white">
                    {isAdditionalSession
                      ? 'Доприёмка единиц товара (ZMU)'
                      : isSerialized
                      ? 'Сканирование единиц товара (ZMU)'
                      : 'Сканирование товаров'}
                  </h2>
                  <p className="text-slate-400 text-sm mt-1">
                    Поставка:{' '}
                    <span className="font-mono font-bold text-blue-400 px-2 py-0.5 bg-blue-500/10 rounded">
                      {dossier?.supplyNumber || session.supplyId}
                    </span>
                    {isAdditionalSession && (
                      <span className="ml-2 text-xs font-semibold text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded">
                        План доприёмки: {totalExpected} шт.
                      </span>
                    )}
                  </p>
                </div>
                <button
                  onClick={resetFlow}
                  className="text-slate-400 hover:text-white flex items-center text-sm px-3 py-1.5 border border-slate-700 rounded-md hover:bg-slate-700 transition-colors"
                >
                  <ArrowLeft className="h-4 w-4 mr-2" />
                  К очереди приёмки
                </button>
              </div>

              {/* Scanner Form */}
              <form onSubmit={handleScanItem} className="relative mb-3">
                <input
                  ref={barcodeRef}
                  type="text"
                  value={barcodeInput}
                  onChange={(e) => setBarcodeInput(e.target.value)}
                  placeholder={isSerialized ? 'Сканируйте ZMU товара...' : 'Скан штрихкода товара...'}
                  className={`w-full bg-slate-900 border ${
                    isDamagedScan
                      ? 'border-rose-500/50 focus:ring-rose-500 shadow-[0_0_15px_rgba(244,63,94,0.1)]'
                      : 'border-blue-500/50 focus:ring-blue-500 shadow-[0_0_15px_rgba(59,130,246,0.1)]'
                  } rounded-lg pl-4 pr-16 py-4 text-lg text-white placeholder-slate-500 focus:outline-none focus:ring-2`}
                  disabled={!canReceive || loading}
                  autoFocus
                />
                <button
                  type="submit"
                  disabled={!canReceive || loading || !barcodeInput.trim()}
                  className={`absolute right-2 top-2 bottom-2 ${
                    isDamagedScan ? 'bg-rose-600 hover:bg-rose-500' : 'bg-blue-600 hover:bg-blue-500'
                  } text-white rounded-md px-4 transition-colors disabled:opacity-50`}
                >
                  <ArrowRight className="h-6 w-6" />
                </button>
              </form>

              {/* Helper text */}
              <div className="mb-6 space-y-1">
                <p className="text-xs text-slate-400">
                  {isSerialized
                    ? 'Сканируйте уникальную этикетку ZAMK на каждой единице товара.'
                    : 'Сканируйте штрихкод товара для добавления в счетчик.'}
                </p>
                {isSerialized && (
                  <p className="text-xs text-slate-500">Нет сканера? Введите ZMU вручную и нажмите Enter.</p>
                )}
              </div>

              {/* Damage Flag Toggle */}
              <div className="flex items-center mb-6 bg-slate-900/50 p-3 rounded-lg border border-slate-700">
                <label className="flex items-center space-x-3 cursor-pointer text-slate-300 hover:text-white transition-colors">
                  <input
                    type="checkbox"
                    className="form-checkbox h-5 w-5 text-rose-500 rounded border-slate-600 bg-slate-800 focus:ring-rose-500 focus:ring-offset-slate-900"
                    checked={isDamagedScan}
                    onChange={(e) => {
                      setIsDamagedScan(e.target.checked);
                      barcodeRef.current?.focus();
                    }}
                  />
                  <span className="font-medium text-sm">Следующий товар — брак</span>
                </label>
              </div>

              {/* Last Scanned Unit Feedback */}
              {isSerialized && lastScannedItem && (
                <div className="mb-6 bg-emerald-500/10 border border-emerald-500/30 rounded-lg p-3.5 flex items-center justify-between shadow-sm">
                  <div className="flex items-center space-x-3">
                    <CheckCircle2 className="h-5 w-5 text-emerald-400 flex-shrink-0" />
                    <div>
                      <div className="text-sm font-semibold text-emerald-300">
                        {lastScannedItem.condition === 'damaged' ? 'Зафиксирован брак' : 'Принято'}
                      </div>
                      <div className="text-xs text-slate-300 mt-0.5">
                        <span className="font-medium text-white">{lastScannedItem.productTitle}</span>
                        {(lastScannedItem.colorName || lastScannedItem.sizeName) && (
                          <span className="text-slate-400"> · {[lastScannedItem.colorName, lastScannedItem.sizeName].filter(Boolean).join(' ')}</span>
                        )}
                        {lastScannedItem.sellerSku && (
                          <span className="text-slate-400"> · Арт: {lastScannedItem.sellerSku}</span>
                        )}
                        <span className="font-mono text-emerald-300 ml-2 font-bold">{lastScannedItem.unitCode}</span>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Recent Scans Panel (Serialized mode only) */}
              {isSerialized && (
                <div className="mb-8 bg-slate-900/60 rounded-xl p-4 border border-slate-700">
                  <div className="flex items-center justify-between mb-3 pb-2 border-b border-slate-800">
                    <h3 className="text-sm font-semibold text-slate-200 flex items-center">
                      <RotateCcw className="w-4 h-4 mr-2 text-slate-400" />
                      Последние сканы
                    </h3>
                    <button
                      onClick={handleUndoLastScan}
                      disabled={!canReceive || undoLoading || !latestNonVoidedScan}
                      className="inline-flex items-center px-3 py-1.5 text-xs font-semibold text-amber-300 bg-amber-500/10 hover:bg-amber-500/20 border border-amber-500/30 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      <RotateCcw className={`w-3.5 h-3.5 mr-1.5 ${undoLoading ? 'animate-spin' : ''}`} />
                      Отменить последний скан
                    </button>
                  </div>

                  {recentScans.length === 0 ? (
                    <div className="py-6 text-center text-xs text-slate-500">
                      Сканов в этой сессии пока нет
                    </div>
                  ) : (
                    <div className="space-y-2 max-h-60 overflow-y-auto pr-1">
                      {recentScans.map((s) => {
                        const isVoided = Boolean(s.voidedAt);
                        const isDamaged = s.condition === 'damaged';
                        const timeStr = new Date(s.scannedAt).toLocaleTimeString('ru-RU', {
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit',
                        });

                        return (
                          <div
                            key={s.scanId}
                            className={`flex items-center justify-between px-3 py-2 rounded-lg text-xs border ${
                              isVoided
                                ? 'bg-slate-950/40 border-slate-800 text-slate-500 line-through'
                                : isDamaged
                                ? 'bg-rose-500/5 border-rose-500/20 text-slate-200'
                                : 'bg-slate-800/40 border-slate-700/50 text-slate-200'
                            }`}
                          >
                            <div className="flex items-center space-x-3 overflow-hidden">
                              <span className="font-mono text-slate-400 text-[11px]">{timeStr}</span>
                              <div className="truncate">
                                <span className="font-medium text-white">{s.productTitle}</span>
                                {(s.colorName || s.sizeName) && (
                                  <span className="text-slate-400"> · {[s.colorName, s.sizeName].filter(Boolean).join(' ')}</span>
                                )}
                                {s.sellerSku && <span className="text-slate-400"> · {s.sellerSku}</span>}
                                <span className="font-mono font-semibold text-blue-400 ml-2">{s.unitCode}</span>
                              </div>
                            </div>
                            <div className="flex-shrink-0 ml-3">
                              {isVoided ? (
                                <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-slate-800 text-slate-400 border border-slate-700">
                                  Отменён
                                </span>
                              ) : isDamaged ? (
                                <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
                                  Брак
                                </span>
                              ) : (
                                <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                                  Принято
                                </span>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}

              {/* Items Table */}
              <h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-4 flex items-center">
                <Box className="w-4 h-4 mr-2" /> {isSerialized ? 'Сводка по товарным позициям' : 'Ожидаемые товары'}
              </h3>

              {!session.items || session.items.length === 0 ? (
                <div className="text-center py-12 text-slate-500 border-2 border-dashed border-slate-700 rounded-xl bg-slate-900/30">
                  <AlertCircle className="mx-auto h-8 w-8 mb-3 opacity-50" />
                  <p className="text-sm">В поставке нет товаров</p>
                </div>
              ) : (
                <div className="bg-slate-900 rounded-xl overflow-hidden border border-slate-700">
                  <table className="w-full text-left border-collapse">
                    <thead>
                      <tr className="border-b border-slate-700 bg-slate-800/50">
                        <th className="py-3 px-4 text-xs font-semibold text-slate-400 uppercase tracking-wider">Товар / Штрихкод</th>
                        <th className="py-3 px-4 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">План</th>
                        <th className="py-3 px-4 text-xs font-semibold text-emerald-400 uppercase tracking-wider text-right">Ок</th>
                        <th className="py-3 px-4 text-xs font-semibold text-rose-400 uppercase tracking-wider text-right">Брак</th>
                        <th className="py-3 px-4 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">Осталось</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {session.items.map((item, idx) => {
                        const remaining = Math.max(0, item.expectedQuantity - item.scannedQuantity - item.damagedQuantity);
                        return (
                          <tr key={idx} className="hover:bg-slate-800/50 transition-colors">
                            <td className="py-4 px-4 text-sm font-mono text-white font-medium">
                              <div>{item.productTitle || item.sku}</div>
                              <div className="text-xs text-slate-400 font-mono mt-0.5">{item.barcode || item.sku}</div>
                            </td>
                            <td className="py-4 px-4 text-sm font-medium text-slate-300 text-right">{item.expectedQuantity}</td>
                            <td className="py-4 px-4 text-lg font-bold text-emerald-400 text-right">{item.scannedQuantity}</td>
                            <td className="py-4 px-4 text-lg font-bold text-rose-400 text-right">{item.damagedQuantity}</td>
                            <td className="py-4 px-4 text-sm font-medium text-slate-400 text-right">{remaining}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>

          {/* Right Summary Sidebar */}
          <div className="lg:col-span-1 space-y-6">
            <div className="bg-slate-800 border border-slate-700 rounded-xl p-6 shadow-lg sticky top-6">
              <h3 className="text-lg font-medium text-white mb-2">
                {isAdditionalSession ? 'Сводка доприёмки' : 'Сводка приёмки'}
              </h3>
              <p className="text-sm text-slate-400 mb-6">
                {isAdditionalSession
                  ? 'Контроль сканирования оставшихся единиц ZMU в текущей сессии доприёмки.'
                  : isSerialized
                  ? 'Контроль сканирования уникальных единиц ZMU.'
                  : 'После того как все товары из поставки отсканированы, нажмите завершить.'}
              </p>

              <div className="mb-6 p-4 bg-slate-900/50 rounded-lg border border-slate-700/50 space-y-3">
                <div className="flex justify-between items-center text-sm">
                  <span className="text-slate-400">
                    {isAdditionalSession ? 'План доприёмки:' : 'Заявлено:'}
                  </span>
                  <span className="font-bold text-white text-base">{totalExpected}</span>
                </div>
                <div className="flex justify-between items-center text-sm">
                  <span className="text-slate-400">Отсканировано:</span>
                  <span className="font-bold text-blue-400 text-base">{totalScanned}</span>
                </div>
                <div className="flex justify-between items-center text-sm">
                  <span className="text-slate-400">Принято (OK):</span>
                  <span className="font-bold text-emerald-400 text-base">{totalOk}</span>
                </div>
                <div className="flex justify-between items-center text-sm">
                  <span className="text-slate-400">Брак:</span>
                  <span className="font-bold text-rose-400 text-base">{totalDamaged}</span>
                </div>
                <div className="flex justify-between items-center text-sm border-t border-slate-800 pt-2">
                  <span className="text-slate-400">Осталось:</span>
                  <span className="font-bold text-orange-400 text-base">{totalRemaining}</span>
                </div>
              </div>

              {/* Clear confirmation summary before finalize */}
              <div className="mb-6 space-y-2 text-xs bg-slate-900/60 p-3.5 rounded-xl border border-slate-700/70">
                <div className="flex items-center text-emerald-400 font-medium">
                  <CheckCircle2 className="w-4 h-4 mr-2 flex-shrink-0" />
                  <span>На склад будет принято: {totalOk} шт.</span>
                </div>
                {hasDiscrepancy && (
                  <div className="flex items-center text-amber-400 font-medium">
                    <AlertTriangle className="w-4 h-4 mr-2 flex-shrink-0" />
                    <span>С расхождениями: {totalDamaged + totalRemaining} шт.</span>
                  </div>
                )}
              </div>

              <button
                type="button"
                onClick={() => setShowFinalizeModal(true)}
                disabled={!canReceive || loading}
                className={`w-full ${
                  isAdditionalSession
                    ? 'bg-amber-600 hover:bg-amber-500 shadow-amber-900/20'
                    : 'bg-emerald-600 hover:bg-emerald-500 shadow-emerald-900/20'
                } text-white font-bold py-4 px-4 rounded-xl flex items-center justify-center transition-colors shadow-lg disabled:opacity-50 disabled:cursor-not-allowed`}
              >
                {loading ? (
                  <RefreshCw className="w-5 h-5 animate-spin mr-2" />
                ) : (
                  <CheckCircle2 className="w-5 h-5 mr-2" />
                )}
                {isAdditionalSession ? 'Завершить доприёмку' : 'Завершить приёмку'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Finalize Confirmation Modal */}
      {showFinalizeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm">
          <div className="bg-slate-800 border border-slate-700 rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-6">
            <div className="flex items-start gap-4">
              <div
                className={`w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0 ${
                  totalRemaining > 0
                    ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                    : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                }`}
              >
                {totalRemaining > 0 ? (
                  <AlertTriangle className="w-6 h-6" />
                ) : (
                  <CheckCircle2 className="w-6 h-6" />
                )}
              </div>
              <div className="flex-1">
                <h3 className="text-xl font-bold text-white">
                  {totalRemaining > 0
                    ? isAdditionalSession
                      ? 'Завершить доприёмку с расхождениями?'
                      : 'Завершить приёмку с расхождениями?'
                    : isAdditionalSession
                    ? 'Завершить доприёмку?'
                    : 'Завершить приёмку?'}
                </h3>
                <p className="text-sm text-slate-400 mt-1">
                  {totalRemaining > 0
                    ? 'В текущей сессии отсканированы не все ожидаемые единицы товара.'
                    : 'Все единицы товара успешно отсканированы.'}
                </p>
              </div>
            </div>

            <div className="bg-slate-900/70 border border-slate-700/60 rounded-xl p-4 space-y-2.5 text-sm">
              <div className="flex justify-between items-center text-slate-300">
                <span className="text-slate-400">
                  {isAdditionalSession ? 'План доприёмки:' : 'Заявлено:'}
                </span>
                <span className="font-bold text-white font-mono">{totalExpected} шт.</span>
              </div>
              <div className="flex justify-between items-center text-slate-300">
                <span className="text-slate-400">Принято:</span>
                <span className="font-bold text-emerald-400 font-mono">{totalOk} шт.</span>
              </div>
              <div className="flex justify-between items-center text-slate-300">
                <span className="text-slate-400">Брак:</span>
                <span className="font-bold text-rose-400 font-mono">{totalDamaged} шт.</span>
              </div>
              <div className="flex justify-between items-center text-slate-300 border-t border-slate-800 pt-2">
                <span className="text-slate-400">Не принято:</span>
                <span
                  className={`font-bold font-mono ${
                    totalRemaining > 0 ? 'text-amber-400' : 'text-slate-400'
                  }`}
                >
                  {totalRemaining} шт.
                </span>
              </div>
            </div>

            {totalRemaining > 0 && (
              <div className="p-3.5 bg-amber-500/10 border border-amber-500/20 rounded-xl flex items-start gap-3 text-amber-300 text-xs leading-relaxed">
                <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                <span>
                  Неотсканированные единицы будут отмечены как недостающие. Их можно будет принять
                  позже через доприёмку.
                </span>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <button
                type="button"
                onClick={() => setShowFinalizeModal(false)}
                disabled={!canReceive || loading}
                className="flex-1 bg-slate-700 hover:bg-slate-600 text-white font-medium py-3 px-4 rounded-xl transition-colors disabled:opacity-50 text-sm"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleFinalizeConfirm}
                disabled={!canReceive || loading}
                className={`flex-1 ${
                  totalRemaining > 0
                    ? 'bg-amber-600 hover:bg-amber-500'
                    : isAdditionalSession
                    ? 'bg-amber-600 hover:bg-amber-500'
                    : 'bg-emerald-600 hover:bg-emerald-500'
                } text-white font-bold py-3 px-4 rounded-xl transition-colors shadow-lg flex items-center justify-center gap-2 disabled:opacity-50 text-sm`}
              >
                {loading ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <CheckCircle2 className="w-4 h-4" />
                )}
                {totalRemaining > 0
                  ? 'Завершить с расхождениями'
                  : isAdditionalSession
                  ? 'Завершить доприёмку'
                  : 'Завершить приёмку'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
