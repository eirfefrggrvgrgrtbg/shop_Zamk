import React, { useState, useRef, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { PackageSearch, ArrowRight, XCircle, CheckCircle2, Box, Store, RefreshCw, Check, AlertTriangle } from 'lucide-react';
import { processFoundUnit, ProcessFoundUnitResponse, finalizeSupplyReceivingSession } from '@zamk/api-client/src/admin';
import { playBeepSound } from '../utils/audio';

export function AdminFreeScanner() {
  const [searchParams] = useSearchParams();
  const initialCode = (searchParams.get('q') || searchParams.get('code') || searchParams.get('unitCode') || '').trim();
  const [unitCode, setUnitCode] = useState(initialCode);
  const [isDamaged, setIsDamaged] = useState(false);
  const [loading, setLoading] = useState(false);
  const [finalizing, setFinalizing] = useState(false);
  const [isFinalized, setIsFinalized] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastResult, setLastResult] = useState<ProcessFoundUnitResponse | null>(null);
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
    const code = unitCode.trim();
    if (!code) return;

    setLoading(true);
    setError(null);
    setIsFinalized(false);

    try {
      const data = await processFoundUnit({
        unitCode: code,
        condition: isDamaged ? 'damaged' : 'ok',
      });
      setLastResult(data);
      setIsDamaged(false);
      playBeepSound('success');
    } catch (err: any) {
      if (err?.code === 'unit_not_found' || err?.code === 'UNIT_NOT_FOUND' || err?.status === 404) {
        setError('Физическая единица с таким кодом не найдена.');
      } else if (err?.code === 'unit_already_scanned' || err?.code === 'UNIT_ALREADY_SCANNED') {
        setError('Эта единица уже отсканирована в текущей сессии.');
      } else {
        setError(err?.message || 'Ошибка при обработке');
      }
      playBeepSound('error');
    } finally {
      setLoading(false);
      setUnitCode('');
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  };

  const handleFinalize = async () => {
    if (!lastResult?.receivingSessionId || finalizing) return;

    setFinalizing(true);
    setError(null);

    try {
      await finalizeSupplyReceivingSession(lastResult.receivingSessionId, {});
      setIsFinalized(true);
      playBeepSound('success');

      // Refetch physical unit to display updated warehouse state
      if (lastResult.unitCode) {
        try {
          const updated = await processFoundUnit({ unitCode: lastResult.unitCode });
          setLastResult(updated);
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
    setLastResult(null);
    setError(null);
    setIsFinalized(false);
    setUnitCode('');
    setIsDamaged(false);
    inputRef.current?.focus();
  };

  const getStatusDisplay = (action: string, status: string) => {
    switch (action) {
      case 'already_in_warehouse':
        return { label: 'Товар на складе', desc: 'Этот товар уже находится на складе.', type: 'success', icon: <CheckCircle2 className="w-5 h-5" /> };
      case 'already_damaged':
        return { label: 'Брак', desc: 'Эта единица уже отмечена как брак.', type: 'error', icon: <XCircle className="w-5 h-5" /> };
      case 'already_shipped':
        return { label: 'Отгружен', desc: 'Эта единица уже отгружена.', type: 'neutral', icon: <ArrowRight className="w-5 h-5" /> };
      case 'written_off':
        return { label: 'Списан', desc: 'Эта единица списана.', type: 'neutral', icon: <XCircle className="w-5 h-5" /> };
      default:
        return { label: `Статус: ${status}`, desc: '', type: 'neutral', icon: null };
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
            <p className="text-sm text-slate-500">Не нужно искать поставку или коробку — система определит их автоматически.</p>
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
                  disabled={loading || finalizing}
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
                disabled={!unitCode.trim() || loading || finalizing}
                className="px-8 py-4 bg-indigo-600 hover:bg-indigo-700 text-white font-medium rounded-lg disabled:opacity-50 transition-colors"
              >
                Принять
              </button>
            </div>

            <div className="flex items-center gap-2">
              <label className="inline-flex items-center gap-2 text-sm font-medium text-slate-700 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={isDamaged}
                  onChange={(e) => setIsDamaged(e.target.checked)}
                  className="w-4 h-4 text-red-600 rounded border-slate-300 focus:ring-red-500"
                  disabled={loading || finalizing}
                />
                <span className={isDamaged ? 'text-red-700 font-semibold' : 'text-slate-600'}>
                  Найденный товар — брак
                </span>
              </label>
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

      {lastResult && (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
          <div className="p-6">
            <div className="flex justify-between items-start mb-6">
              <div>
                <h2 className="text-2xl font-bold text-slate-900 mb-1">{lastResult.unitCode}</h2>
                <div className="text-slate-600 font-medium">{lastResult.productTitle || 'Товарная единица'}</div>
              </div>
              <button onClick={handleReset} className="text-sm text-slate-500 hover:text-slate-700">Очистить</button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
              <div className="bg-slate-50 rounded-lg p-4 border border-slate-100">
                <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Детали товара</h3>
                <dl className="space-y-2 text-sm">
                  <div className="flex justify-between"><dt className="text-slate-500">SKU продавца</dt><dd className="font-mono text-slate-900">{lastResult.sellerSku || '—'}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Штрихкод</dt><dd className="font-mono text-slate-900">{lastResult.variantBarcode || '—'}</dd></div>
                  {(lastResult.colorName || lastResult.sizeName) && (
                    <div className="flex justify-between"><dt className="text-slate-500">Вариант</dt><dd className="text-slate-900">{[lastResult.colorName, lastResult.sizeName].filter(Boolean).join(' / ')}</dd></div>
                  )}
                </dl>
              </div>

              <div className="bg-slate-50 rounded-lg p-4 border border-slate-100">
                <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Происхождение</h3>
                <dl className="space-y-2 text-sm">
                  <div className="flex justify-between items-center"><dt className="text-slate-500">Поставка</dt><dd className="font-medium text-slate-900 flex items-center gap-1.5"><Box className="w-4 h-4 text-slate-400" /> {lastResult.supplyNumber}</dd></div>
                  <div className="flex justify-between items-center"><dt className="text-slate-500">Продавец</dt><dd className="font-medium text-slate-900 flex items-center gap-1.5"><Store className="w-4 h-4 text-slate-400" /> {lastResult.sellerName || '—'}</dd></div>
                  {lastResult.boxNumber && (
                    <div className="flex justify-between"><dt className="text-slate-500">Коробка</dt><dd className="text-slate-900">{lastResult.boxNumber}</dd></div>
                  )}
                </dl>
              </div>
            </div>

            {isFinalized && (
              <div className="mb-6 p-4 bg-emerald-100 border border-emerald-300 text-emerald-800 rounded-lg flex items-center gap-2 font-medium">
                <Check className="w-5 h-5 flex-shrink-0" />
                Доприёмка завершена. Товар оприходован на склад.
              </div>
            )}

            {lastResult.unitStatus === 'expected' ? (
              <div className="space-y-4">
                <div className="bg-emerald-50 border border-emerald-200 text-emerald-900 rounded-xl p-5">
                  <div className="flex items-start gap-4">
                    <div className="mt-0.5 text-emerald-600"><CheckCircle2 className="w-6 h-6" /></div>
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <h4 className="font-bold text-lg">Единица добавлена в доприёмку</h4>
                        {lastResult.condition === 'damaged' && (
                          <span className="px-2 py-0.5 text-xs font-semibold bg-red-100 text-red-800 rounded">Брак</span>
                        )}
                      </div>

                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mt-4 pt-3 border-t border-emerald-200/60 text-sm">
                        <div>
                          <div className="text-emerald-700 text-xs uppercase font-semibold">Поставка</div>
                          <div className="font-mono font-medium text-emerald-950 mt-0.5">{lastResult.supplyNumber}</div>
                        </div>
                        <div>
                          <div className="text-emerald-700 text-xs uppercase font-semibold">Принято в доприёмке</div>
                          <div className="font-semibold text-emerald-950 mt-0.5">{lastResult.sessionScanned} / {lastResult.sessionExpected}</div>
                        </div>
                        <div>
                          <div className="text-emerald-700 text-xs uppercase font-semibold">Осталось</div>
                          <div className="font-semibold text-emerald-950 mt-0.5">{lastResult.sessionRemaining}</div>
                        </div>
                        <div>
                          <div className="text-emerald-700 text-xs uppercase font-semibold">Состояние</div>
                          <div className="font-medium text-emerald-950 mt-0.5">{lastResult.condition === 'damaged' ? 'Брак' : 'Годен'}</div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                {!isFinalized && lastResult.sessionRemaining === 0 && (
                  <div className="p-4 bg-amber-50 border border-amber-200 text-amber-900 rounded-xl flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                    <div className="flex items-center gap-2.5">
                      <AlertTriangle className="w-5 h-5 text-amber-600 flex-shrink-0" />
                      <span className="font-medium text-sm">Все найденные единицы этой доприёмки отсканированы.</span>
                    </div>
                    <button
                      onClick={handleFinalize}
                      disabled={finalizing}
                      className="px-4 py-2 bg-amber-600 hover:bg-amber-700 text-white font-medium text-sm rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
                    >
                      {finalizing && <RefreshCw className="w-4 h-4 animate-spin" />}
                      Завершить доприёмку
                    </button>
                  </div>
                )}
              </div>
            ) : (
              (() => {
                const display = getStatusDisplay(lastResult.recommendedNextAction, lastResult.unitStatus);
                let colors = 'bg-slate-100 text-slate-800 border-slate-200';
                if (display.type === 'success') colors = 'bg-emerald-50 text-emerald-800 border-emerald-200';
                if (display.type === 'error') colors = 'bg-red-50 text-red-800 border-red-200';
                if (display.type === 'info') colors = 'bg-amber-50 text-amber-800 border-amber-200';

                return (
                  <div className={`rounded-xl border p-5 ${colors}`}>
                    <div className="flex items-start gap-4">
                      <div className="mt-0.5">{display.icon}</div>
                      <div className="flex-1">
                        <h4 className="font-bold text-lg mb-1">{display.label}</h4>
                        {display.desc && <p className="opacity-90 text-sm">{display.desc}</p>}
                        <p className="opacity-75 text-xs mt-2 font-medium">Поставка: {lastResult.supplyNumber}</p>
                      </div>
                    </div>
                  </div>
                );
              })()
            )}
          </div>
        </div>
      )}
    </div>
  );
}
