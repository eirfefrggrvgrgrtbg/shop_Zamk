import React, { useState, useRef, useEffect } from 'react';
import { Truck, Box, CheckCircle2, AlertTriangle, ArrowRight, RefreshCw, AlertCircle } from 'lucide-react';
import { startSupplyReceivingSession, recordSupplyReceivingScan, finalizeSupplyReceivingSession } from '@zamk/api-client/src/admin';
import type { SupplyReceivingSession } from '@zamk/api-client/src/types';

function playBeepSound(type: 'success' | 'error' = 'success') {
  try {
    const ctx = new (window.AudioContext || (window as any).webkitAudioContext)();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);

    if (type === 'success') {
      osc.frequency.setValueAtTime(880, ctx.currentTime);
      gain.gain.setValueAtTime(0.15, ctx.currentTime);
      osc.start();
      osc.stop(ctx.currentTime + 0.12);
    } else {
      osc.frequency.setValueAtTime(220, ctx.currentTime);
      gain.gain.setValueAtTime(0.25, ctx.currentTime);
      osc.start();
      osc.stop(ctx.currentTime + 0.3);
    }
  } catch (_) {}
}

export function AdminSupplyReceiving() {
  const [session, setSession] = useState<SupplyReceivingSession | null>(null);
  const [qrInput, setQrInput] = useState('');
  const [barcodeInput, setBarcodeInput] = useState('');
  const [isDamagedScan, setIsDamagedScan] = useState(false);
  
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const [isFinalized, setIsFinalized] = useState(false);

  const qrRef = useRef<HTMLInputElement>(null);
  const barcodeRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!session && !isFinalized) {
      qrRef.current?.focus();
    } else if (session && !isFinalized) {
      barcodeRef.current?.focus();
    }
  }, [session, isFinalized]);

  const handleStartSession = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!qrInput.trim()) return;

    try {
      setLoading(true);
      setError(null);
      setIsFinalized(false);
      const data = await startSupplyReceivingSession(qrInput.trim());
      setSession(data);
      playBeepSound('success');
    } catch (err: any) {
      setError(err.message || 'Ошибка старта сессии приемки');
      playBeepSound('error');
    } finally {
      setLoading(false);
      setQrInput('');
    }
  };

  const handleScanItem = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!barcodeInput.trim() || !session || !session.items) return;

    try {
      setLoading(true);
      setError(null);
      
      const matchedItem = session.items.find(i => 
        (i.barcode && i.barcode === barcodeInput.trim()) || 
        i.sku === barcodeInput.trim()
      );
      if (!matchedItem || !matchedItem.variantId) {
        throw new Error('Штрихкод не найден в данной поставке');
      }

      await recordSupplyReceivingScan(session.id, {
        variantId: matchedItem.variantId,
        quantity: 1, 
        isDamage: isDamagedScan
      });
      
      // Update local state to reflect the scan
      setSession(prev => {
        if (!prev || !prev.items) return prev;
        const newItems = prev.items.map(i => {
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
    } catch (err: any) {
      setError(err.message || 'Ошибка сканирования товара');
      playBeepSound('error');
    } finally {
      setLoading(false);
      setBarcodeInput('');
      setTimeout(() => barcodeRef.current?.focus(), 50);
    }
  };

  const handleFinalize = async () => {
    if (!session) return;
    try {
      setLoading(true);
      setError(null);
      
      await finalizeSupplyReceivingSession(session.id, {});
      
      setIsFinalized(true);
      playBeepSound('success');
    } catch (err: any) {
      setError(err.message || 'Ошибка завершения приемки');
      playBeepSound('error');
    } finally {
      setLoading(false);
    }
  };

  const resetFlow = () => {
    setSession(null);
    setIsFinalized(false);
  };

  const totalScanned = session?.items?.reduce((acc, i) => acc + i.scannedQuantity + i.damagedQuantity, 0) || 0;
  const hasDiscrepancy = session?.items?.some(i => i.expectedQuantity !== i.scannedQuantity + i.damagedQuantity);

  return (
    <div className="space-y-6 pb-20">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Приемка поставок (Supplies)</h1>
          <p className="text-sm text-slate-400 mt-1">Сканирование QR поставок и штрихкодов товаров</p>
        </div>
      </div>

      {error && (
        <div className="mx-4 sm:mx-6 bg-rose-500/10 border border-rose-500/20 rounded-lg p-4 flex items-center">
          <AlertTriangle className="h-5 w-5 text-rose-500 mr-3 flex-shrink-0" />
          <span className="text-rose-200 text-sm">{error}</span>
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
              <p className="text-slate-400">Сессия <span className="font-mono text-emerald-400">{session.id}</span> успешно закрыта.</p>
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
                          <td className="py-4 px-6 font-mono text-white text-sm">
                            {item.barcode || item.sku}
                          </td>
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
        <div className="mx-4 sm:mx-6 bg-slate-800 border border-slate-700 rounded-xl p-8 max-w-2xl text-center shadow-lg">
          <Truck className="mx-auto h-16 w-16 text-slate-500 mb-4" />
          <h2 className="text-xl font-medium text-white mb-2">Начать приемку</h2>
          <p className="text-slate-400 mb-6 text-sm">Отсканируйте QR-код поставки (SUP-XXXXX) или штрихкод коробки для старта сессии.</p>
          
          <form onSubmit={handleStartSession} className="max-w-md mx-auto relative">
            <input
              ref={qrRef}
              type="text"
              value={qrInput}
              onChange={(e) => setQrInput(e.target.value)}
              placeholder="Скан QR поставки..."
              className="w-full bg-slate-900 border border-slate-600 rounded-lg pl-4 pr-12 py-3 text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-inner"
              disabled={loading}
              autoFocus
            />
            <button
              type="submit"
              disabled={loading || !qrInput.trim()}
              className="absolute right-2 top-2 bottom-2 bg-blue-600 hover:bg-blue-500 text-white rounded-md px-3 transition-colors disabled:opacity-50"
            >
              <ArrowRight className="h-5 w-5" />
            </button>
          </form>
        </div>
      ) : (
        <div className="mx-4 sm:mx-6 grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-6">
            <div className="bg-slate-800 border border-slate-700 rounded-xl p-6 shadow-lg">
              <div className="flex justify-between items-center mb-6">
                <div>
                  <h2 className="text-xl font-medium text-white">Сканирование товаров</h2>
                  <p className="text-slate-400 text-sm mt-1">Поставка: <span className="font-mono font-bold text-blue-400 px-2 py-0.5 bg-blue-500/10 rounded">{session.supplyId}</span></p>
                </div>
                <button
                  onClick={resetFlow}
                  className="text-slate-400 hover:text-white flex items-center text-sm px-3 py-1.5 border border-slate-700 rounded-md hover:bg-slate-700 transition-colors"
                >
                  <RefreshCw className="h-4 w-4 mr-2" />
                  Сбросить
                </button>
              </div>

              <form onSubmit={handleScanItem} className="relative mb-8">
                <input
                  ref={barcodeRef}
                  type="text"
                  value={barcodeInput}
                  onChange={(e) => setBarcodeInput(e.target.value)}
                  placeholder="Скан штрихкода товара..."
                  className={`w-full bg-slate-900 border ${isDamagedScan ? 'border-rose-500/50 focus:ring-rose-500 shadow-[0_0_15px_rgba(244,63,94,0.1)]' : 'border-blue-500/50 focus:ring-blue-500 shadow-[0_0_15px_rgba(59,130,246,0.1)]'} rounded-lg pl-4 pr-16 py-4 text-lg text-white placeholder-slate-500 focus:outline-none focus:ring-2`}
                  disabled={loading}
                />
                <button
                  type="submit"
                  disabled={loading || !barcodeInput.trim()}
                  className={`absolute right-2 top-2 bottom-2 ${isDamagedScan ? 'bg-rose-600 hover:bg-rose-500' : 'bg-blue-600 hover:bg-blue-500'} text-white rounded-md px-4 transition-colors disabled:opacity-50`}
                >
                  <ArrowRight className="h-6 w-6" />
                </button>
              </form>

              <div className="flex items-center mb-8 bg-slate-900/50 p-3 rounded-lg border border-slate-700">
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
                  <span className="font-medium">Отметить следующий скан как БРАК</span>
                </label>
              </div>

              <h3 className="text-sm font-semibold text-slate-300 uppercase tracking-wider mb-4 flex items-center">
                <Box className="w-4 h-4 mr-2" /> Ожидаемые товары
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
                        <th className="py-3 px-4 text-xs font-semibold text-slate-400 uppercase tracking-wider">Штрихкод</th>
                        <th className="py-3 px-4 text-xs font-semibold text-slate-400 uppercase tracking-wider text-right">План</th>
                        <th className="py-3 px-4 text-xs font-semibold text-emerald-400 uppercase tracking-wider text-right">Ок</th>
                        <th className="py-3 px-4 text-xs font-semibold text-rose-400 uppercase tracking-wider text-right">Брак</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {session.items.map((item, idx) => (
                        <tr key={idx} className="hover:bg-slate-800/50 transition-colors">
                          <td className="py-4 px-4 text-sm font-mono text-white font-medium">{item.barcode || item.sku}</td>
                          <td className="py-4 px-4 text-sm font-medium text-slate-300 text-right">{item.expectedQuantity}</td>
                          <td className="py-4 px-4 text-lg font-bold text-emerald-400 text-right">{item.scannedQuantity}</td>
                          <td className="py-4 px-4 text-lg font-bold text-rose-400 text-right">{item.damagedQuantity}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>

          <div className="lg:col-span-1 space-y-6">
            <div className="bg-slate-800 border border-slate-700 rounded-xl p-6 shadow-lg sticky top-6">
              <h3 className="text-lg font-medium text-white mb-2">Завершение приемки</h3>
              <p className="text-sm text-slate-400 mb-6">
                После того как все товары из поставки отсканированы, нажмите завершить.
              </p>

              <div className="mb-6 p-4 bg-slate-900/50 rounded-lg border border-slate-700/50">
                <div className="flex justify-between items-center mb-2">
                  <span className="text-sm text-slate-400">Всего отсканировано:</span>
                  <span className="text-xl font-bold text-white">
                    {totalScanned}
                  </span>
                </div>
                <div className="flex justify-between items-center text-xs text-slate-500 mt-2 border-t border-slate-800 pt-2">
                  <span>Ожидалось всего:</span>
                  <span>{session.items?.reduce((acc, i) => acc + i.expectedQuantity, 0) || 0}</span>
                </div>
              </div>

              <button
                onClick={handleFinalize}
                disabled={loading || totalScanned === 0}
                className="w-full bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-4 px-4 rounded-xl flex items-center justify-center transition-colors shadow-lg shadow-emerald-900/20 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <CheckCircle2 className="w-5 h-5 mr-2" />
                Завершить приёмку
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

