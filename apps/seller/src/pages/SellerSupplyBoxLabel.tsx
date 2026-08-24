import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { QRCodeSVG } from 'qrcode.react';
import { Printer, ArrowLeft, Loader2, AlertCircle } from 'lucide-react';
import { getSellerSupply, SellerSupply, SellerSupplyBox } from '@zamk/api-client';

export const SellerSupplyBoxLabel: React.FC = () => {
  const { id, boxId } = useParams<{ id: string; boxId?: string }>();
  const [supply, setSupply] = useState<SellerSupply | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (id) {
      fetchSupply(id);
    }
  }, [id]);

  const fetchSupply = async (supplyId: string) => {
    try {
      setLoading(true);
      setError(null);
      const data = await getSellerSupply(supplyId);
      setSupply(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить данные поставки');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
        <Loader2 className="w-8 h-8 animate-spin text-gray-500" />
      </div>
    );
  }

  if (error || !supply) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
        <div className="bg-white p-8 rounded-2xl shadow-sm border border-gray-200 max-w-md w-full text-center">
          <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <h2 className="text-xl font-bold text-gray-900 mb-2">Ошибка</h2>
          <p className="text-gray-600 mb-6">{error || 'Поставка не найдена'}</p>
          <Link
            to={`/supplies/${id || ''}`}
            className="inline-flex items-center justify-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800"
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Назад к поставке
          </Link>
        </div>
      </div>
    );
  }

  const box: SellerSupplyBox | undefined =
    supply.boxes && supply.boxes.length > 0
      ? (boxId ? supply.boxes.find(b => b.id === boxId) : supply.boxes[0]) || supply.boxes[0]
      : undefined;

  const boxNumber = box?.boxNumber || `${supply.supplyNumber}-B1`;
  const qrValue = box?.qrToken || supply.qrToken || supply.supplyNumber || '';
  const boxIndex = supply.boxes && box ? supply.boxes.findIndex(b => b.id === box.id) + 1 : 1;
  const totalBoxes = supply.totalExpectedBoxes || supply.boxes?.length || 1;

  const handlePrint = () => {
    window.print();
  };

  return (
    <div className="min-h-screen bg-gray-100 py-8 px-4 sm:px-6">
      <style>{`
        @media print {
          @page {
            size: 100mm 150mm;
            margin: 0;
          }
          html, body {
            margin: 0 !important;
            padding: 0 !important;
            background: #ffffff !important;
            width: 100mm !important;
            height: 150mm !important;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
          }
          .no-print {
            display: none !important;
          }
          .print-label-wrapper {
            margin: 0 !important;
            padding: 0 !important;
            background: white !important;
            width: 100mm !important;
            height: 150mm !important;
            display: flex !important;
            justify-content: center !important;
            align-items: center !important;
          }
          .print-label-card {
            width: 100mm !important;
            height: 150mm !important;
            box-shadow: none !important;
            border: none !important;
            border-radius: 0 !important;
            padding: 8mm !important;
            page-break-after: avoid !important;
            page-break-inside: avoid !important;
          }
        }
      `}</style>

      {/* Screen Toolbar */}
      <div className="no-print max-w-xl mx-auto mb-6 flex items-center justify-between">
        <Link
          to={`/supplies/${supply.id}`}
          className="inline-flex items-center text-sm font-bold text-gray-600 hover:text-gray-900 bg-white px-4 py-2.5 rounded-xl border border-gray-200 shadow-sm transition-colors"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          Назад к поставке
        </Link>
        <button
          onClick={handlePrint}
          className="inline-flex items-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 shadow-sm transition-colors"
        >
          <Printer className="w-4 h-4 mr-2" />
          Печать этикетки
        </button>
      </div>

      {/* Label Container (Screen + Print) */}
      <div className="print-label-wrapper flex justify-center">
        <div
          className="print-label-card bg-white border-2 border-black rounded-2xl shadow-md p-6 flex flex-col justify-between"
          style={{ width: '380px', minHeight: '560px' }}
        >
          {/* Header */}
          <div className="border-b-2 border-black pb-4 text-center">
            <div className="flex items-center justify-between mb-1">
              <span className="text-2xl font-black tracking-tight text-black">ZAMK</span>
              <span className="text-xs font-bold uppercase tracking-wider bg-black text-white px-2.5 py-0.5 rounded">
                FBO СКЛАД
              </span>
            </div>
            <p className="text-xs font-medium text-gray-500">Грузовая этикетка поставки</p>
          </div>

          {/* Core Identification */}
          <div className="my-4 space-y-3">
            <div className="flex justify-between items-baseline">
              <span className="text-xs font-bold uppercase text-gray-500">Поставка:</span>
              <span className="font-mono text-xl font-black text-black">{supply.supplyNumber}</span>
            </div>

            <div className="flex justify-between items-baseline">
              <span className="text-xs font-bold uppercase text-gray-500">Грузоместо:</span>
              <span className="font-bold text-base text-black">
                {boxIndex} из {totalBoxes}
              </span>
            </div>

            <div className="flex justify-between items-baseline">
              <span className="text-xs font-bold uppercase text-gray-500">Товаров в месте:</span>
              <span className="font-bold text-base text-black">
                {supply.totalExpectedItems} шт ({supply.skuCount || supply.items?.length || 0} SKU)
              </span>
            </div>

            {supply.carrierName && (
              <div className="flex justify-between items-baseline">
                <span className="text-xs font-bold uppercase text-gray-500">Доставка:</span>
                <span className="font-medium text-sm text-black">
                  {supply.carrierName} {supply.trackingNumber ? `· ${supply.trackingNumber}` : ''}
                </span>
              </div>
            )}
          </div>

          {/* QR Code Centerpiece */}
          <div className="border-2 border-dashed border-gray-300 rounded-xl p-4 flex flex-col items-center justify-center my-2 bg-gray-50/50">
            <div className="bg-white p-2 rounded-lg shadow-sm mb-3">
              <QRCodeSVG value={qrValue} size={180} level="H" />
            </div>
            <div className="text-center">
              <p className="text-[11px] font-bold text-gray-400 uppercase tracking-widest mb-0.5">
                Идентификатор грузоместа
              </p>
              <p className="font-mono text-lg font-black tracking-wider text-black">{boxNumber}</p>
            </div>
          </div>

          {/* Footer Note */}
          <div className="border-t-2 border-black pt-3 mt-2 text-center">
            <p className="text-[11px] font-bold text-gray-700 uppercase tracking-wider">
              Склад назначения: ZAMK FBO
            </p>
            <p className="text-[10px] text-gray-400 mt-0.5">
              Отсканируйте QR при приёмке на складе ZAMK
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};
