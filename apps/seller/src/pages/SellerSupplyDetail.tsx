import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, Printer, CheckCircle, Truck, PackageCheck, FileWarning, HelpCircle, AlertCircle } from 'lucide-react';
import { getSellerSupply, shipSellerSupply } from '@zamk/api-client/src/seller';
import type { SellerSupply } from '@zamk/api-client/src/types';
import { QRCodeSVG } from 'qrcode.react';

export function SellerSupplyDetail() {
  const { id } = useParams<{ id: string }>();
  const [supply, setSupply] = useState<SellerSupply | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchSupply();
  }, [id]);

  const fetchSupply = async () => {
    try {
      setLoading(true);
      const data = await getSellerSupply(id!);
      setSupply(data);
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки поставки');
    } finally {
      setLoading(false);
    }
  };

  const handleMarkShipped = async () => {
    try {
      setSubmitting(true);
      setActionError(null);
      const updated = await shipSellerSupply(id!);
      if (updated && updated.status) {
        setSupply(updated);
      } else {
        await fetchSupply();
      }
      setShowConfirmModal(false);
    } catch (err: any) {
      setActionError(err.message || 'Не удалось подтвердить отправку');
    } finally {
      setSubmitting(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'draft': return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">Черновик</span>;
      case 'ready_to_ship': return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">Готова к отправке</span>;
      case 'shipped_by_seller': return <span className="px-3 py-1 bg-blue-100 text-blue-800 rounded-full text-xs font-bold uppercase tracking-wide">В пути</span>;
      case 'arrived_at_zamk': return <span className="px-3 py-1 bg-blue-100 text-blue-800 rounded-full text-xs font-bold uppercase tracking-wide">Прибыла в ZAMK</span>;
      case 'receiving': return <span className="px-3 py-1 bg-yellow-100 text-yellow-800 rounded-full text-xs font-bold uppercase tracking-wide">Приёмка</span>;
      case 'completed': return <span className="px-3 py-1 bg-green-100 text-green-800 rounded-full text-xs font-bold uppercase tracking-wide">Принята</span>;
      case 'completed_with_discrepancies': return <span className="px-3 py-1 bg-orange-100 text-orange-800 rounded-full text-xs font-bold uppercase tracking-wide">Принята с расхождениями</span>;
      case 'cancelled': return <span className="px-3 py-1 bg-red-100 text-red-800 rounded-full text-xs font-bold uppercase tracking-wide">Отменена</span>;
      default: return <span className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-bold uppercase tracking-wide">{status}</span>;
    }
  };

  if (loading) {
    return <div className="p-8 flex justify-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div></div>;
  }

  if (error || !supply) {
    return <div className="p-8 text-red-500 font-medium">Ошибка: {error}</div>;
  }

  const isCompleted = supply.status === 'completed' || supply.status === 'completed_with_discrepancies';

  // Find timeline progression
  const states = ['ready_to_ship', 'shipped_by_seller', 'arrived_at_zamk', 'receiving', 'completed'];
  let currentStateIndex = states.indexOf(supply.status);
  if (supply.status === 'completed_with_discrepancies') currentStateIndex = states.indexOf('completed');

  const box = supply.boxes?.[0];
  const hasBoxQR = Boolean(box?.qrToken);
  const qrValue = (hasBoxQR ? box?.qrToken : (supply.qrToken || supply.supplyNumber)) || '';
  const qrLabel = hasBoxQR ? 'QR грузоместа' : 'QR поставки';
  const boxCode = box?.boxNumber || supply.supplyNumber;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-8">
        <Link to="/supplies" className="inline-flex items-center text-sm font-medium text-gray-500 hover:text-black mb-4">
          <ArrowLeft className="w-4 h-4 mr-1" /> К списку поставок
        </Link>
        <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div>
            <h1 className="text-3xl font-black text-gray-900 tracking-tight flex items-center gap-4">
              {supply.supplyNumber || 'SUP-...'}
              {getStatusBadge(supply.status)}
            </h1>
            <p className="mt-2 text-sm text-gray-500 font-medium">
              Создана {new Date(supply.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
            </p>
          </div>
          {supply.status === 'ready_to_ship' && (
            <button
              onClick={() => setShowConfirmModal(true)}
              className="px-6 py-3 bg-black text-white rounded-xl font-bold hover:bg-gray-800 transition-colors shadow-sm"
            >
              Передал перевозчику
            </button>
          )}
        </div>
      </div>

      {/* Newly Created Hero Message */}
      {supply.status === 'ready_to_ship' && (
        <div className="bg-blue-50 border border-blue-100 rounded-2xl p-6 mb-8 flex items-start">
          <CheckCircle className="w-6 h-6 text-blue-600 mr-4 flex-shrink-0 mt-0.5" />
          <div>
            <h3 className="text-lg font-bold text-blue-900">Поставка создана</h3>
            <p className="text-blue-800 mt-1">Подготовьте грузоместо, распечатайте этикетку и передайте заказ транспортной компании.</p>
          </div>
        </div>
      )}

      {/* Action Error State */}
      {actionError && (
        <div className="mb-8 bg-red-50 p-4 rounded-xl flex items-center border border-red-100">
          <AlertCircle className="h-5 w-5 text-red-500 mr-3 flex-shrink-0" />
          <span className="text-red-700 font-medium">{actionError}</span>
        </div>
      )}

      {/* Completed Summary Block */}
      {isCompleted && (
        <div className={`rounded-2xl p-6 mb-8 border ${supply.status === 'completed' ? 'bg-green-50 border-green-100' : 'bg-orange-50 border-orange-100'}`}>
          <div className="flex items-start">
            {supply.status === 'completed' ? (
              <PackageCheck className="w-8 h-8 text-green-600 mr-4 flex-shrink-0" />
            ) : (
              <FileWarning className="w-8 h-8 text-orange-600 mr-4 flex-shrink-0" />
            )}
            <div className="w-full">
              <h3 className={`text-xl font-bold ${supply.status === 'completed' ? 'text-green-900' : 'text-orange-900'}`}>
                {supply.status === 'completed' ? 'Поставка принята' : 'Поставка принята с расхождениями'}
              </h3>

              <div className="mt-6 flex flex-wrap gap-8">
                <div>
                  <p className={`text-sm font-bold uppercase tracking-widest ${supply.status === 'completed' ? 'text-green-700' : 'text-orange-700'}`}>Заявлено</p>
                  <p className={`mt-1 text-3xl font-black ${supply.status === 'completed' ? 'text-green-900' : 'text-orange-900'}`}>{supply.totalExpectedItems}</p>
                </div>
                <div>
                  <p className={`text-sm font-bold uppercase tracking-widest ${supply.status === 'completed' ? 'text-green-700' : 'text-orange-700'}`}>Принято</p>
                  <p className={`mt-1 text-3xl font-black ${supply.status === 'completed' ? 'text-green-900' : 'text-orange-900'}`}>{supply.totalAcceptedItems}</p>
                </div>
                {supply.status === 'completed_with_discrepancies' && (
                  <div>
                    <p className="text-sm font-bold uppercase tracking-widest text-red-700">Расхождение</p>
                    <p className="mt-1 text-3xl font-black text-red-700">{supply.totalAcceptedItems - supply.totalExpectedItems}</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Status Timeline */}
      <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6 sm:p-8 mb-8 overflow-hidden">
        <div className="relative">
          <div className="absolute inset-0 flex items-center" aria-hidden="true">
            <div className="h-1 w-full bg-gray-100 rounded"></div>
          </div>
          <div className="relative flex justify-between">
            {['Создана', 'Передана перевозчику', 'Прибыла в ZAMK', 'Приёмка', 'Завершена'].map((label, idx) => {
              const isActive = currentStateIndex >= idx;
              const isCurrent = currentStateIndex === idx;
              return (
                <div key={label} className="flex flex-col items-center">
                  <div className={`h-6 w-6 rounded-full flex items-center justify-center ring-4 ring-white z-10 transition-colors ${isActive ? 'bg-black' : 'bg-gray-200'} ${isCurrent ? 'ring-gray-100 ring-8' : ''}`}>
                    {isActive && <div className="h-2 w-2 bg-white rounded-full"></div>}
                  </div>
                  <p className={`mt-3 text-xs sm:text-sm font-bold ${isActive ? 'text-black' : 'text-gray-400'} max-w-[80px] sm:max-w-none text-center leading-tight`}>{label}</p>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8 mb-8">
        {/* Left Column */}
        <div className="lg:col-span-1 space-y-8">
          {/* Delivery Card */}
          <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6">
            <h3 className="text-sm font-bold text-gray-500 uppercase tracking-widest mb-6">Доставка</h3>
            <div className="flex items-start">
              <Truck className="w-5 h-5 text-gray-400 mr-3 mt-0.5" />
              <div>
                <p className="font-bold text-gray-900">Транспортная компания</p>
                <div className="mt-4 space-y-3">
                  <div>
                    <p className="text-xs text-gray-500">Перевозчик</p>
                    <p className="text-sm font-medium text-gray-900">{supply.carrierName || 'Не указан'}</p>
                  </div>
                  <div>
                    <p className="text-xs text-gray-500">Трек-номер</p>
                    <p className="text-sm font-mono text-gray-900 font-medium">{supply.trackingNumber || 'Не указан'}</p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Gruzomesto Card */}
          <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6">
            <h3 className="text-sm font-bold text-gray-500 uppercase tracking-widest mb-6">Грузоместо 1</h3>

            <div className="flex flex-col items-center p-6 bg-gray-50 rounded-xl border border-gray-100 mb-6">
              <span className="text-xs font-bold text-gray-500 uppercase tracking-widest mb-4">{qrLabel}</span>
              <div className="bg-white p-4 rounded-xl shadow-sm mb-4">
                <QRCodeSVG value={qrValue} size={160} level="H" />
              </div>
              <div className="text-center">
                <p className="text-xs text-gray-500 mb-1">Код</p>
                <p className="font-mono font-bold text-lg tracking-wider text-gray-900">{boxCode}</p>
              </div>
            </div>

            <p className="text-xs text-center text-gray-500 mb-6 flex items-center justify-center">
              <HelpCircle className="w-4 h-4 mr-1.5" />
              Сотрудник ZAMK отсканирует код при приёмке.
            </p>

            <Link
              to={`/supplies/${supply.id}/boxes/${box?.id || 'default'}/label`}
              className="w-full flex items-center justify-center py-3 border border-gray-300 rounded-xl text-sm font-bold text-gray-700 hover:bg-gray-50 transition-colors"
            >
              <Printer className="w-4 h-4 mr-2" />
              Открыть этикетку
            </Link>
          </div>
        </div>

        {/* Right Column: Specification */}
        <div className="lg:col-span-2 space-y-6">
          <div className="bg-white rounded-2xl shadow-sm border border-gray-200 overflow-hidden flex flex-col h-full">
            <div className="px-6 py-5 border-b border-gray-200">
              <h3 className="text-lg font-bold text-gray-900">Состав поставки</h3>
            </div>
            <div className="flex-1 overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="px-6 py-4 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">Товар / Вариант</th>
                    <th scope="col" className="px-6 py-4 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">Артикул / Штрихкод</th>
                    <th scope="col" className="px-6 py-4 text-right text-xs font-bold text-gray-500 uppercase tracking-wider">Заявлено</th>
                    <th scope="col" className="px-6 py-4 text-right text-xs font-bold text-gray-500 uppercase tracking-wider">Принято</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-100">
                  {supply.items?.map(item => {
                    const hasDiscrepancy = isCompleted && item.expectedQuantity !== item.acceptedQuantity;
                    const variantOptions = (item.colorName || item.sizeName)
                      ? [item.colorName, item.sizeName].filter(Boolean).join(' · ')
                      : 'Стандарт';
                    const displaySku = item.sellerSku || item.sku || '—';
                    const displayBarcode = item.barcode || '—';

                    return (
                      <tr key={item.id} className={hasDiscrepancy ? 'bg-orange-50/20' : 'hover:bg-gray-50/50'}>
                        <td className="px-6 py-5 whitespace-nowrap">
                          <div className="text-sm font-bold text-gray-900">{item.productTitle || 'Товар'}</div>
                          <div className="text-sm text-gray-500">{variantOptions}</div>
                        </td>
                        <td className="px-6 py-5 whitespace-nowrap">
                          <div className="text-sm font-mono font-medium text-gray-900">{displaySku}</div>
                          <div className="text-xs font-mono text-gray-500">{displayBarcode}</div>
                        </td>
                        <td className="px-6 py-5 whitespace-nowrap text-right">
                          <div className="text-lg font-medium text-gray-900">{item.expectedQuantity}</div>
                        </td>
                        <td className="px-6 py-5 whitespace-nowrap text-right">
                          {!isCompleted ? (
                            <span className="text-gray-300 font-bold">—</span>
                          ) : (
                            <div className="flex flex-col items-end">
                              <span className={`text-lg font-bold ${hasDiscrepancy ? (item.acceptedQuantity > item.expectedQuantity ? 'text-green-600' : 'text-orange-600') : 'text-gray-900'}`}>
                                {item.acceptedQuantity}
                              </span>
                              {hasDiscrepancy && (
                                <span className={`text-xs font-bold ${item.acceptedQuantity > item.expectedQuantity ? 'text-green-600' : 'text-orange-600'}`}>
                                  {item.acceptedQuantity > item.expectedQuantity ? '+' : ''}{item.acceptedQuantity - item.expectedQuantity}
                                </span>
                              )}
                            </div>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
                <tfoot className="bg-gray-50 border-t border-gray-200">
                  <tr>
                    <td colSpan={2} className="px-6 py-4 text-right text-xs font-bold text-gray-500 uppercase tracking-wider">Итого:</td>
                    <td className="px-6 py-4 text-right text-lg font-black text-gray-900">{supply.totalExpectedItems}</td>
                    <td className="px-6 py-4 text-right text-lg font-black text-gray-900">{isCompleted ? supply.totalAcceptedItems : '—'}</td>
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        </div>
      </div>

      {/* Confirmation Modal */}
      {showConfirmModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          {/* Backdrop */}
          <div
            className="fixed inset-0 bg-black/50 backdrop-blur-sm transition-opacity"
            aria-hidden="true"
            onClick={() => {
              if (!submitting) {
                setActionError(null);
                setShowConfirmModal(false);
              }
            }}
          />

          {/* Centering wrapper */}
          <div className="min-h-full flex items-center justify-center p-4 text-center">
            {/* Modal Card */}
            <div
              className="relative bg-white rounded-2xl text-left overflow-hidden shadow-2xl transform transition-all sm:max-w-lg sm:w-full z-10"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="p-6 sm:p-8">
                <div className="sm:flex sm:items-start">
                  <div className="mx-auto flex-shrink-0 flex items-center justify-center h-12 w-12 rounded-2xl bg-blue-50 sm:mx-0 sm:h-12 sm:w-12">
                    <Truck className="h-6 w-6 text-blue-600" />
                  </div>
                  <div className="mt-3 text-center sm:mt-0 sm:ml-5 sm:text-left flex-1">
                    <h3 className="text-xl font-bold text-gray-900">Подтвердить отправку?</h3>
                    <div className="mt-2">
                      <p className="text-sm text-gray-500">
                        После подтверждения поставка перейдёт в статус «В пути» и будет ожидать приёмки на складе ZAMK.
                      </p>
                    </div>
                    {actionError && (
                      <div className="mt-4 bg-red-50 p-3.5 rounded-xl flex items-center border border-red-100 text-left">
                        <AlertCircle className="h-5 w-5 text-red-500 mr-2.5 flex-shrink-0" />
                        <span className="text-red-700 text-xs font-medium">{actionError}</span>
                      </div>
                    )}
                  </div>
                </div>
              </div>

              <div className="bg-gray-50/80 px-6 py-4 border-t border-gray-100 sm:flex sm:flex-row-reverse gap-3">
                <button
                  type="button"
                  disabled={submitting}
                  onClick={(e) => {
                    e.stopPropagation();
                    handleMarkShipped();
                  }}
                  className="w-full sm:w-auto inline-flex justify-center items-center px-6 py-2.5 bg-black text-white font-bold rounded-xl text-sm hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-sm"
                >
                  {submitting ? 'Отправляем...' : 'Передал перевозчику'}
                </button>
                <button
                  type="button"
                  disabled={submitting}
                  onClick={(e) => {
                    e.stopPropagation();
                    setActionError(null);
                    setShowConfirmModal(false);
                  }}
                  className="mt-3 sm:mt-0 w-full sm:w-auto inline-flex justify-center items-center px-5 py-2.5 bg-white border border-gray-300 text-gray-700 font-bold rounded-xl text-sm hover:bg-gray-50 disabled:opacity-50 transition-colors"
                >
                  Отмена
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
