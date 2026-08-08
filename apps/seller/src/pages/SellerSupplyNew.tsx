import { useState, useEffect, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, AlertCircle, Plus, Trash2, CheckCircle, Package } from 'lucide-react';
import { getSellerProducts, createSellerSupply } from '@zamk/api-client/src/seller';
import type { SellerProduct } from '@zamk/api-client/src/types';

interface BoxItem {
  variantId: string;
  sku: string;
  title: string;
  options: string;
  quantity: number;
}

interface SupplyBox {
  id: string; // temp id for UI
  items: BoxItem[];
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
  const [handoffMethod, setHandoffMethod] = useState<'self' | 'carrier' | 'other'>('self');
  const [carrierCompany, setCarrierCompany] = useState('');
  const [trackingNumber, setTrackingNumber] = useState('');
  
  // Step 3: Boxes
  const [boxes, setBoxes] = useState<SupplyBox[]>([{ id: 'BOX-001', items: [] }]);
  
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchProducts();
  }, []);

  const fetchProducts = async () => {
    try {
      setLoading(true);
      const data = await getSellerProducts();
      setProducts(data.filter(p => p.status === 'approved'));
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

  // Step 3 Handlers
  const declaredTotal = selectedItems.reduce((sum, i) => sum + i.quantity, 0);
  const distributedTotal = boxes.reduce((sum, box) => sum + box.items.reduce((s, i) => s + i.quantity, 0), 0);
  
  const addBox = () => {
    setBoxes(prev => [...prev, { id: `BOX-00${prev.length + 1}`, items: [] }]);
  };
  
  const removeBox = (boxId: string) => {
    setBoxes(prev => prev.filter(b => b.id !== boxId));
  };

  const updateBoxItemQuantity = (boxId: string, variantId: string, qty: number, maxQty: number) => {
    if (qty < 0 || qty > maxQty) return;
    setBoxes(prev => prev.map(box => {
      if (box.id !== boxId) return box;
      const existing = box.items.find(i => i.variantId === variantId);
      if (qty === 0) {
        return { ...box, items: box.items.filter(i => i.variantId !== variantId) };
      }
      if (existing) {
        return { ...box, items: box.items.map(i => i.variantId === variantId ? { ...i, quantity: qty } : i) };
      }
      const itemDef = selectedItems.find(i => i.variantId === variantId)!;
      return { ...box, items: [...box.items, { ...itemDef, quantity: qty }] };
    }));
  };

  const getRemainingToDistribute = (variantId: string) => {
    const declared = selectedItems.find(i => i.variantId === variantId)?.quantity || 0;
    const distributed = boxes.reduce((sum, box) => sum + (box.items.find(i => i.variantId === variantId)?.quantity || 0), 0);
    return declared - distributed;
  };

  const isBoxStepValid = declaredTotal > 0 && declaredTotal === distributedTotal;

  const handleSubmit = async () => {
    const payloadBoxes = boxes.filter(b => b.items.length > 0).map(b => ({
      items: b.items.map(i => ({ variantId: i.variantId, quantity: i.quantity }))
    }));

    const payloadItems = selectedItems.map(i => ({
      variantId: i.variantId,
      expectedQuantity: i.quantity
    }));

    // Map UI handoff to backend enum
    let methodEnum = 'pvz';
    if (handoffMethod === 'carrier') methodEnum = 'pvz'; // We treat all external carriers as PVZ delivery for now in backend, just capturing UX intent
    if (handoffMethod === 'other') methodEnum = 'pvz';
    
    try {
      setSubmitting(true);
      setError(null);
      const res = await createSellerSupply({
        handoffMethod: methodEnum,
        items: payloadItems,
        boxes: payloadBoxes
      });
      navigate(`/supplies/${res.id}`);
    } catch (err: any) {
      setError(err.message || 'Ошибка создания поставки');
      setSubmitting(false);
    }
  };

  if (loading) {
    return <div className="p-8 flex justify-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black"></div></div>;
  }

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-6">
        <Link to="/supplies" className="flex items-center text-sm text-gray-500 hover:text-black">
          <ArrowLeft className="w-4 h-4 mr-1" />
          Назад к поставкам
        </Link>
      </div>

      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900 mb-6">Создание новой поставки</h1>
        
        {/* Stepper */}
        <div className="flex items-center justify-between relative">
          <div className="absolute left-0 top-1/2 -mt-px w-full h-0.5 bg-gray-200 -z-10"></div>
          {[
            { num: 1, label: 'Товары' },
            { num: 2, label: 'Передача' },
            { num: 3, label: 'Короба' },
            { num: 4, label: 'Проверка' }
          ].map((s) => (
            <div key={s.num} className="flex flex-col items-center bg-gray-50 px-2 relative">
              <div className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium border-2 
                ${step > s.num ? 'bg-black border-black text-white' : 
                  step === s.num ? 'bg-white border-black text-black' : 
                  'bg-white border-gray-300 text-gray-400'}`}>
                {step > s.num ? <CheckCircle className="w-5 h-5" /> : s.num}
              </div>
              <span className={`mt-2 text-xs font-medium ${step >= s.num ? 'text-black' : 'text-gray-400'}`}>
                {s.label}
              </span>
            </div>
          ))}
        </div>
      </div>

      {error && (
        <div className="mb-6 bg-red-50 p-4 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 text-red-400 mr-2" />
          <span className="text-red-700 text-sm">{error}</span>
        </div>
      )}

      {/* STEP 1: Products */}
      {step === 1 && (
        <div className="space-y-6">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <h3 className="text-lg font-medium text-gray-900 mb-6">Выберите товары для поставки</h3>
            
            {groupedProducts.length === 0 ? (
              <p className="text-sm text-gray-500 text-center py-4">Нет одобренных товаров с добавленными вариантами.</p>
            ) : (
              <div className="space-y-8">
                {groupedProducts.map(p => (
                  <div key={p.id}>
                    <h4 className="font-bold text-gray-900 mb-3">{p.title}</h4>
                    <div className="bg-gray-50 rounded-lg overflow-hidden border border-gray-200">
                      <ul className="divide-y divide-gray-200">
                        {p.variants.map(v => {
                          const currentQty = selectedItems.find(i => i.variantId === v.id)?.quantity || 0;
                          return (
                            <li key={v.id} className="p-4 flex items-center justify-between hover:bg-gray-100 transition-colors">
                              <div>
                                <p className="text-sm font-medium text-gray-900">{v.optionsStr}</p>
                                <p className="text-xs text-gray-500 font-mono mt-1">SKU: {v.sku || 'Нет SKU'}</p>
                              </div>
                              <div className="flex items-center space-x-3">
                                <button 
                                  onClick={() => handleItemQuantityChange(v, currentQty - 1)}
                                  className="w-8 h-8 rounded-full border border-gray-300 flex items-center justify-center hover:bg-gray-200"
                                >-</button>
                                <span className="w-8 text-center font-medium">{currentQty}</span>
                                <button 
                                  onClick={() => handleItemQuantityChange(v, currentQty + 1)}
                                  className="w-8 h-8 rounded-full border border-gray-300 flex items-center justify-center hover:bg-gray-200"
                                >+</button>
                              </div>
                            </li>
                          );
                        })}
                      </ul>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
          
          <div className="flex justify-between items-center bg-white shadow sm:rounded-lg p-6">
            <div>
              <p className="text-sm text-gray-500">Итого выбрано</p>
              <p className="text-xl font-bold">{selectedItems.length} вариантов / {declaredTotal} единиц</p>
            </div>
            <button
              onClick={() => setStep(2)}
              disabled={declaredTotal === 0}
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
            <h3 className="text-lg font-medium text-gray-900 mb-6">Способ передачи</h3>
            <div className="space-y-4">
              <label className={`flex items-start p-4 border rounded-lg cursor-pointer transition-colors ${handoffMethod === 'self' ? 'border-black bg-gray-50' : 'border-gray-200 hover:bg-gray-50'}`}>
                <input
                  type="radio"
                  name="handoff"
                  checked={handoffMethod === 'self'}
                  onChange={() => setHandoffMethod('self')}
                  className="mt-1 h-4 w-4 text-black focus:ring-black border-gray-300"
                />
                <div className="ml-3">
                  <span className="block text-sm font-medium text-gray-900">Привезу сам</span>
                  <span className="block text-sm text-gray-500 mt-1">Доставка на центральный склад собственными силами.</span>
                </div>
              </label>

              <label className={`flex items-start p-4 border rounded-lg cursor-pointer transition-colors ${handoffMethod === 'carrier' ? 'border-black bg-gray-50' : 'border-gray-200 hover:bg-gray-50'}`}>
                <input
                  type="radio"
                  name="handoff"
                  checked={handoffMethod === 'carrier'}
                  onChange={() => setHandoffMethod('carrier')}
                  className="mt-1 h-4 w-4 text-black focus:ring-black border-gray-300"
                />
                <div className="ml-3 w-full">
                  <span className="block text-sm font-medium text-gray-900">Курьер / транспортная компания</span>
                  <span className="block text-sm text-gray-500 mt-1">Отправка сторонней транспортной компанией (СДЭК, Деловые Линии и др.)</span>
                  
                  {handoffMethod === 'carrier' && (
                    <div className="mt-4 grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-xs text-gray-700 mb-1">Компания</label>
                        <input type="text" value={carrierCompany} onChange={e => setCarrierCompany(e.target.value)} placeholder="Например, СДЭК" className="w-full px-3 py-2 border rounded-md text-sm" />
                      </div>
                      <div>
                        <label className="block text-xs text-gray-700 mb-1">Трек-номер</label>
                        <input type="text" value={trackingNumber} onChange={e => setTrackingNumber(e.target.value)} placeholder="Трек-номер" className="w-full px-3 py-2 border rounded-md text-sm" />
                      </div>
                    </div>
                  )}
                </div>
              </label>

              <label className={`flex items-start p-4 border rounded-lg cursor-pointer transition-colors ${handoffMethod === 'other' ? 'border-black bg-gray-50' : 'border-gray-200 hover:bg-gray-50'}`}>
                <input
                  type="radio"
                  name="handoff"
                  checked={handoffMethod === 'other'}
                  onChange={() => setHandoffMethod('other')}
                  className="mt-1 h-4 w-4 text-black focus:ring-black border-gray-300"
                />
                <div className="ml-3">
                  <span className="block text-sm font-medium text-gray-900">Другое</span>
                  <span className="block text-sm text-gray-500 mt-1">Индивидуальные условия доставки.</span>
                </div>
              </label>
            </div>
          </div>
          
          <div className="flex justify-between items-center">
            <button onClick={() => setStep(1)} className="px-6 py-2 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50">
              Назад
            </button>
            <button onClick={() => setStep(3)} className="px-6 py-2 bg-black text-white rounded-md">
              Продолжить
            </button>
          </div>
        </div>
      )}

      {/* STEP 3: Boxes */}
      {step === 3 && (
        <div className="space-y-6">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <div className="flex justify-between items-start mb-6">
              <div>
                <h3 className="text-lg font-medium text-gray-900">Короба</h3>
                <p className="text-sm text-gray-500 mt-1">Распределите товары по коробам для корректной приемки.</p>
              </div>
              <div className="text-right bg-gray-50 px-4 py-2 rounded-md border border-gray-200">
                <p className="text-sm font-medium">
                  Заявлено: {declaredTotal}
                </p>
                <p className={`text-sm font-bold flex items-center justify-end ${distributedTotal === declaredTotal ? 'text-green-600' : 'text-orange-600'}`}>
                  Распределено: {distributedTotal} {distributedTotal === declaredTotal && <CheckCircle className="w-4 h-4 ml-1" />}
                </p>
              </div>
            </div>

            <div className="space-y-6">
              {boxes.map((box) => (
                <div key={box.id} className="border border-gray-200 rounded-lg overflow-hidden">
                  <div className="bg-gray-50 px-4 py-3 flex justify-between items-center border-b border-gray-200">
                    <h4 className="font-bold text-gray-900 flex items-center">
                      <Package className="w-4 h-4 mr-2 text-gray-500" />
                      {box.id}
                      <span className="ml-3 text-sm font-normal text-gray-500">
                        {box.items.reduce((s,i) => s + i.quantity, 0)} шт
                      </span>
                    </h4>
                    {boxes.length > 1 && (
                      <button onClick={() => removeBox(box.id)} className="text-red-500 hover:text-red-700 text-sm flex items-center">
                        <Trash2 className="w-4 h-4 mr-1" /> Удалить
                      </button>
                    )}
                  </div>
                  <div className="p-4 bg-white space-y-4">
                    {selectedItems.map(item => {
                      const boxItemQty = box.items.find(i => i.variantId === item.variantId)?.quantity || 0;
                      const remaining = getRemainingToDistribute(item.variantId);
                      const maxAllowed = boxItemQty + remaining;
                      
                      return (
                        <div key={item.variantId} className="flex items-center justify-between">
                          <div className="flex-1">
                            <p className="text-sm font-medium text-gray-900">{item.title}</p>
                            <p className="text-xs text-gray-500">{item.options} • SKU: <span className="font-mono">{item.sku}</span></p>
                          </div>
                          <div className="flex items-center space-x-4">
                            <div className="text-xs text-gray-400 w-24 text-right">
                              Осталось: {remaining}
                            </div>
                            <div className="flex items-center space-x-2 border rounded-md px-2 py-1">
                              <button 
                                onClick={() => updateBoxItemQuantity(box.id, item.variantId, boxItemQty - 1, maxAllowed)}
                                className="w-6 h-6 text-gray-500 hover:text-black flex items-center justify-center disabled:opacity-30"
                                disabled={boxItemQty <= 0}
                              >-</button>
                              <span className="w-6 text-center text-sm font-medium">{boxItemQty}</span>
                              <button 
                                onClick={() => updateBoxItemQuantity(box.id, item.variantId, boxItemQty + 1, maxAllowed)}
                                className="w-6 h-6 text-gray-500 hover:text-black flex items-center justify-center disabled:opacity-30"
                                disabled={remaining <= 0}
                              >+</button>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>

            <button
              onClick={addBox}
              className="mt-6 flex items-center justify-center w-full py-3 border-2 border-dashed border-gray-300 rounded-lg text-sm font-medium text-gray-600 hover:border-black hover:text-black transition-colors"
            >
              <Plus className="w-5 h-5 mr-2" />
              Добавить короб
            </button>
          </div>

          <div className="flex justify-between items-center">
            <button onClick={() => setStep(2)} className="px-6 py-2 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50">
              Назад
            </button>
            <div className="flex items-center">
              {!isBoxStepValid && <span className="text-orange-600 text-sm mr-4">Распределите все {declaredTotal} единиц</span>}
              <button 
                onClick={() => setStep(4)} 
                disabled={!isBoxStepValid}
                className="px-6 py-2 bg-black text-white rounded-md disabled:opacity-50"
              >
                Продолжить
              </button>
            </div>
          </div>
        </div>
      )}

      {/* STEP 4: Review */}
      {step === 4 && (
        <div className="space-y-6">
          <div className="bg-white shadow sm:rounded-lg p-6">
            <h3 className="text-xl font-bold text-gray-900 mb-6">Проверка перед созданием</h3>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8 mb-8">
              <div>
                <h4 className="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">Товары</h4>
                <p className="text-lg font-medium">{selectedItems.length} SKU / {declaredTotal} единиц</p>
              </div>
              
              <div>
                <h4 className="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">Короба</h4>
                <p className="text-lg font-medium">{boxes.length}</p>
              </div>
              
              <div className="md:col-span-2">
                <h4 className="text-sm font-bold text-gray-500 uppercase tracking-wider mb-2">Передача</h4>
                <p className="text-lg font-medium">
                  {handoffMethod === 'self' && 'Привезу сам'}
                  {handoffMethod === 'carrier' && `СДЭК / транспортная компания ${carrierCompany ? `(${carrierCompany})` : ''}`}
                  {handoffMethod === 'other' && 'Другое'}
                </p>
                {handoffMethod === 'carrier' && trackingNumber && (
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
            <button onClick={() => setStep(3)} className="px-6 py-2 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50">
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
