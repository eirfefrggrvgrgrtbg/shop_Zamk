import { useState, useEffect, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, AlertCircle } from 'lucide-react';
import { getSellerProducts, createSellerSupply } from '@zamk/api-client/src/seller';
import type { SellerProduct } from '@zamk/api-client/src/types';

interface BoxItem {
  variantId: string;
  sku: string;
  title: string;
  options: string;
  quantity: number;
}

export function SellerSupplyNew() {
  const navigate = useNavigate();
  const [products, setProducts] = useState<SellerProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [step, setStep] = useState(1);
  
  // Step 1: Products
  const [selectedItems, setSelectedItems] = useState<BoxItem[]>([]);
  
  // Step 2: Handoff
  const [carrierCompany, setCarrierCompany] = useState('');
  const [trackingNumber, setTrackingNumber] = useState('');
  
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchProducts();
  }, []);

  const fetchProducts = async () => {
    try {
      setLoading(true);
      const data = await getSellerProducts();
      setProducts(data.filter(p => p.status === 'approved' || p.status === 'published'));
    } catch (err: any) {
      setError(err.message || 'Ошибка загрузки товаров');
    } finally {
      setLoading(false);
    }
  };

  const groupedProducts = useMemo(() => {
    return products.map(p => ({
      ...p,
      variants: (p.variants || []).map(v => ({
        ...v,
        optionsStr: Object.entries(v.optionValues || {})
          .map(([_, val]) => `${val}`)
          .join(' / ') || 'Стандарт',
        productTitle: p.title
      }))
    }));
  }, [products]);

  // Step 1 Handlers
  const handleItemQuantityChange = (variant: any, qty: number) => {
    setSelectedItems(prev => {
      const existing = prev.find(i => i.variantId === variant.id);
      if (qty <= 0) {
        return prev.filter(i => i.variantId !== variant.id);
      }
      if (existing) {
        return prev.map(i => i.variantId === variant.id ? { ...i, quantity: qty } : i);
      }
      return [...prev, {
        variantId: variant.id,
        sku: variant.sku || '',
        title: variant.productTitle,
        options: variant.optionsStr,
        quantity: qty
      }];
    });
  };

  const declaredTotal = selectedItems.reduce((sum, i) => sum + i.quantity, 0);

  const handleSubmit = async () => {
    const payloadItems = selectedItems.map(i => ({
      variantId: i.variantId,
      expectedQuantity: i.quantity
    }));

    try {
      setSubmitting(true);
      setError(null);
      const res = await createSellerSupply({
        handoffMethod: 'carrier_delivery',
        carrierName: carrierCompany,
        trackingNumber: trackingNumber,
        items: payloadItems
      });
      navigate(`/supplies/${res.id}`);
    } catch (err: any) {
      setError(err.message || 'Ошибка при создании поставки');
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-4xl mx-auto py-8">
      <div className="mb-8 flex items-center">
        <Link to="/supplies" className="mr-4 text-gray-500 hover:text-black">
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <h1 className="text-2xl font-bold text-gray-900">Новая поставка</h1>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="w-5 h-5 mr-2" />
          {error}
        </div>
      )}

      {/* Progress */}
      <div className="mb-8">
        <div className="flex items-center justify-between">
          <div className={`flex flex-col items-center ${step >= 1 ? 'text-black' : 'text-gray-400'}`}>
            <div className={`w-8 h-8 rounded-full flex items-center justify-center font-bold mb-2 ${step >= 1 ? 'bg-black text-white' : 'bg-gray-200 text-gray-500'}`}>1</div>
            <span className="text-sm font-medium">Товары</span>
          </div>
          <div className="flex-1 h-1 mx-4 bg-gray-200">
            <div className={`h-full bg-black transition-all ${step >= 2 ? 'w-full' : 'w-0'}`} />
          </div>
          <div className={`flex flex-col items-center ${step >= 2 ? 'text-black' : 'text-gray-400'}`}>
            <div className={`w-8 h-8 rounded-full flex items-center justify-center font-bold mb-2 ${step >= 2 ? 'bg-black text-white' : 'bg-gray-200 text-gray-500'}`}>2</div>
            <span className="text-sm font-medium">Передача</span>
          </div>
          <div className="flex-1 h-1 mx-4 bg-gray-200">
            <div className={`h-full bg-black transition-all ${step >= 3 ? 'w-full' : 'w-0'}`} />
          </div>
          <div className={`flex flex-col items-center ${step >= 3 ? 'text-black' : 'text-gray-400'}`}>
            <div className={`w-8 h-8 rounded-full flex items-center justify-center font-bold mb-2 ${step >= 3 ? 'bg-black text-white' : 'bg-gray-200 text-gray-500'}`}>3</div>
            <span className="text-sm font-medium">Проверка</span>
          </div>
        </div>
      </div>

      {/* STEP 1: Products */}
      {step === 1 && (
        <div className="space-y-6">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <div className="flex justify-between items-center mb-6">
              <h3 className="text-xl font-bold text-gray-900">Выберите товары для поставки</h3>
              <p className="text-sm text-gray-500 font-medium">
                Выбрано: {declaredTotal} шт.
              </p>
            </div>
            
            {loading ? (
              <div className="py-12 text-center text-gray-500">Загрузка товаров...</div>
            ) : groupedProducts.length === 0 ? (
              <div className="py-12 text-center text-gray-500">Нет одобренных товаров для поставки.</div>
            ) : (
              <div className="space-y-8">
                {groupedProducts.map(product => (
                  <div key={product.id} className="border border-gray-200 rounded-lg overflow-hidden">
                    <div className="bg-gray-50 px-4 py-3 border-b border-gray-200">
                      <h4 className="font-bold text-gray-900">{product.title}</h4>
                    </div>
                    <div className="divide-y divide-gray-100">
                      {product.variants?.map((v: any) => {
                        const selectedQty = selectedItems.find(i => i.variantId === v.id)?.quantity || 0;
                        return (
                          <div key={v.id} className="p-4 flex items-center justify-between hover:bg-gray-50">
                            <div>
                              <p className="text-sm font-medium text-gray-900">{v.optionsStr}</p>
                              <p className="text-xs text-gray-500 mt-1 flex flex-col gap-1">
                                <span>Артикул продавца: <span className="font-mono bg-gray-100 px-1 rounded">{v.sku || 'нет'}</span></span>
                                <span>Штрихкод ZAMK: <span className="font-mono bg-gray-100 px-1 rounded">{v.barcode || 'нет'}</span></span>
                              </p>
                            </div>
                            <div className="w-32">
                              <label className="block text-xs text-gray-500 mb-1 text-right">Количество (шт)</label>
                              <input
                                type="number"
                                min="0"
                                value={selectedQty || ''}
                                onChange={(e) => handleItemQuantityChange(v, parseInt(e.target.value) || 0)}
                                className="block w-full text-right rounded-md border-gray-300 shadow-sm focus:border-black focus:ring-black sm:text-sm"
                                placeholder="0"
                              />
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="flex justify-end">
            <button 
              onClick={() => setStep(2)} 
              disabled={declaredTotal <= 0}
              className="px-6 py-2 bg-black text-white rounded-md disabled:opacity-50"
            >
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* STEP 2: Handoff */}
      {step === 2 && (
        <div className="space-y-6">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <h3 className="text-xl font-bold text-gray-900 mb-6">Способ передачи</h3>
            
            <div className="space-y-4">
              <div className={`p-4 border-2 rounded-lg cursor-pointer border-black bg-gray-50`}>
                <div className="flex items-center">
                  <div className="ml-3">
                    <span className="block text-sm font-bold text-gray-900">Транспортная компания (СДЭК и др.)</span>
                    <span className="block text-sm text-gray-500 mt-1">Отправка силами транспортной компании на склад ZAMK</span>
                  </div>
                </div>
              </div>
              
              <div className="pt-4 space-y-4 border-t border-gray-200 mt-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700">Транспортная компания</label>
                  <input
                    type="text"
                    value={carrierCompany}
                    onChange={e => setCarrierCompany(e.target.value)}
                    placeholder="Например: СДЭК"
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-black focus:ring-black sm:text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Трек-номер отправления (если есть)</label>
                  <input
                    type="text"
                    value={trackingNumber}
                    onChange={e => setTrackingNumber(e.target.value)}
                    className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-black focus:ring-black sm:text-sm"
                  />
                </div>
              </div>
            </div>
          </div>

          <div className="flex justify-between items-center">
            <button onClick={() => setStep(1)} className="px-6 py-2 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50">
              Назад
            </button>
            <button 
              onClick={() => setStep(3)} 
              className="px-6 py-2 bg-black text-white rounded-md"
            >
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* STEP 3: Review */}
      {step === 3 && (
        <div className="space-y-6">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <h3 className="text-xl font-bold text-gray-900 mb-6">Проверка перед созданием</h3>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 mb-8">
              <div>
                <h4 className="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">Товары</h4>
                <p className="text-lg font-medium">{selectedItems.length} SKU / {declaredTotal} единиц</p>
              </div>
              
              <div className="md:col-span-2">
                <h4 className="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">Передача</h4>
                <p className="text-lg font-medium">Транспортная компания {carrierCompany ? `(${carrierCompany})` : ''}</p>
                {trackingNumber && (
                  <p className="text-sm text-gray-600 mt-1">Трек-номер: {trackingNumber}</p>
                )}
              </div>
            </div>

            <div className="border-t border-gray-200 pt-6">
              <h4 className="font-bold text-gray-900 mb-4">Спецификация</h4>
              <ul className="divide-y divide-gray-100 bg-gray-50 rounded-lg p-4">
                {selectedItems.map(i => (
                  <li key={i.variantId} className="py-2 flex justify-between">
                    <div>
                      <span className="text-sm font-medium">{i.title}</span>
                      <span className="text-xs text-gray-500 ml-2">{i.options}</span>
                    </div>
                    <span className="text-sm font-bold">{i.quantity} шт</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>

          <div className="flex justify-between items-center">
            <button onClick={() => setStep(2)} className="px-6 py-2 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50">
              Назад
            </button>
            <button 
              onClick={handleSubmit} 
              disabled={submitting}
              className="px-8 py-3 bg-black text-white text-lg font-medium rounded-md hover:bg-gray-800 disabled:opacity-50"
            >
              {submitting ? 'Создание...' : 'Создать поставку'}
            </button>
          </div>
        </div>
      )}

    </div>
  );
}
