import { useState, useEffect, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, AlertCircle, Search, Truck, Image as ImageIcon } from 'lucide-react';
import { getSellerProducts, createSellerSupply } from '@zamk/api-client/src/seller';
import type { SellerProduct } from '@zamk/api-client/src/types';

interface BoxItem {
  variantId: string;
  sku: string;
  title: string;
  options: string;
  barcode: string;
  quantity: number;
  imageUrl?: string;
}

export function SellerSupplyNew() {
  const navigate = useNavigate();
  const [products, setProducts] = useState<SellerProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [step, setStep] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');

  // Step 1: Products
  const [selectedItems, setSelectedItems] = useState<BoxItem[]>([]);

  // Step 2: Handoff
  const [carrierCompany, setCarrierCompany] = useState('');
  const [trackingNumber, setTrackingNumber] = useState('');
  const [carrierError, setCarrierError] = useState<string | null>(null);
  const [trackingError, setTrackingError] = useState<string | null>(null);

  const [submitting, setSubmitting] = useState(false);

  const handleStep2Continue = () => {
    let hasErr = false;
    if (!carrierCompany.trim()) {
      setCarrierError('Укажите транспортную компанию.');
      hasErr = true;
    } else {
      setCarrierError(null);
    }
    if (!trackingNumber.trim()) {
      setTrackingError('Укажите трек-номер отправления.');
      hasErr = true;
    } else {
      setTrackingError(null);
    }
    if (!hasErr) {
      setStep(3);
    }
  };

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
    let filtered = products;
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      filtered = products.filter(p =>
        p.title.toLowerCase().includes(query) ||
        p.variants?.some(v =>
          (v.sku && v.sku.toLowerCase().includes(query)) ||
          (v.sellerSku && v.sellerSku.toLowerCase().includes(query)) ||
          (v.barcode && v.barcode.toLowerCase().includes(query))
        )
      );
    }

    return filtered.map(p => {
      const mainImage = p.images?.find(i => i.isMain)?.url || p.images?.[0]?.url;
      return {
        ...p,
        mainImage,
        variants: (p.variants || []).map(v => {
          const color = v.colorName || v.color;
          const size = v.sizeName || v.size;
          const optionsStr = (color || size)
            ? [color, size].filter(Boolean).join(' · ')
            : (v.optionValues && Object.keys(v.optionValues).length > 0
                ? Object.entries(v.optionValues).map(([_, val]) => `${val}`).join(' · ')
                : 'Стандарт');

          return {
            ...v,
            optionsStr,
            productTitle: p.title
          };
        })
      };
    });
  }, [products, searchQuery]);

  // Step 1 Handlers
  const handleItemQuantityChange = (variant: any, qty: number, imageUrl?: string) => {
    const validQty = Math.max(0, qty);
    setSelectedItems(prev => {
      const existing = prev.find(i => i.variantId === variant.id);
      if (validQty <= 0) {
        return prev.filter(i => i.variantId !== variant.id);
      }
      if (existing) {
        return prev.map(i => i.variantId === variant.id ? { ...i, quantity: validQty } : i);
      }
      return [...prev, {
        variantId: variant.id,
        sku: variant.sellerSku || variant.sku || '',
        title: variant.productTitle,
        options: variant.optionsStr,
        barcode: variant.barcode || '',
        quantity: validQty,
        imageUrl
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
    <div className="max-w-6xl mx-auto py-8 px-4 sm:px-6">
      <div className="mb-8 flex items-center">
        <Link to="/supplies" className="mr-4 text-gray-500 hover:text-black">
          <ArrowLeft className="w-5 h-5" />
        </Link>
        <h1 className="text-3xl font-bold text-gray-900">Новая поставка</h1>
      </div>

      {error && (
        <div className="mb-6 p-4 bg-red-50 text-red-700 rounded-lg flex items-center border border-red-100">
          <AlertCircle className="w-5 h-5 mr-3" />
          <span className="font-medium">{error}</span>
        </div>
      )}

      {/* Progress */}
      <div className="mb-10 max-w-2xl mx-auto">
        <div className="flex items-center justify-between">
          <div className={`flex flex-col items-center ${step >= 1 ? 'text-black' : 'text-gray-400'}`}>
            <div className={`w-10 h-10 rounded-full flex items-center justify-center font-bold mb-2 transition-colors ${step >= 1 ? 'bg-black text-white shadow-md' : 'bg-gray-100 text-gray-400'}`}>1</div>
            <span className="text-sm font-bold tracking-wide">Товары</span>
          </div>
          <div className="flex-1 h-1 mx-4 bg-gray-100 rounded">
            <div className={`h-full bg-black rounded transition-all duration-300 ${step >= 2 ? 'w-full' : 'w-0'}`} />
          </div>
          <div className={`flex flex-col items-center ${step >= 2 ? 'text-black' : 'text-gray-400'}`}>
            <div className={`w-10 h-10 rounded-full flex items-center justify-center font-bold mb-2 transition-colors ${step >= 2 ? 'bg-black text-white shadow-md' : 'bg-gray-100 text-gray-400'}`}>2</div>
            <span className="text-sm font-bold tracking-wide">Доставка</span>
          </div>
          <div className="flex-1 h-1 mx-4 bg-gray-100 rounded">
            <div className={`h-full bg-black rounded transition-all duration-300 ${step >= 3 ? 'w-full' : 'w-0'}`} />
          </div>
          <div className={`flex flex-col items-center ${step >= 3 ? 'text-black' : 'text-gray-400'}`}>
            <div className={`w-10 h-10 rounded-full flex items-center justify-center font-bold mb-2 transition-colors ${step >= 3 ? 'bg-black text-white shadow-md' : 'bg-gray-100 text-gray-400'}`}>3</div>
            <span className="text-sm font-bold tracking-wide">Проверка</span>
          </div>
        </div>
      </div>

      {/* STEP 1: Products */}
      {step === 1 && (
        <div className="space-y-6">
          <div className="bg-white shadow-sm border border-gray-200 sm:rounded-2xl p-6 sm:p-8">
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-8">
              <div>
                <h3 className="text-2xl font-bold text-gray-900">Товары для поставки</h3>
                <p className="mt-1 text-sm text-gray-500">Укажите количество товаров, которое отправляете на склад ZAMK.</p>
              </div>
              <div className="mt-4 sm:mt-0 text-right">
                <div className="inline-flex flex-col items-end bg-gray-50 px-4 py-2 rounded-lg border border-gray-100">
                  <span className="text-xs text-gray-500 font-bold uppercase tracking-wide">Всего товаров</span>
                  <span className="text-2xl font-black text-gray-900">{declaredTotal}</span>
                </div>
              </div>
            </div>

            <div className="mb-6">
              <div className="relative rounded-md shadow-sm max-w-md">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <Search className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  type="text"
                  className="focus:ring-black focus:border-black block w-full pl-10 sm:text-sm border-gray-300 rounded-lg py-3"
                  placeholder="Поиск по названию, артикулу или штрихкоду"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            {loading ? (
              <div className="py-20 text-center text-gray-500">Загрузка товаров...</div>
            ) : groupedProducts.length === 0 ? (
              <div className="py-20 text-center text-gray-500 bg-gray-50 rounded-xl border border-gray-100">Нет подходящих товаров для поставки.</div>
            ) : (
              <div className="space-y-4">
                {groupedProducts.map(product => (
                  <div key={product.id} className="border border-gray-200 rounded-xl overflow-hidden bg-white">
                    <div className="divide-y divide-gray-100">
                      {product.variants?.map((v: any) => {
                        const selectedQty = selectedItems.find(i => i.variantId === v.id)?.quantity || 0;
                        const isSelected = selectedQty > 0;
                        return (
                          <div key={v.id} className={`p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between transition-colors ${isSelected ? 'bg-gray-50/50' : 'hover:bg-gray-50/30'}`}>
                            <div className="flex items-center mb-4 sm:mb-0">
                              <div className="w-16 h-20 bg-gray-100 rounded-md overflow-hidden flex-shrink-0 flex items-center justify-center border border-gray-200">
                                {product.mainImage ? (
                                  <img src={product.mainImage} alt={product.title} className="w-full h-full object-cover" />
                                ) : (
                                  <ImageIcon className="w-6 h-6 text-gray-300" />
                                )}
                              </div>
                              <div className="ml-4">
                                <h4 className="text-sm font-bold text-gray-900">{product.title}</h4>
                                <p className="text-sm font-medium text-gray-600 mt-0.5">{v.optionsStr}</p>
                                <div className="flex items-center gap-3 mt-2">
                                  <span className="text-xs text-gray-500 flex items-center">
                                    Артикул: <span className="font-mono font-medium text-gray-900 ml-1 px-1.5 py-0.5 bg-gray-100 rounded">{v.sku || 'нет'}</span>
                                  </span>
                                  <span className="text-xs text-gray-500 flex items-center">
                                    Штрихкод: <span className="font-mono font-medium text-gray-900 ml-1 px-1.5 py-0.5 bg-gray-100 rounded">{v.barcode || 'нет'}</span>
                                  </span>
                                </div>
                              </div>
                            </div>
                            <div className="w-full sm:w-32 flex-shrink-0">
                              <label className="block text-xs font-bold text-gray-500 uppercase tracking-wider mb-1.5 sm:text-right">Количество (шт)</label>
                              <input
                                type="number"
                                min="0"
                                step="1"
                                value={selectedQty || ''}
                                onChange={(e) => handleItemQuantityChange(v, parseInt(e.target.value) || 0, product.mainImage)}
                                className={`block w-full sm:text-right rounded-lg shadow-sm focus:ring-black focus:border-black sm:text-lg font-medium py-2 ${isSelected ? 'border-black bg-white' : 'border-gray-300'}`}
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

          <div className="flex justify-between items-center fixed bottom-0 left-0 right-0 p-4 bg-white border-t border-gray-200 z-10 sm:static sm:bg-transparent sm:border-0 sm:p-0">
            <span className="text-sm font-medium text-gray-500 sm:hidden">Всего: {declaredTotal} шт</span>
            <div className="flex justify-end w-full sm:w-auto">
              {declaredTotal === 0 && selectedItems.length === 0 && (
                <span className="text-orange-600 text-sm font-medium mr-4 hidden sm:inline-flex items-center">Укажите количество хотя бы для одного товара</span>
              )}
              <button
                onClick={() => setStep(2)}
                disabled={declaredTotal <= 0}
                className="w-full sm:w-auto px-8 py-3 bg-black text-white font-bold rounded-xl disabled:opacity-30 disabled:cursor-not-allowed hover:bg-gray-800 transition-colors"
              >
                Продолжить
              </button>
            </div>
          </div>
        </div>
      )}

      {/* STEP 2: Handoff */}
      {step === 2 && (
        <div className="space-y-6 max-w-3xl mx-auto">
          <div className="bg-white shadow-sm border border-gray-200 sm:rounded-2xl p-6 sm:p-10">
            <h3 className="text-2xl font-bold text-gray-900 mb-8">Доставка на склад ZAMK</h3>

            <div className="space-y-8">
              <div className={`p-5 border-2 rounded-xl border-black bg-gray-50/50 flex items-start`}>
                <Truck className="w-6 h-6 text-black mt-0.5 mr-4 flex-shrink-0" />
                <div>
                  <span className="block text-base font-bold text-gray-900">Транспортная компания</span>
                  <span className="block text-sm text-gray-600 mt-1">Передайте поставку перевозчику для доставки на склад ZAMK.</span>
                </div>
              </div>

              <div className="space-y-5">
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-1.5">
                    Транспортная компания <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={carrierCompany}
                    onChange={e => {
                      setCarrierCompany(e.target.value);
                      if (e.target.value.trim()) setCarrierError(null);
                    }}
                    placeholder="Например: СДЭК, Деловые Линии"
                    className={`block w-full rounded-lg shadow-sm focus:border-black focus:ring-black sm:text-base py-2.5 ${carrierError ? 'border-red-500 bg-red-50/20' : 'border-gray-300'}`}
                  />
                  {carrierError && <p className="mt-1 text-xs font-bold text-red-600">{carrierError}</p>}
                </div>
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-1.5">
                    Трек-номер отправления <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={trackingNumber}
                    onChange={e => {
                      setTrackingNumber(e.target.value);
                      if (e.target.value.trim()) setTrackingError(null);
                    }}
                    placeholder="Например: 121212123241"
                    className={`block w-full rounded-lg shadow-sm focus:border-black focus:ring-black sm:text-base py-2.5 font-mono ${trackingError ? 'border-red-500 bg-red-50/20' : 'border-gray-300'}`}
                  />
                  {trackingError && <p className="mt-1 text-xs font-bold text-red-600">{trackingError}</p>}
                </div>
              </div>

              <div className="mt-8 pt-8 border-t border-gray-100">
                <h4 className="text-sm font-bold text-gray-500 uppercase tracking-widest mb-4">Получатель</h4>
                <div className="bg-gray-50 rounded-xl p-5 border border-gray-100">
                  <p className="font-bold text-gray-900">Склад ZAMK</p>
                  <p className="text-sm text-gray-500 mt-1">Ожидает доставки вашей транспортной компанией.</p>
                </div>
              </div>
            </div>
          </div>

          <div className="flex justify-between items-center fixed bottom-0 left-0 right-0 p-4 bg-white border-t border-gray-200 z-10 sm:static sm:bg-transparent sm:border-0 sm:p-0">
            <button onClick={() => setStep(1)} className="px-6 py-3 border border-gray-300 text-gray-700 font-bold rounded-xl hover:bg-gray-50 transition-colors">
              Назад
            </button>
            <button
              onClick={handleStep2Continue}
              className="px-8 py-3 bg-black text-white font-bold rounded-xl hover:bg-gray-800 transition-colors"
            >
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* STEP 3: Review */}
      {step === 3 && (
        <div className="space-y-6 max-w-4xl mx-auto">
          <div className="bg-white shadow-sm border border-gray-200 sm:rounded-2xl p-6 sm:p-10">
            <h3 className="text-2xl font-bold text-gray-900 mb-8">Проверьте поставку</h3>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-10">
              <div className="bg-gray-50 p-5 rounded-xl border border-gray-100">
                <h4 className="text-xs font-bold text-gray-500 uppercase tracking-widest mb-2">Товары</h4>
                <p className="text-lg font-medium text-gray-900">{selectedItems.length} SKU · <span className="font-bold">{declaredTotal} единиц</span></p>
              </div>

              <div className="bg-gray-50 p-5 rounded-xl border border-gray-100">
                <h4 className="text-xs font-bold text-gray-500 uppercase tracking-widest mb-2">Упаковка</h4>
                <p className="text-lg font-bold text-gray-900">1 грузоместо</p>
              </div>

              <div className="bg-gray-50 p-5 rounded-xl border border-gray-100">
                <h4 className="text-xs font-bold text-gray-500 uppercase tracking-widest mb-2">Доставка</h4>
                <p className="text-base font-bold text-gray-900">
                  {carrierCompany || 'Транспортная компания'}
                </p>
                {trackingNumber && (
                  <p className="text-sm font-mono text-gray-600 mt-1">Трек: {trackingNumber}</p>
                )}
              </div>
            </div>

            <div className="border-t border-gray-200 pt-8">
              <h4 className="text-lg font-bold text-gray-900 mb-6">Спецификация</h4>
              <div className="bg-white border border-gray-200 rounded-xl overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">Товар и вариант</th>
                      <th scope="col" className="px-6 py-3 text-left text-xs font-bold text-gray-500 uppercase tracking-wider">SKU</th>
                      <th scope="col" className="px-6 py-3 text-right text-xs font-bold text-gray-500 uppercase tracking-wider">Количество</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-100">
                    {selectedItems.map(i => (
                      <tr key={i.variantId}>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="flex items-center">
                            {i.imageUrl && (
                              <div className="flex-shrink-0 h-10 w-8 mr-3 rounded overflow-hidden border border-gray-100 bg-gray-50">
                                <img src={i.imageUrl} alt="" className="h-full w-full object-cover" />
                              </div>
                            )}
                            <div>
                              <div className="text-sm font-bold text-gray-900">{i.title}</div>
                              <div className="text-sm text-gray-500">{i.options}</div>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-600">
                          {i.sku || '-'}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-bold text-gray-900">
                          {i.quantity} шт
                        </td>
                      </tr>
                    ))}
                  </tbody>
                  <tfoot className="bg-gray-50">
                    <tr>
                      <td colSpan={2} className="px-6 py-4 text-right text-sm font-bold text-gray-900 uppercase">Всего:</td>
                      <td className="px-6 py-4 text-right text-lg font-black text-gray-900">{declaredTotal} единиц</td>
                    </tr>
                  </tfoot>
                </table>
              </div>
            </div>
          </div>

          <div className="flex justify-between items-center fixed bottom-0 left-0 right-0 p-4 bg-white border-t border-gray-200 z-10 sm:static sm:bg-transparent sm:border-0 sm:p-0">
            <button onClick={() => setStep(2)} className="px-6 py-3 border border-gray-300 text-gray-700 font-bold rounded-xl hover:bg-gray-50 transition-colors">
              Назад
            </button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="px-8 py-3 bg-black text-white text-lg font-bold rounded-xl hover:bg-gray-800 disabled:opacity-50 transition-colors"
            >
              {submitting ? 'Создание...' : 'Создать поставку'}
            </button>
          </div>
        </div>
      )}

    </div>
  );
}
