import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Truck, ArrowLeft, AlertCircle, Printer, PackageCheck, AlertTriangle } from 'lucide-react';
import { getSellerSupply, shipSellerSupply } from '@zamk/api-client/src/seller';
import type { SellerSupply } from '@zamk/api-client/src/types';
import { QRCodeSVG } from 'qrcode.react';

export function SellerSupplyDetail() {
  const { id } = useParams<{ id: string }>();
  const [supply, setSupply] = useState<SellerSupply | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [shipping, setShipping] = useState(false);

  useEffect(() => {
    if (id) {
      fetchSupply();
    }
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

  const handleShip = async () => {
    if (!confirm('Вы уверены, что хотите отметить поставку как отправленную? После этого редактирование будет недоступно.')) {
      return;
    }
    
    try {
      setShipping(true);
      await shipSellerSupply(id!);
      await fetchSupply();
    } catch (err: any) {
      setError(err.message || 'Ошибка при отправке');
    } finally {
      setShipping(false);
    }
  };

  const getHumanStatus = (status: string) => {
    switch (status) {
      case 'draft': return 'Черновик';
      case 'ready_to_ship':
      case 'pending_shipment': return 'Готовится к отправке';
      case 'shipped':
      case 'shipped_by_seller': return 'Отправлена';
      case 'receiving': return 'На приёмке';
      case 'completed': return 'Принята';
      case 'completed_with_discrepancies': return 'Принята с расхождениями';
      default: return status;
    }
  };

  const renderProgressBar = (status: string) => {
    const steps = [
      { id: 'draft', label: 'Создана' },
      { id: 'prep', label: 'Подготовка' },
      { id: 'shipped', label: 'В пути' },
      { id: 'receiving', label: 'Приёмка' },
      { id: 'completed', label: 'Завершена' }
    ];

    let currentIndex = 0;
    if (status === 'draft') currentIndex = 1;
    if (status === 'shipped') currentIndex = 2;
    if (status === 'receiving') currentIndex = 3;
    if (status === 'completed' || status === 'completed_with_discrepancies') currentIndex = 4;

    return (
      <div className="flex items-center justify-between relative mt-8 mb-12">
        <div className="absolute left-0 top-1/2 -mt-px w-full h-1 bg-gray-200 -z-10"></div>
        <div className="absolute left-0 top-1/2 -mt-px h-1 bg-black -z-10 transition-all duration-500" style={{ width: `${(currentIndex / 4) * 100}%` }}></div>
        {steps.map((step, idx) => (
          <div key={step.id} className="flex flex-col items-center relative z-10">
            <div className={`w-6 h-6 rounded-full border-4 ${idx <= currentIndex ? 'bg-black border-black' : 'bg-white border-gray-300'}`}></div>
            <span className={`absolute top-8 text-xs font-bold whitespace-nowrap ${idx <= currentIndex ? 'text-black' : 'text-gray-400'}`}>{step.label}</span>
          </div>
        ))}
      </div>
    );
  };

  if (loading) {
    return <div className="p-8 flex justify-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div></div>;
  }

  if (!supply) {
    return <div className="p-8 text-center text-red-500">Поставка не найдена</div>;
  }

  const isCompleted = supply.status === 'completed' || supply.status === 'completed_with_discrepancies';

  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-8 pb-32">
      <div className="mb-6">
        <Link to="/supplies" className="flex items-center text-sm text-gray-500 hover:text-black transition-colors">
          <ArrowLeft className="w-4 h-4 mr-1" />
          Назад к поставкам
        </Link>
      </div>

      <div className="flex justify-between items-start mb-4">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 tracking-tight">
            Поставка {supply.supplyNumber}
          </h1>
          <p className="mt-2 text-sm text-gray-500">
            Создана {new Date(supply.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' })}
          </p>
        </div>
        
        <div className="flex items-center space-x-4">
          <span className={`px-4 py-2 rounded-full text-sm font-bold uppercase tracking-wider
            ${supply.status === 'draft' ? 'bg-gray-100 text-gray-800' : ''}
            ${supply.status === 'shipped' || supply.status === 'shipped_by_seller' ? 'bg-blue-100 text-blue-800' : ''}
            ${supply.status === 'receiving' ? 'bg-yellow-100 text-yellow-800' : ''}
            ${supply.status === 'completed' ? 'bg-green-100 text-green-800' : ''}
            ${supply.status === 'completed_with_discrepancies' ? 'bg-orange-100 text-orange-800' : ''}
          `}>
            {getHumanStatus(supply.status)}
          </span>
          
          {supply.status === 'draft' && (
            <button
              onClick={handleShip}
              disabled={shipping}
              className="inline-flex items-center px-6 py-2 border border-transparent rounded-full shadow-sm text-sm font-bold text-white bg-black hover:bg-gray-800 focus:outline-none disabled:opacity-50 transition-colors"
            >
              <Truck className="-ml-1 mr-2 h-4 w-4" />
              {shipping ? 'Отправка...' : 'Отправить поставку'}
            </button>
          )}
        </div>
      </div>

      {renderProgressBar(supply.status)}

      {error && (
        <div className="mb-8 bg-red-50 p-4 rounded-lg flex items-center border border-red-100">
          <AlertCircle className="h-5 w-5 text-red-500 mr-3" />
          <span className="text-red-700 font-medium">{error}</span>
        </div>
      )}

      {/* Discrepancy Results block */}
      {isCompleted && (
        <div className={`mb-8 p-6 rounded-2xl border ${supply.status === 'completed' ? 'bg-green-50 border-green-200' : 'bg-orange-50 border-orange-200'}`}>
          <div className="flex items-start">
            {supply.status === 'completed' ? (
              <PackageCheck className="w-8 h-8 text-green-600 mr-4 mt-1" />
            ) : (
              <AlertTriangle className="w-8 h-8 text-orange-600 mr-4 mt-1" />
            )}
            <div className="flex-1">
              <h3 className={`text-xl font-bold ${supply.status === 'completed' ? 'text-green-900' : 'text-orange-900'}`}>
                {supply.status === 'completed' ? 'Поставка принята полностью.' : 'Итоги приёмки (есть расхождения)'}
              </h3>
              
              {supply.status === 'completed' && (
                <p className="mt-2 text-green-800">Все заявленные позиции были успешно приняты. {supply.totalAcceptedItems} единиц добавлено на склад ZAMK.</p>
              )}

              {supply.status === 'completed_with_discrepancies' && (
                <div className="mt-6 bg-white rounded-xl shadow-sm border border-orange-100 overflow-hidden">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-orange-50">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-bold text-orange-800 uppercase tracking-wider">SKU (Variant)</th>
                        <th className="px-6 py-3 text-right text-xs font-bold text-orange-800 uppercase tracking-wider">Expected</th>
                        <th className="px-6 py-3 text-right text-xs font-bold text-orange-800 uppercase tracking-wider">Accepted</th>
                        <th className="px-6 py-3 text-right text-xs font-bold text-orange-800 uppercase tracking-wider">Damaged</th>
                        <th className="px-6 py-3 text-right text-xs font-bold text-orange-800 uppercase tracking-wider">Missing</th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {/* Aggregate discrepancy from items */}
                      {supply.items?.map((item) => (
                          <tr key={item.sku} className={item.expectedQuantity !== item.acceptedQuantity ? 'bg-orange-50/30' : ''}>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-bold text-gray-900">{item.sku}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600 text-right">{item.expectedQuantity}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-bold text-green-600 text-right">{item.acceptedQuantity}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-bold text-red-600 text-right">{item.damagedQuantity}</td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-bold text-gray-500 text-right">{item.missingQuantity}</td>
                          </tr>
                      ))}
                    </tbody>
                  </table>
                  {supply.receivingComment && (
                    <div className="p-4 bg-orange-50 border-t border-orange-100">
                      <p className="text-sm font-bold text-orange-900 mb-1">Комментарий склада ZAMK:</p>
                      <p className="text-sm text-orange-800">{supply.receivingComment}</p>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8 mb-8">
        {/* QR Codes and Labels */}
        <div className="lg:col-span-1 space-y-6">
          <div className="bg-white rounded-2xl shadow-sm border border-gray-200 p-6">
            <div className="flex justify-between items-start mb-6">
              <h3 className="text-lg font-bold text-gray-900">QR-коды и этикетки</h3>
              <button className="p-2 text-gray-400 hover:text-black hover:bg-gray-100 rounded-full transition-colors">
                <Printer className="w-5 h-5" />
              </button>
            </div>
            
            <div className="flex flex-col items-center mb-8 p-4 bg-gray-50 rounded-xl">
              <span className="text-xs font-bold text-gray-500 uppercase tracking-widest mb-4">Главный QR поставки</span>
              <div className="bg-white p-3 rounded-lg shadow-sm">
                <QRCodeSVG value={supply.qrToken || supply.supplyNumber || ''} size={140} level="H" />
              </div>
              <span className="mt-3 font-mono font-bold text-lg">{supply.supplyNumber || ''}</span>
            </div>

            <div className="space-y-4">
              <h4 className="text-sm font-bold text-gray-900">Короба ({supply.totalExpectedBoxes})</h4>
              {supply.boxes?.map((box, idx) => (
                <div key={box.id} className="flex items-center p-3 bg-gray-50 rounded-lg border border-gray-100">
                  <div className="bg-white p-1.5 rounded shadow-sm mr-4">
                    <QRCodeSVG value={box.qrToken || box.boxNumber || box.id} size={48} level="M" />
                  </div>
                  <div>
                    <p className="font-bold text-sm text-gray-900">Короб {idx + 1}</p>
                    <p className="font-mono text-xs text-gray-500">{box.boxNumber || box.id}</p>
                  </div>
                </div>
              ))}
            </div>

            <button className="w-full mt-6 py-3 bg-black text-white rounded-xl font-bold hover:bg-gray-800 transition-colors">
              Распечатать этикетки
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="lg:col-span-2 space-y-6">
          <div className="bg-white rounded-2xl shadow-sm border border-gray-200 overflow-hidden">
            <div className="px-6 py-5 border-b border-gray-200 bg-gray-50/50">
              <h3 className="text-lg font-bold text-gray-900">Спецификация</h3>
            </div>
            <div className="p-6">
              <dl className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 mb-8">
                <div>
                  <dt className="text-xs font-bold text-gray-500 uppercase tracking-wider">Товаров заявлено</dt>
                  <dd className="mt-1 text-2xl font-light text-gray-900">{supply.totalExpectedItems} шт</dd>
                </div>
                <div>
                  <dt className="text-xs font-bold text-gray-500 uppercase tracking-wider">Коробов</dt>
                  <dd className="mt-1 text-2xl font-light text-gray-900">{supply.totalExpectedBoxes}</dd>
                </div>
                <div>
                  <dt className="text-xs font-bold text-gray-500 uppercase tracking-wider">Способ передачи</dt>
                  <dd className="mt-1 text-lg font-medium text-gray-900 mt-2">
                    {supply.handoffMethod === 'courier' ? 'Курьером' : 'Привоз на ПВЗ'}
                  </dd>
                </div>
              </dl>

              <div className="mt-8 space-y-6">
                {supply.boxes?.map((box, idx) => (
                  <div key={box.id} className="border border-gray-200 rounded-xl overflow-hidden">
                    <div className="px-4 py-3 bg-gray-50 border-b border-gray-200 flex justify-between items-center">
                      <h4 className="font-bold text-gray-900">Короб {idx + 1}</h4>
                      <span className="text-xs font-mono text-gray-500">{box.id}</span>
                    </div>
                    <ul className="divide-y divide-gray-100">
                      {box.items.map(item => (
                        <li key={item.id} className="p-4 flex justify-between items-center hover:bg-gray-50 transition-colors">
                          <div>
                            <p className="text-sm font-bold text-gray-900">{item.sku}</p>
                            <p className="text-xs text-gray-500 mt-1">Ожидалось: {item.quantity} шт</p>
                          </div>
                          <div className="text-right">
                            <span className="text-lg font-bold text-gray-900">{item.quantity} шт</span>
                          </div>
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
