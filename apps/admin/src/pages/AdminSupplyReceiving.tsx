import React, { useState, useRef, useEffect } from 'react';
import {
  Truck,
  Box,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  RefreshCw,
  AlertCircle,
  RotateCcw,
  Tag,
  ShieldCheck,
  Building2,
  Calendar,
  Layers,
  Search,
  Volume2,
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
} from '@zamk/api-client/src/admin';
import type {
  SellerSupply,
  SupplyReceivingSession,
  SerializedRecentScan,
  SerializedScanResponse,
} from '@zamk/api-client/src/types';

import { playBeepSound, unlockScannerAudio } from '../utils/audio';

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
    case 'receiving_session_already_active':
      return 'Для этой поставки уже открыта приёмка.';
    case 'invalid_receiving_code':
      return 'Введите номер поставки, грузоместа или отсканируйте QR-код.';
    case 'unit_already_scanned':
      return 'Эта единица уже отсканирована.';
    case 'unit_already_received':
      return 'Эта единица уже принята.';
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
    case 'serialized_finalize_not_supported':
      return 'Завершение приёмки будет доступно после проверки всех единиц.';
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
    case 'completed':
    case 'completed_with_discrepancies':
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

export function AdminSupplyReceiving() {
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

  const [isFinalized, setIsFinalized] = useState(false);
  const [audioTestError, setAudioTestError] = useState<string | null>(null);

  const qrRef = useRef<HTMLInputElement>(null);
  const barcodeRef = useRef<HTMLInputElement>(null);

  const isSerialized = session?.receivingMode === 'serialized';

  // Synchronously unlock Safari audio on first user gesture while component is mounted
  useEffect(() => {
    const handleUserGesture = () => {
      unlockScannerAudio();
    };
    window.addEventListener('keydown', handleUserGesture, { capture: true, passive: true });
    window.addEventListener('pointerdown', handleUserGesture, { capture: true, passive: true });
    return () => {
      window.removeEventListener('keydown', handleUserGesture, { capture: true });
      window.removeEventListener('pointerdown', handleUserGesture, { capture: true });
    };
  }, []);

  const handleTestSound = () => {
    const ok = unlockScannerAudio();
    if (!ok) {
      setAudioTestError('Браузер не разрешил воспроизведение звука.');
      return;
    }
    setAudioTestError(null);
    playBeepSound('success');
  };

  useEffect(() => {
    if (!session && !dossier && !isFinalized) {
      qrRef.current?.focus();
    } else if (session && !isFinalized) {
      barcodeRef.current?.focus();
    }
  }, [session, dossier, isFinalized]);

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
    unlockScannerAudio();
    const input = qrInput.trim();
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
    unlockScannerAudio();
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
    unlockScannerAudio();
    if (!dossier) return;
    const lookupCode = dossier.qrToken || dossier.supplyNumber || dossier.id;
    if (!lookupCode) return;

    try {
      setLoading(true);
      setError(null);
      setSuccessMessage(null);
      setIsFinalized(false);
      setLastScannedItem(null);
      const data = await startSupplyReceivingSession(lookupCode);
      setSession(data);
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
    setError(null);
    setSuccessMessage(null);
    setAudioTestError(null);
    setQrInput('');
  };

  const handleScanItem = async (e: React.FormEvent) => {
    e.preventDefault();
    unlockScannerAudio();
    const rawInput = barcodeInput.trim();
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
    unlockScannerAudio();
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

  const handleFinalize = async () => {
    unlockScannerAudio();
    if (!session || isSerialized) return;
    try {
      setLoading(true);
      setError(null);

      await finalizeSupplyReceivingSession(session.id, {});

      setIsFinalized(true);
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

  return (
    <div className="space-y-6 pb-20">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Приемка поставок (Supplies)</h1>
          <p className="text-sm text-slate-400 mt-1">
            {isSerialized
              ? 'Сериализованная приёмка физических единиц по ZMU'
              : 'Сканирование QR поставок и штрихкодов товаров'}
          </p>
        </div>
        <div className="mt-3 sm:mt-0 flex items-center space-x-3">
          <button
            type="button"
            onClick={handleTestSound}
            className="text-slate-400 hover:text-white flex items-center text-xs px-2.5 py-1.5 border border-slate-700 rounded-md hover:bg-slate-700 transition-colors"
            title="Проверить звуковой сигнал сканера"
          >
            <Volume2 className="h-3.5 w-3.5 mr-1.5 text-blue-400" />
            Проверить звук
          </button>
          {session && (
            <div>
              {isSerialized ? (
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

      {audioTestError && (
        <div className="mx-4 sm:mx-6 bg-amber-500/10 border border-amber-500/20 rounded-lg p-3 flex items-center">
          <AlertCircle className="h-4 w-4 text-amber-400 mr-2 flex-shrink-0" />
          <span className="text-amber-200 text-xs font-medium">{audioTestError}</span>
        </div>
      )}

      {error && (
        <div className="mx-4 sm:mx-6 bg-rose-500/10 border border-rose-500/20 rounded-lg p-4 flex items-center">
          <AlertTriangle className="h-5 w-5 text-rose-500 mr-3 flex-shrink-0" />
          <span className="text-rose-200 text-sm font-medium">{error}</span>
        </div>
      )}

      {successMessage && (
        <div className="mx-4 sm:mx-6 bg-emerald-500/10 border border-emerald-500/20 rounded-lg p-4 flex items-center">
          <CheckCircle2 className="h-5 w-5 text-emerald-500 mr-3 flex-shrink-0" />
          <span className="text-emerald-200 text-sm font-medium">{successMessage}</span>
        </div>
      )}

      {isFinalized && session ? (
        <div className="mx-4 sm:mx-6">
          <div className="bg-slate-800 border border-slate-700 rounded-xl p-8 max-w-4xl mx-auto shadow-2xl">
            <div className="text-center mb-8">
              <div className="inline-flex items-center justify-center w-20 h-20 rounded-full bg-emerald-500/10 mb-4">
                <CheckCircle2 className="h-10 w-10 text-emerald-500" />
              </div>
              <h2 className="text-3xl font-bold text-white mb-2">Приёмка завершена</h2>
              <p className="text-slate-400">
                Сессия <span className="font-mono text-emerald-400">{session.id}</span> успешно закрыта.
              </p>
            </div>

            <div className="bg-slate-900 border border-slate-700 rounded-xl overflow-hidden mb-8">
              <div className="px-6 py-4 border-b border-slate-700 bg-slate-800/50">
                <h3 className="font-medium text-white">Итоги приёмки {hasDiscrepancy && '(есть расхождения)'}</h3>
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

            <button
              onClick={resetFlow}
              className="w-full bg-slate-700 hover:bg-slate-600 text-white font-medium py-3 px-4 rounded-lg flex items-center justify-center transition-colors"
            >
              <RefreshCw className="w-5 h-5 mr-2" />
              Вернуться к приёмке поставок
            </button>
          </div>
        </div>
      ) : !session ? (
        !dossier ? (
          <div className="mx-4 sm:mx-6 bg-slate-800 border border-slate-700 rounded-xl p-8 max-w-2xl text-center shadow-lg">
            <Truck className="mx-auto h-16 w-16 text-slate-500 mb-4" />
            <h2 className="text-xl font-medium text-white mb-2">Поиск поставки</h2>
            <p className="text-slate-400 mb-6 text-sm">
              Отсканируйте QR-код поставки (SUP-XXXXX), штрихкод коробки или введите номер вручную.
            </p>

            <form onSubmit={handleLookup} className="max-w-md mx-auto relative">
              <input
                ref={qrRef}
                type="text"
                value={qrInput}
                onChange={(e) => setQrInput(e.target.value)}
                placeholder="Номер SUP-..., коробка или скан QR..."
                className="w-full bg-slate-900 border border-slate-600 rounded-lg pl-4 pr-12 py-3 text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-inner"
                disabled={lookupLoading}
                autoFocus
              />
              <button
                type="submit"
                disabled={lookupLoading || !qrInput.trim()}
                className="absolute right-2 top-2 bottom-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md px-3 transition-colors disabled:opacity-50 flex items-center justify-center"
              >
                {lookupLoading ? <RefreshCw className="h-5 w-5 animate-spin" /> : <Search className="h-5 w-5" />}
              </button>
            </form>
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
                <RefreshCw className="h-4 w-4 mr-2" />
                Новый поиск
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
                <div className="px-4 py-3 bg-slate-800/60 border-b border-slate-700 text-xs font-semibold uppercase tracking-wider text-slate-400">
                  Состав поставки
                </div>
                <div className="max-h-48 overflow-y-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="text-xs text-slate-400 bg-slate-900/50 border-b border-slate-800">
                      <tr>
                        <th className="py-2.5 px-4">Товар</th>
                        <th className="py-2.5 px-4 font-mono">Артикул / Штрихкод</th>
                        <th className="py-2.5 px-4 text-right">Ожидается</th>
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
                  disabled={arrivalLoading}
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
                  disabled={loading}
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
                    <ShieldCheck className="w-5 h-5 mr-2" /> Открыта активная сессия приёмки
                  </h4>
                  <p className="text-slate-300 text-sm mt-1">
                    Для этой поставки уже начата приёмка. Вы можете продолжить сканирование товаров.
                  </p>
                </div>
                <button
                  onClick={handleStartOrResumeSession}
                  disabled={loading}
                  className="bg-indigo-600 hover:bg-indigo-500 text-white font-medium px-6 py-3 rounded-lg flex items-center justify-center transition-colors flex-shrink-0 disabled:opacity-50 shadow-lg"
                >
                  {loading ? (
                    <RefreshCw className="w-5 h-5 animate-spin mr-2" />
                  ) : (
                    <ArrowRight className="w-5 h-5 mr-2" />
                  )}
                  Продолжить приёмку
                </button>
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

            {(dossier.status === 'completed' || dossier.status === 'completed_with_discrepancies') && (
              <div className="bg-slate-900 border border-slate-700 rounded-xl p-5 text-left">
                <h4 className="text-emerald-400 font-medium text-sm flex items-center">
                  <CheckCircle2 className="w-4 h-4 mr-2" />
                  Приёмка по этой поставке уже завершена.
                </h4>
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
                    {isSerialized ? 'Сканирование единиц товара (ZMU)' : 'Сканирование товаров'}
                  </h2>
                  <p className="text-slate-400 text-sm mt-1">
                    Поставка: <span className="font-mono font-bold text-blue-400 px-2 py-0.5 bg-blue-500/10 rounded">{dossier?.supplyNumber || session.supplyId}</span>
                  </p>
                </div>
                <div className="flex items-center space-x-2">
                  <button
                    type="button"
                    onClick={handleTestSound}
                    className="text-slate-400 hover:text-white flex items-center text-xs px-2.5 py-1.5 border border-slate-700 rounded-md hover:bg-slate-700 transition-colors"
                    title="Проверить звуковой сигнал"
                  >
                    <Volume2 className="h-3.5 w-3.5 mr-1.5 text-blue-400" />
                    Проверить звук
                  </button>
                  <button
                    onClick={resetFlow}
                    className="text-slate-400 hover:text-white flex items-center text-sm px-3 py-1.5 border border-slate-700 rounded-md hover:bg-slate-700 transition-colors"
                  >
                    <RefreshCw className="h-4 w-4 mr-2" />
                    Сбросить
                  </button>
                </div>
              </div>

              {/* Scanner Form */}
              <form onSubmit={handleScanItem} className="relative mb-3">
                <input
                  ref={barcodeRef}
                  type="text"
                  value={barcodeInput}
                  onChange={(e) => setBarcodeInput(e.target.value)}
                  onKeyDown={() => unlockScannerAudio()}
                  placeholder={isSerialized ? 'Сканируйте ZMU товара...' : 'Скан штрихкода товара...'}
                  className={`w-full bg-slate-900 border ${
                    isDamagedScan
                      ? 'border-rose-500/50 focus:ring-rose-500 shadow-[0_0_15px_rgba(244,63,94,0.1)]'
                      : 'border-blue-500/50 focus:ring-blue-500 shadow-[0_0_15px_rgba(59,130,246,0.1)]'
                  } rounded-lg pl-4 pr-16 py-4 text-lg text-white placeholder-slate-500 focus:outline-none focus:ring-2`}
                  disabled={loading}
                  autoFocus
                />
                <button
                  type="submit"
                  disabled={loading || !barcodeInput.trim()}
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
                      disabled={undoLoading || !latestNonVoidedScan}
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
              <h3 className="text-lg font-medium text-white mb-2">Сводка приёмки</h3>
              <p className="text-sm text-slate-400 mb-6">
                {isSerialized
                  ? 'Контроль сканирования уникальных единиц ZMU.'
                  : 'После того как все товары из поставки отсканированы, нажмите завершить.'}
              </p>

              <div className="mb-6 p-4 bg-slate-900/50 rounded-lg border border-slate-700/50 space-y-3">
                <div className="flex justify-between items-center text-sm">
                  <span className="text-slate-400">Заявлено:</span>
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

              {isSerialized ? (
                <div className="p-4 bg-slate-900/60 border border-slate-700/60 rounded-xl text-center">
                  <AlertCircle className="w-6 h-6 text-blue-400 mx-auto mb-2" />
                  <div className="text-sm font-semibold text-slate-200">Сериализованная приёмка</div>
                  <p className="text-xs text-slate-400 mt-1">
                    Завершение приёмки будет доступно после проверки всех единиц.
                  </p>
                </div>
              ) : (
                <button
                  onClick={handleFinalize}
                  disabled={loading || totalScanned === 0}
                  className="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-4 px-4 rounded-xl flex items-center justify-center transition-colors shadow-lg shadow-emerald-900/20 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <CheckCircle2 className="w-5 h-5 mr-2" />
                  Завершить приёмку
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
