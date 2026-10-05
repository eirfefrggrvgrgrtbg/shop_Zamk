import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { X, ExternalLink, RotateCcw, Loader2, AlertCircle, ArrowLeft, Package } from 'lucide-react';
import {
  getAdminReturn,
  getReturnStatusLabel,
  getReturnReasonLabel,
  type AdminReturn,
} from '../../api/adminReturns';

interface ReturnQuickViewProps {
  returnId: string;
  onClose: () => void;
  onBack?: () => void;
  onContextClick?: (type: 'ORDER' | 'RETURN' | 'PRODUCT', id: string) => void;
}

export function ReturnQuickView({ returnId, onClose, onBack, onContextClick }: ReturnQuickViewProps) {
  const [returnObj, setReturnObj] = useState<AdminReturn | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    setError(null);
    getAdminReturn(returnId)
      .then((data) => {
        if (mounted) {
          setReturnObj(data);
          setLoading(false);
        }
      })
      .catch((e: unknown) => {
        if (mounted) {
          const message = e instanceof Error ? e.message : 'Ошибка загрузки возврата';
          setError(message);
          setLoading(false);
        }
      });
    return () => {
      mounted = false;
    };
  }, [returnId]);

  return (
    <div
      data-testid="return-quick-view"
      className="absolute top-0 right-0 bottom-0 w-full sm:w-96 md:w-[410px] bg-white shadow-2xl z-30 flex flex-col border-l border-gray-200 transform transition-transform duration-300 translate-x-0"
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100 bg-gray-50/80 shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          {onBack && (
            <button
              type="button"
              onClick={onBack}
              title="Назад"
              aria-label="Назад"
              className="p-1.5 -ml-1 text-gray-400 hover:text-gray-600 hover:bg-gray-200/50 rounded-md transition-colors mr-1 cursor-pointer"
            >
              <ArrowLeft className="w-4 h-4" />
            </button>
          )}
          <RotateCcw className="w-4 h-4 text-purple-600 shrink-0" />
          <h3 className="font-semibold text-gray-900 text-sm truncate">
            Возврат {returnObj?.orderNumber ? `#${returnObj.orderNumber}` : returnId.slice(0, 8)}
          </h3>
        </div>
        <button
          type="button"
          onClick={onClose}
          title="Закрыть инспектор"
          aria-label="Закрыть"
          className="p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-200/50 rounded-md transition-colors cursor-pointer shrink-0"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 select-none">
        {loading ? (
          <div className="flex flex-col items-center justify-center h-48 text-gray-400 gap-3">
            <Loader2 className="w-6 h-6 animate-spin text-gray-300" />
            <span className="text-xs font-medium">Загрузка данных возврата...</span>
          </div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center h-48 text-red-500 gap-3 text-center">
            <AlertCircle className="w-6 h-6" />
            <span className="text-xs font-medium px-4">{error}</span>
          </div>
        ) : returnObj ? (
          <div className="space-y-6">
            {/* Status & Reason */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-500">Статус</span>
                <span className="text-xs font-semibold px-2 py-0.5 bg-purple-50 text-purple-800 rounded">
                  {getReturnStatusLabel(returnObj.status)}
                </span>
              </div>
              {returnObj.reason && (
                <div className="flex items-center justify-between">
                  <span className="text-xs text-gray-500">Причина</span>
                  <span className="text-xs font-medium text-gray-900">
                    {getReturnReasonLabel(returnObj.reason)}
                  </span>
                </div>
              )}
            </div>

            {/* Linked Order Drill-down */}
            {returnObj.orderId && (
              <div>
                <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                  Связанный заказ
                </h4>
                <button
                  type="button"
                  onClick={() => onContextClick?.('ORDER', returnObj.orderId)}
                  className="w-full flex items-center justify-between p-2.5 rounded-lg border border-gray-100 bg-gray-50/60 hover:bg-gray-100/80 hover:border-gray-200 transition-colors text-left group cursor-pointer"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-white border border-gray-200/80 flex items-center justify-center shrink-0 group-hover:border-blue-300 transition-colors">
                      <Package className="w-3.5 h-3.5 text-blue-600" />
                    </div>
                    <div className="min-w-0">
                      <span className="block text-xs font-semibold text-gray-900 group-hover:text-blue-600 truncate">
                        Заказ {returnObj.orderNumber || returnObj.orderId.slice(0, 8)}
                      </span>
                      <span className="block text-[10px] text-gray-500 mt-0.5">
                        Открыть детали заказа
                      </span>
                    </div>
                  </div>
                </button>
              </div>
            )}

            {/* Customer Details */}
            {(returnObj.customerEmail || returnObj.customerName || returnObj.customerPhone) && (
              <div>
                <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                  Покупатель
                </h4>
                <div className="bg-gray-50/70 p-3 rounded-lg border border-gray-100 space-y-1">
                  {returnObj.customerName && (
                    <p className="text-xs font-medium text-gray-900">{returnObj.customerName}</p>
                  )}
                  {returnObj.customerEmail && (
                    <p className="text-xs text-gray-600 font-mono truncate">{returnObj.customerEmail}</p>
                  )}
                  {returnObj.customerPhone && (
                    <p className="text-xs text-gray-500">{returnObj.customerPhone}</p>
                  )}
                </div>
              </div>
            )}

            {/* Items */}
            {returnObj.items && returnObj.items.length > 0 && (
              <div>
                <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                  Позиции возврата ({returnObj.items.length})
                </h4>
                <div className="space-y-2">
                  {returnObj.items.map((item) => (
                    <div key={item.id} className="flex gap-3 items-start p-2.5 border border-gray-100 rounded-lg bg-white">
                      {item.productImageUrl ? (
                        <img src={item.productImageUrl} alt={item.productTitle || 'Товар'} className="w-10 h-10 object-cover rounded bg-gray-50 shrink-0" />
                      ) : (
                        <div className="w-10 h-10 rounded bg-gray-100 flex items-center justify-center shrink-0">
                          <Package className="w-5 h-5 text-gray-400" />
                        </div>
                      )}
                      <div className="flex-1 min-w-0">
                        <p className="text-xs font-medium text-gray-900 line-clamp-2 leading-snug">{item.productTitle || 'Товар'}</p>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-[11px] text-gray-500">{item.quantity} шт.</span>
                          {item.priceCents ? (
                            <span className="text-[11px] font-semibold text-gray-700">{(item.priceCents / 100).toFixed(0)} ₽</span>
                          ) : null}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Comment */}
            {returnObj.comment && (
              <div>
                <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                  Комментарий покупателя
                </h4>
                <div className="bg-gray-50/70 p-3 rounded-lg border border-gray-100">
                  <p className="text-xs text-gray-600 leading-relaxed">{returnObj.comment}</p>
                </div>
              </div>
            )}
          </div>
        ) : null}
      </div>

      {/* Footer / Canonical Escape Hatch */}
      <div className="p-4 border-t border-gray-200 bg-white shrink-0">
        <Link
          to={`/returns?id=${returnId}`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center justify-center gap-2 w-full py-2 px-4 bg-gray-900 hover:bg-gray-800 text-white rounded-lg text-xs font-semibold transition-colors"
        >
          <span>Открыть полный возврат</span>
          <ExternalLink className="w-3.5 h-3.5" />
        </Link>
      </div>
    </div>
  );
}
