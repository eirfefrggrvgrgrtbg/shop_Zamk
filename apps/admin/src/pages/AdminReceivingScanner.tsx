import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  QrCode, CheckCircle2, AlertTriangle, ArrowRight, RefreshCw, Box, ShieldAlert, Truck, Printer
} from 'lucide-react';
import { formatOrderNumber } from '../utils/orderFormatters';
import { resolveReceivingCode, startReceiving, scanItem, confirmReceiving, recordDiscrepancy } from '../api/adminOrders';
import type { AdminFulfillment } from '@zamk/api-client/src/types';

interface ReceivingSessionState {
  id: string;
  fulfillmentId: string;
  status: string;
  version: number;
  canConfirm: boolean;
  items: Array<{
    id: string;
    sku: string;
    barcode?: string | null;
    productTitle: string;
    expectedQuantity: number;
    scannedQuantity: number;
  }>;
}

import { playBeepSound } from '../utils/audio';

export function AdminReceivingScanner() {
  const navigate = useNavigate();
  const inputRef = useRef<HTMLInputElement>(null);

  const [scannedCodeInput, setScannedCodeInput] = useState('');
  const [activeFulfillment, setActiveFulfillment] = useState<AdminFulfillment | null>(null);
  const [session, setSession] = useState<ReceivingSessionState | null>(null);
  const [isResolving, setIsResolving] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);
  const [isAlreadyAccepted, setIsAlreadyAccepted] = useState(false);

  const [itemBarcodeInput, setItemBarcodeInput] = useState('');
  const itemInputRef = useRef<HTMLInputElement>(null);

  // Discrepancy Modal
  const [isDiscrepancyModalOpen, setIsDiscrepancyModalOpen] = useState(false);
  const [discrepancyReason, setDiscrepancyReason] = useState('shortage');
  const [discrepancyComment, setDiscrepancyComment] = useState('');

  // Result Screen
  const [createdShipmentId, setCreatedShipmentId] = useState<string | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (!activeFulfillment) {
      inputRef.current?.focus();
    } else {
      itemInputRef.current?.focus();
    }
  }, [activeFulfillment, isSuccess]);

  const handleCodeSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const code = scannedCodeInput.trim();
    if (!code) return;

    try {
      setIsResolving(true);
      setScanError(null);
      setIsAlreadyAccepted(false);

      const f = await resolveReceivingCode(code);
      if (f.status === 'accepted') {
        setIsAlreadyAccepted(true);
        setActiveFulfillment(f);
        playBeepSound('error');
        return;
      }

      const sessData = await startReceiving(f.id);
      setActiveFulfillment(f);
      setSession(sessData);
      playBeepSound('success');
    } catch (err: any) {
      setScanError(err.message || 'Сборка не найдена или недоступна для приёмки');
      playBeepSound('error');
    } finally {
      setIsResolving(false);
      setScannedCodeInput('');
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  };

  const handleItemScan = async (e: React.FormEvent) => {
    e.preventDefault();
    const barcode = itemBarcodeInput.trim();
    if (!barcode || !activeFulfillment || !session) return;

    try {
      setScanError(null);
      const idempotencyKey = typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : 'scan-' + Date.now();
      const updatedSess = await scanItem(activeFulfillment.id, {
        barcode,
        expectedVersion: session.version,
        idempotencyKey,
      });
      setSession(updatedSess);
      playBeepSound('success');
    } catch (err: any) {
      playBeepSound('error');
      let msg = err.message || 'Ошибка сканирования товара';
      if (err.code === 'invalid_barcode') {
        msg = `Штрихкод "${barcode}" не найден в этой сборке.`;
      } else if (err.code === 'excess_quantity') {
        msg = `Превышено допустимое количество для штрихкода "${barcode}".`;
      }
      setScanError(msg);
      alert(msg);
    } finally {
      setItemBarcodeInput('');
      setTimeout(() => itemInputRef.current?.focus(), 50);
    }
  };

  const handleConfirmReceiving = async () => {
    if (!activeFulfillment || !session) return;
    try {
      setIsSubmitting(true);
      const idempotencyKey = typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : 'confirm-' + Date.now();
      const res = await confirmReceiving(activeFulfillment.id, {
        sessionId: session.id,
        expectedVersion: session.version,
        idempotencyKey,
      });
      setIsSuccess(true);
      setCreatedShipmentId(res.shipmentId || res.shipment?.id || null);
      playBeepSound('success');
    } catch (err: any) {
      alert('Ошибка приёмки: ' + err.message);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRecordDiscrepancy = async () => {
    if (!activeFulfillment || !session) return;
    try {
      setIsSubmitting(true);
      await recordDiscrepancy(activeFulfillment.id, {
        sessionId: session.id,
        reason: discrepancyReason,
        comment: discrepancyComment || 'Расхождение состава при сканировании на хабе',
      });
      setIsDiscrepancyModalOpen(false);
      alert('Расхождение зафиксировано. Сборка переведена в статус «Обнаружено расхождение».');
      resetScanner();
    } catch (err: any) {
      alert('Ошибка фиксации расхождения: ' + err.message);
    } finally {
      setIsSubmitting(false);
    }
  };

  const resetScanner = () => {
    setActiveFulfillment(null);
    setSession(null);
    setIsSuccess(false);
    setCreatedShipmentId(null);
    setIsAlreadyAccepted(false);
    setScanError(null);
    setScannedCodeInput('');
    setTimeout(() => inputRef.current?.focus(), 100);
  };

  return (
    <div data-testid="receiving-page" className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Приёмка сборок по QR/штрихкоду</h1>
          <p className="text-sm text-gray-500 mt-1">Быстрый сканер кладовщика хаба ZAMK</p>
        </div>
      </div>

      <div className="px-4 sm:px-6 space-y-6 max-w-5xl mx-auto">
        {/* SUCCESS SCREEN */}
        {isSuccess ? (
          <div className="bg-slate-900/90 border border-emerald-500/60 rounded-2xl p-8 text-center space-y-6 shadow-2xl animate-fade-in">
            <div className="w-16 h-16 bg-emerald-950 border border-emerald-500/80 rounded-full flex items-center justify-center mx-auto text-emerald-400">
              <CheckCircle2 className="h-10 w-10" />
            </div>
            <div>
              <h2 className="text-2xl font-bold text-white">Сборка успешно принята на хабе!</h2>
              <p className="text-sm text-slate-300 mt-1">
                Для данной сборки создано единое отправление {createdShipmentId && <span className="font-mono text-indigo-400 font-bold">({createdShipmentId})</span>}.
              </p>
            </div>

            <div className="flex flex-wrap justify-center gap-3 pt-4 border-t border-slate-800">
              {createdShipmentId && (
                <button
                  onClick={() => navigate(`/shipments`)}
                  className="px-5 py-2.5 bg-indigo-600 hover:bg-indigo-500 text-white font-bold text-sm rounded-xl inline-flex items-center gap-2 transition-colors"
                >
                  <Truck className="h-4 w-4" />
                  <span>Открыть отправления</span>
                </button>
              )}
              <button
                onClick={() => alert('Этикетка отправлена на печать')}
                className="px-5 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold text-sm rounded-xl inline-flex items-center gap-2 transition-colors border border-slate-700"
              >
                <Printer className="h-4 w-4" />
                <span>Распечатать этикетку</span>
              </button>
              <button
                onClick={resetScanner}
                className="px-5 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-sm rounded-xl inline-flex items-center gap-2 transition-colors"
              >
                <RefreshCw className="h-4 w-4" />
                <span>Принять следующую сборку</span>
              </button>
            </div>
          </div>
        ) : isAlreadyAccepted && activeFulfillment ? (
          /* RESCAN SCREEN */
          <div className="bg-slate-900/90 border border-amber-500/60 rounded-2xl p-8 text-center space-y-6 shadow-2xl">
            <div className="w-16 h-16 bg-amber-950 border border-amber-500/80 rounded-full flex items-center justify-center mx-auto text-amber-400">
              <ShieldAlert className="h-10 w-10" />
            </div>
            <div>
              <h2 className="text-2xl font-bold text-white">Сборка уже принята ранее!</h2>
              <p className="text-sm text-slate-300 mt-1">
                Код приёмки: <span className="font-mono text-indigo-400 font-bold">{activeFulfillment.receivingCode}</span> • Дата приёмки: {activeFulfillment.acceptedAt || 'Ранее'}
              </p>
            </div>

            <div className="flex justify-center gap-3 pt-4 border-t border-slate-800">
              <button
                onClick={() => navigate(`/orders/${activeFulfillment.orderId}`)}
                className="px-5 py-2.5 bg-indigo-600 hover:bg-indigo-500 text-white font-bold text-sm rounded-xl inline-flex items-center gap-2"
              >
                <span>Открыть заказ</span>
                <ArrowRight className="h-4 w-4" />
              </button>
              <button
                onClick={resetScanner}
                className="px-5 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold text-sm rounded-xl"
              >
                Сканировать другую сборку
              </button>
            </div>
          </div>
        ) : !activeFulfillment || !session ? (
          /* INITIAL SCANNER SCREEN */
          <div className="bg-slate-900/90 border border-slate-800 rounded-2xl p-8 space-y-6 shadow-2xl">
            <div className="text-center space-y-2">
              <div className="w-14 h-14 bg-indigo-950 border border-indigo-500/50 rounded-2xl flex items-center justify-center mx-auto text-indigo-400 shadow-lg">
                <QrCode className="h-8 w-8" />
              </div>
              <h2 className="text-xl font-bold text-white">Отсканируйте QR или штрихкод сборки</h2>
              <p className="text-xs text-slate-400">
                Поддерживается обычный аппаратный сканер. Отсканируйте этикетку упаковки продавца или введите код вручную.
              </p>
            </div>

            <form onSubmit={handleCodeSubmit} className="space-y-4 max-w-xl mx-auto">
              <div className="relative">
                <input
                  data-testid="receiving-code-input"
                  ref={inputRef}
                  type="text"
                  placeholder="FUL-2026-XXXXXX или сканируйте QR..."
                  value={scannedCodeInput}
                  onChange={(e) => setScannedCodeInput(e.target.value)}
                  className="w-full py-4 pl-5 pr-12 text-lg font-mono font-bold bg-slate-950 border-2 border-indigo-500/80 rounded-2xl text-white placeholder-slate-600 focus:outline-none focus:border-indigo-400 focus:ring-4 focus:ring-indigo-500/20 shadow-inner"
                />
                <button
                  data-testid="receiving-search-submit"
                  type="submit"
                  disabled={isResolving || !scannedCodeInput.trim()}
                  className="absolute right-2 top-2 bottom-2 px-4 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white font-bold rounded-xl text-sm transition-colors"
                >
                  {isResolving ? 'Поиск...' : 'Найти'}
                </button>
              </div>
            </form>

            {scanError && (
              <div className="p-4 bg-rose-950/80 border border-rose-800 text-rose-300 rounded-xl flex items-center gap-3 text-sm max-w-xl mx-auto">
                <AlertTriangle className="h-5 w-5 text-rose-400 shrink-0" />
                <span className="font-medium">{scanError}</span>
              </div>
            )}
          </div>
        ) : (
          /* RECEIVING WORKSPACE SCREEN */
          <div className="space-y-6">
            <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-lg font-bold text-indigo-400">{activeFulfillment.receivingCode || activeFulfillment.id}</span>
                  <span className="px-2.5 py-0.5 rounded-full text-xs font-bold bg-indigo-950 text-indigo-300 border border-indigo-800">
                    Приёмка v{session.version}
                  </span>
                </div>
                <p className="text-xs text-slate-400 mt-1">
                  Заказ: {formatOrderNumber({ id: activeFulfillment.orderId, orderNumber: activeFulfillment.orderNumber })} • Продавец: {activeFulfillment.sellerName || 'Продавец ZAMK'}
                </p>
              </div>

              <button
                onClick={resetScanner}
                className="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-bold rounded-lg border border-slate-700"
              >
                Отмена приёмки
              </button>
            </div>

            {/* Barcode scanner input */}
            <form onSubmit={handleItemScan} className="bg-slate-950 p-4 rounded-xl border border-indigo-500/40 flex items-center gap-3">
              <Box className="h-5 w-5 text-indigo-400 shrink-0" />
              <input
                data-testid="receiving-item-barcode-input"
                ref={itemInputRef}
                type="text"
                placeholder="Сканируйте штрихкод каждой позиции (Barcode / SKU)..."
                value={itemBarcodeInput}
                onChange={(e) => setItemBarcodeInput(e.target.value)}
                className="flex-1 bg-transparent border-none text-white text-sm focus:outline-none placeholder-slate-500 font-mono"
              />
              <button
                data-testid="receiving-item-scan-submit"
                type="submit"
                className="px-4 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white font-bold text-xs rounded-lg"
              >
                Сканировать
              </button>
            </form>

            {scanError && (
              <div className="p-3 bg-rose-950/80 border border-rose-800 text-rose-300 rounded-xl flex items-center gap-2 text-xs">
                <AlertTriangle className="h-4 w-4 text-rose-400 shrink-0" />
                <span>{scanError}</span>
              </div>
            )}

            {/* Expected items list */}
            <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
              <table className="min-w-full divide-y divide-slate-800 text-sm text-left">
                <thead className="bg-slate-950 text-slate-400 text-xs font-semibold uppercase">
                  <tr>
                    <th className="px-4 py-3.5">Товар / Вариант</th>
                    <th className="px-4 py-3.5">SKU / Штрихкод</th>
                    <th className="px-4 py-3.5 text-center">Ожидается</th>
                    <th className="px-4 py-3.5 text-center">Отсканировано</th>
                    <th className="px-4 py-3.5">Состояние</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800 text-slate-300">
                  {session.items.map((item) => (
                    <tr key={item.id} className={item.scannedQuantity === item.expectedQuantity ? 'bg-emerald-950/20' : ''}>
                      <td className="px-4 py-3.5 font-medium text-white">{item.productTitle}</td>
                      <td className="px-4 py-3.5 font-mono text-xs text-indigo-300">
                        {item.barcode ? `${item.barcode} (${item.sku})` : item.sku}
                      </td>
                      <td className="px-4 py-3.5 text-center font-bold text-white">{item.expectedQuantity} шт.</td>
                      <td className="px-4 py-3.5 text-center font-bold text-emerald-400 text-base">{item.scannedQuantity} шт.</td>
                      <td className="px-4 py-3.5">
                        {item.scannedQuantity === 0 ? (
                          <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-800 text-slate-400">Не проверено</span>
                        ) : item.scannedQuantity === item.expectedQuantity ? (
                          <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-950 text-emerald-300 border border-emerald-800">Совпало ✓</span>
                        ) : (
                          <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-950 text-amber-300 border border-amber-800">Недостача</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Receiving Action Buttons */}
            <div className="flex flex-col sm:flex-row justify-end gap-3 pt-2">
              <button
                data-testid="receiving-discrepancy"
                onClick={() => setIsDiscrepancyModalOpen(true)}
                className="px-5 py-2.5 bg-amber-950/80 hover:bg-amber-900 border border-amber-800 text-amber-300 font-bold text-sm rounded-xl transition-colors"
              >
                Зафиксировать расхождение
              </button>
              <button
                data-testid="receiving-confirm"
                disabled={isSubmitting || !session.canConfirm}
                onClick={handleConfirmReceiving}
                className="px-6 py-2.5 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 text-white font-bold text-sm rounded-xl transition-colors shadow-lg"
              >
                {isSubmitting ? 'Обработка...' : 'Принять и создать отправление'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* DISCREPANCY MODAL */}
      {isDiscrepancyModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full p-6 space-y-5 shadow-2xl">
            <h3 className="text-lg font-bold text-white flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-amber-400" />
              <span>Фиксация расхождения при приёмке</span>
            </h3>

            <div className="space-y-3">
              <label className="block text-xs font-medium text-slate-400">Причина расхождения:</label>
              <select
                value={discrepancyReason}
                onChange={(e) => setDiscrepancyReason(e.target.value)}
                className="w-full p-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-white focus:outline-none focus:border-indigo-500"
              >
                <option value="shortage">Недостача товара</option>
                <option value="excess">Лишний товар</option>
                <option value="wrong_item">Неверный товар / пересорт</option>
                <option value="damaged">Повреждение товара / дефект</option>
                <option value="broken_packaging">Упаковка нарушена</option>
                <option value="unreadable_code">Код не читается</option>
              </select>

              <label className="block text-xs font-medium text-slate-400 mt-3">Комментарий оператора:</label>
              <textarea
                rows={3}
                value={discrepancyComment}
                onChange={(e) => setDiscrepancyComment(e.target.value)}
                placeholder="Укажите детали обнаруженного расхождения..."
                className="w-full p-3 bg-slate-950 border border-slate-700 rounded-xl text-sm text-white focus:outline-none focus:border-indigo-500"
              />
            </div>

            <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
              <button
                onClick={() => setIsDiscrepancyModalOpen(false)}
                className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-bold rounded-xl"
              >
                Отмена
              </button>
              <button
                disabled={isSubmitting}
                onClick={handleRecordDiscrepancy}
                className="px-5 py-2 bg-rose-600 hover:bg-rose-500 text-white font-bold text-xs rounded-xl"
              >
                {isSubmitting ? 'Сохранение...' : 'Подтвердить расхождение'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
