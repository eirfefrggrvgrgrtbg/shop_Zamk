import React, { useState, useRef, useEffect } from 'react';
import { PackageSearch, ArrowRight, XCircle, CheckCircle2, Box, Store, RefreshCw } from 'lucide-react';
import { resolvePhysicalUnit, ResolvedPhysicalUnit } from '@zamk/api-client/src/admin';
import { useNavigate } from 'react-router-dom';
import { playBeepSound } from '../utils/audio';

export function AdminFreeScanner() {
  const [unitCode, setUnitCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resolvedUnit, setResolvedUnit] = useState<ResolvedPhysicalUnit | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleScan = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = unitCode.trim();
    if (!code) return;

    setLoading(true);
    setError(null);
    setResolvedUnit(null);

    try {
      const data = await resolvePhysicalUnit(code);
      setResolvedUnit(data);
      playBeepSound('success');
    } catch (err: any) {
      if (err.response?.data?.error === 'unit_not_found') {
        setError('Физическая единица с таким кодом не найдена.');
      } else {
        setError(err.response?.data?.message || err.message || 'Ошибка при поиске');
      }
      playBeepSound('error');
    } finally {
      setLoading(false);
      setUnitCode('');
      inputRef.current?.focus();
    }
  };

  const handleReset = () => {
    setResolvedUnit(null);
    setError(null);
    setUnitCode('');
    inputRef.current?.focus();
  };

  const getStatusDisplay = (action: string, status: string) => {
    switch (action) {
      case 'additional_receiving':
        return { label: 'Найдена недостающая единица', desc: 'Можно принять через доприёмку.', type: 'info', icon: <PackageSearch className="w-5 h-5" /> };
      case 'continue_receiving':
        return { label: 'Товар в процессе приёмки', desc: 'Приёмка поставки еще не завершена.', type: 'info', icon: <PackageSearch className="w-5 h-5" /> };
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
            <p className="text-sm text-slate-500">Узнайте информацию о товаре, отсканировав маркер ZMU</p>
          </div>
        </div>
        
        <div className="p-6">
          <form onSubmit={handleScan} className="flex gap-3">
            <div className="relative flex-1">
              <input
                ref={inputRef}
                type="text"
                value={unitCode}
                onChange={(e) => setUnitCode(e.target.value.toUpperCase())}
                placeholder="Сканируйте ZMU..."
                className="w-full pl-4 pr-12 py-4 text-xl font-mono uppercase bg-slate-50 border border-slate-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 transition-shadow"
                disabled={loading}
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
              disabled={!unitCode.trim() || loading}
              className="px-8 py-4 bg-indigo-600 hover:bg-indigo-700 text-white font-medium rounded-lg disabled:opacity-50 transition-colors"
            >
              Найти
            </button>
          </form>
          {error && (
            <div className="mt-4 p-4 bg-red-50 text-red-700 rounded-lg flex items-center gap-2">
              <XCircle className="w-5 h-5" />
              {error}
            </div>
          )}
        </div>
      </div>

      {resolvedUnit && (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
          <div className="p-6">
            <div className="flex justify-between items-start mb-6">
              <div>
                <h2 className="text-2xl font-bold text-slate-900 mb-1">{resolvedUnit.unitCode}</h2>
                <div className="text-slate-600 font-medium">{resolvedUnit.product.title}</div>
              </div>
              <button onClick={handleReset} className="text-sm text-slate-500 hover:text-slate-700">Очистить</button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
              <div className="bg-slate-50 rounded-lg p-4 border border-slate-100">
                <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Детали товара</h3>
                <dl className="space-y-2 text-sm">
                  <div className="flex justify-between"><dt className="text-slate-500">SKU продавца</dt><dd className="font-mono text-slate-900">{resolvedUnit.variant.sellerSku || '—'}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Штрихкод</dt><dd className="font-mono text-slate-900">{resolvedUnit.variant.barcode || '—'}</dd></div>
                  {(resolvedUnit.variant.color || resolvedUnit.variant.size) && (
                    <div className="flex justify-between"><dt className="text-slate-500">Вариант</dt><dd className="text-slate-900">{[resolvedUnit.variant.color, resolvedUnit.variant.size].filter(Boolean).join(' / ')}</dd></div>
                  )}
                </dl>
              </div>

              <div className="bg-slate-50 rounded-lg p-4 border border-slate-100">
                <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">Происхождение</h3>
                <dl className="space-y-2 text-sm">
                  <div className="flex justify-between items-center"><dt className="text-slate-500">Поставка</dt><dd className="font-medium text-slate-900 flex items-center gap-1.5"><Box className="w-4 h-4 text-slate-400" /> {resolvedUnit.origin.supplyNumber}</dd></div>
                  <div className="flex justify-between items-center"><dt className="text-slate-500">Продавец</dt><dd className="font-medium text-slate-900 flex items-center gap-1.5"><Store className="w-4 h-4 text-slate-400" /> {resolvedUnit.origin.sellerName || '—'}</dd></div>
                  {resolvedUnit.origin.boxNumber && (
                    <div className="flex justify-between"><dt className="text-slate-500">Коробка</dt><dd className="text-slate-900">{resolvedUnit.origin.boxNumber}</dd></div>
                  )}
                </dl>
              </div>
            </div>

            {(() => {
              const display = getStatusDisplay(resolvedUnit.recommendedAction, resolvedUnit.unitStatus);
              const isActionable = resolvedUnit.recommendedAction === 'additional_receiving' || resolvedUnit.recommendedAction === 'continue_receiving';
              
              let colors = 'bg-slate-100 text-slate-800 border-slate-200';
              if (display.type === 'success') colors = 'bg-emerald-50 text-emerald-800 border-emerald-200';
              if (display.type === 'error') colors = 'bg-red-50 text-red-800 border-red-200';
              if (display.type === 'info') colors = 'bg-amber-50 text-amber-800 border-amber-200';

              return (
                <div className={`rounded-xl border p-5 ${colors}`}>
                  <div className="flex items-start gap-4">
                    <div className={`mt-0.5 ${display.type === 'info' ? 'text-amber-600' : ''}`}>{display.icon}</div>
                    <div className="flex-1">
                      <h4 className="font-bold text-lg mb-1">{display.label}</h4>
                      {display.desc && <p className="opacity-90">{display.desc}</p>}
                      <p className="opacity-75 text-sm mt-1 font-medium">Поставка: {resolvedUnit.origin.supplyNumber}</p>
                      
                      {isActionable && (
                        <div className="mt-4 pt-4 border-t border-black/10">
                          <button
                            onClick={() => navigate(`/supplies/receiving?qr=${encodeURIComponent(resolvedUnit.origin.supplyNumber)}`)}
                            className="bg-white/90 hover:bg-white text-slate-900 font-semibold py-2 px-4 rounded-lg shadow-sm transition-colors text-sm"
                          >
                            Перейти к приёмке {resolvedUnit.origin.supplyNumber}
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              );
            })()}
          </div>
        </div>
      )}
    </div>
  );
}
