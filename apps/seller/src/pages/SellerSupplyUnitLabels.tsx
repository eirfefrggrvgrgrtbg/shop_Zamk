import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Printer, ArrowLeft, Loader2, AlertCircle, Info } from 'lucide-react';
import { getSellerSupplyUnitLabels, SellerSupplyUnitLabelsResponse } from '@zamk/api-client';
import { Code128Barcode } from '../components/Code128Barcode';

export const SellerSupplyUnitLabels: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [data, setData] = useState<SellerSupplyUnitLabelsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<{ code?: string; message: string } | null>(null);

  useEffect(() => {
    if (id) {
      fetchUnitLabels(id);
    }
  }, [id]);

  const fetchUnitLabels = async (supplyId: string) => {
    try {
      setLoading(true);
      setError(null);
      const res = await getSellerSupplyUnitLabels(supplyId);
      setData(res);
    } catch (err: any) {
      const code = err?.response?.data?.error?.code || err?.code || '';
      let message = err?.response?.data?.error?.message || err?.message || 'Не удалось загрузить этикетки товаров';
      if (code === 'supply_unit_identity_mismatch') {
        message = 'Количество товарных этикеток не совпадает с составом поставки. Обновите страницу или обратитесь к администратору.';
      } else if (code === 'supply_not_found') {
        message = 'Поставка не найдена.';
      } else if (code === 'supply_forbidden') {
        message = 'У вас нет доступа к этой поставке.';
      }
      setError({ code, message });
    } finally {
      setLoading(false);
    }
  };

  const handlePrint = () => {
    window.print();
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex flex-col items-center justify-center p-6">
        <Loader2 className="w-8 h-8 animate-spin text-gray-500 mb-3" />
        <p className="text-sm font-medium text-gray-600">Загружаем этикетки...</p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-gray-200 max-w-md w-full text-center">
          <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <h2 className="text-xl font-bold text-gray-900 mb-2">Ошибка</h2>
          <p className="text-gray-600 mb-6">{error?.message || 'Не удалось загрузить этикетки'}</p>
          <Link
            to={`/supplies/${id || ''}`}
            className="inline-flex items-center justify-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 transition-colors"
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Назад к поставке
          </Link>
        </div>
      </div>
    );
  }

  // Legacy Supply without serialized units
  if (!data.serialized || data.totalUnits === 0 || !data.units || data.units.length === 0) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-gray-200 max-w-lg w-full text-center">
          <Info className="w-12 h-12 text-blue-500 mx-auto mb-4" />
          <h2 className="text-xl font-bold text-gray-900 mb-2">Индивидуальные этикетки недоступны</h2>
          <p className="text-gray-700 font-medium mb-2">
            Для этой поставки индивидуальные этикетки ZAMK не создавались.
          </p>
          <p className="text-sm text-gray-500 mb-6">
            Эта поставка была создана до перехода на индивидуальную маркировку товаров.
          </p>
          <Link
            to={`/supplies/${data.supplyId}`}
            className="inline-flex items-center justify-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 transition-colors"
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Назад к поставке
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-100 py-8 px-4 sm:px-6">
      <style>{`
        @media print {
          @page {
            size: 58mm 40mm;
            margin: 0;
          }
          html, body {
            margin: 0 !important;
            padding: 0 !important;
            background: #ffffff !important;
            width: 58mm !important;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
          }
          .no-print {
            display: none !important;
          }
          .unit-labels-grid {
            display: block !important;
            margin: 0 !important;
            padding: 0 !important;
          }
          .unit-label-card {
            width: 58mm !important;
            height: 40mm !important;
            max-width: 58mm !important;
            max-height: 40mm !important;
            margin: 0 !important;
            padding: 2mm 2.5mm !important;
            box-sizing: border-box !important;
            box-shadow: none !important;
            border: none !important;
            border-radius: 0 !important;
            page-break-after: always !important;
            break-after: page !important;
            page-break-inside: avoid !important;
            break-inside: avoid !important;
            display: flex !important;
            flex-direction: column !important;
            justify-content: space-between !important;
            background: #ffffff !important;
            overflow: hidden !important;
          }
          .unit-label-card:last-child {
            page-break-after: auto !important;
            break-after: auto !important;
          }
        }
      `}</style>

      {/* Screen Toolbar */}
      <div className="no-print max-w-5xl mx-auto mb-8 bg-white p-6 rounded-2xl shadow-sm border border-gray-200">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <Link
                to={`/supplies/${data.supplyId}`}
                className="inline-flex items-center text-sm font-bold text-gray-600 hover:text-gray-900 bg-gray-100 px-3 py-1.5 rounded-lg transition-colors"
              >
                <ArrowLeft className="w-4 h-4 mr-1.5" />
                Назад к поставке
              </Link>
              <h1 className="text-xl font-bold text-gray-900">Этикетки товаров</h1>
            </div>
            <p className="text-sm font-medium text-gray-500 mt-2">
              Поставка <span className="font-mono font-bold text-gray-800">{data.supplyNumber}</span> · {data.totalUnits} {data.totalUnits === 1 ? 'этикетка' : data.totalUnits < 5 ? 'этикетки' : 'этикеток'}
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={handlePrint}
              className="inline-flex items-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 shadow-sm transition-colors cursor-pointer"
            >
              <Printer className="w-4 h-4 mr-2" />
              Печать ({data.totalUnits} шт)
            </button>
          </div>
        </div>

        <div className="mt-4 pt-4 border-t border-gray-100 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs text-gray-500">
          <p>Наклейте по одной этикетке на каждую физическую единицу товара.</p>
          <p className="font-medium text-gray-400">Повторная печать не создаёт новые ZMU.</p>
        </div>
      </div>

      {/* Labels Grid (Screen Preview + Print layout) */}
      <div className="max-w-5xl mx-auto">
        <div className="unit-labels-grid grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {data.units.map((unit, idx) => {
            const variantDetails = [unit.colorName, unit.sizeName].filter(Boolean).join(' · ');

            return (
              <div
                key={unit.inventoryUnitId}
                className="unit-label-card bg-white border border-gray-300 rounded-xl p-3 shadow-sm flex flex-col justify-between"
                style={{ minHeight: '160px' }}
              >
                {/* Top Brand & Sequence */}
                <div>
                  <div className="flex items-center justify-between border-b border-black pb-0.5 mb-1">
                    <span className="text-[11px] font-black tracking-tight text-black">ZAMK</span>
                    <span className="text-[8px] font-mono text-gray-600">
                      {data.supplyNumber} · {idx + 1}/{data.totalUnits}
                    </span>
                  </div>

                  {/* Product Title */}
                  <h4 className="text-[10px] font-bold text-black truncate leading-tight" title={unit.productTitle}>
                    {unit.productTitle}
                  </h4>

                  {/* Color / Size & SKU */}
                  <div className="flex items-baseline justify-between text-[8px] text-gray-800 mt-0.5">
                    {variantDetails && <span className="font-semibold truncate max-w-[55%]">{variantDetails}</span>}
                    {unit.sellerSku && <span className="font-mono text-gray-600 truncate">Арт: {unit.sellerSku}</span>}
                  </div>

                  {/* Secondary Variant Barcode (ZMK) */}
                  {unit.variantBarcode && (
                    <div className="text-[7px] font-mono text-gray-500 mt-0.5">
                      ZMK: {unit.variantBarcode}
                    </div>
                  )}
                </div>

                {/* Primary Code128 Barcode & ZMU */}
                <div className="flex flex-col items-center justify-center my-1 pt-1 border-t border-dashed border-gray-300">
                  <div className="w-full flex justify-center overflow-hidden py-0.5">
                    <Code128Barcode value={unit.unitCode} width={1.2} height={28} className="max-w-full" />
                  </div>
                  <p className="font-mono font-black text-[9px] tracking-wider text-black mt-0.5">
                    {unit.unitCode}
                  </p>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
};
