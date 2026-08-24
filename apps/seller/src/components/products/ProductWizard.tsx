import { useState, useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, CheckCircle2 } from 'lucide-react';
import { getSellerCategories, getSellerCategorySchema, createSellerProduct, updateSellerProduct, submitSellerProductModeration, updateSellerProductPrices, type SellerCategory, type SellerCategorySchema } from '@zamk/api-client/src/seller';

import { initialWizardState, type WizardState } from './wizard/WizardState';
import { Step1Basics } from './wizard/Step1Basics';
import { Step2Category } from './wizard/Step2Category';
import { Step3Attributes } from './wizard/Step3Attributes';
import { Step4Variants } from './wizard/Step4Variants';
import { Step5SizeChart } from './wizard/Step5SizeChart';
import { Step6Media } from './wizard/Step6Media';
import { Step7Pricing } from './wizard/Step7Pricing';
import { Step8Review } from './wizard/Step8Review';

export type ProductWizardProps = {
  initialData?: any; 
  isEdit?: boolean;
};

export const ProductWizard: React.FC<ProductWizardProps> = ({ initialData, isEdit }) => {
  const navigate = useNavigate();
  const [step, setStep] = useState(1);
  const [savingDraft, setSavingDraft] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [showPublishedModal, setShowPublishedModal] = useState(false);

  const [toast, setToast] = useState<{message: string, type: 'error' | 'success'} | null>(null);
  const showToast = (message: string, type: 'error' | 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  };
    const [state, setState] = useState<WizardState>(initialWizardState);





  
  const normalizeState = (s: WizardState) => {
    return JSON.stringify({
      title: s.title,
      description: s.description,
      categoryId: s.categoryId,
      materialComposition: s.materialComposition,
      productAttributes: s.productAttributes,
      selectedColorIds: [...(s.selectedColorIds || [])].sort(),
      shadeNamesByColor: s.shadeNamesByColor,
      selectedSizeSystemId: s.selectedSizeSystemId,
      selectedSizeValueIds: [...(s.selectedSizeValueIds || [])].sort(),
      sizeChartRows: s.sizeChartRows,
      commonImages: (s.commonImages || []).map(i => ({id: i.id, url: i.url, sortOrder: i.sortOrder, isMain: i.isMain, isReady: i.isReady})),
      colorImages: Object.fromEntries(Object.entries(s.colorImages || {}).map(([k,v]) => [k, v.map(i => ({id: i.id, url: i.url, sortOrder: i.sortOrder, isMain: i.isMain, isReady: i.isReady}))])),
      variants: (s.variants || []).map(v => ({
        id: v.id,
        colorId: v.colorId,
        sizeValueId: v.sizeValueId,
        sellerSku: v.sellerSku,
        barcode: v.barcode,
        priceCents: v.priceCents,
        active: v.active,
        attributes: v.attributes
      }))
    });
  };

  const [lastSavedStateStr, setLastSavedStateStr] = useState<string>('');

  const contentDirty = useMemo(() => {
    if (!lastSavedStateStr) return false;
    const current = JSON.parse(normalizeState(state));
    const saved = JSON.parse(lastSavedStateStr);
    current.variants.forEach((v: any) => delete v.priceCents);
    saved.variants.forEach((v: any) => delete v.priceCents);
    return JSON.stringify(current) !== JSON.stringify(saved);
  }, [state, lastSavedStateStr]);

  const priceDirty = useMemo(() => {
    if (!lastSavedStateStr) return false;
    const current = JSON.parse(normalizeState(state));
    const saved = JSON.parse(lastSavedStateStr);
    
    return current.variants.some((v: any) => {
      if (!v.active || !v.id) return false;
      const sv = saved.variants.find((svv: any) => svv.id === v.id);
      if (!sv) return false;
      return sv.priceCents !== v.priceCents;
    });
  }, [state, lastSavedStateStr]);

  const isDirty = contentDirty || priceDirty;

  const [showLeaveModal, setShowLeaveModal] = useState(false);
  const [pendingNavigation, setPendingNavigation] = useState<string | null>(null);

  const handleBackNavigation = () => {
    if (isDirty) {
      setShowLeaveModal(true);
      setPendingNavigation('/products');
    } else {
      navigate('/products');
    }
  };

  const confirmLeave = () => {
    if (pendingNavigation) {
      navigate(pendingNavigation);
    }
  };

  const cancelLeave = () => {
    setShowLeaveModal(false);
    setPendingNavigation(null);
  };

  const [categories, setCategories] = useState<SellerCategory[]>([]);
  const [schema, setSchema] = useState<SellerCategorySchema | null>(null);
  const [schemaLoading, setSchemaLoading] = useState(false);
  const [schemaError, setSchemaError] = useState(false);

  useEffect(() => {
    if (isEdit && initialData && !state.categoryId) {
      const p = initialData;
      
      const pAttrs: Record<string, any> = {};
      if (p.attributes) {
        p.attributes.forEach((a: any) => {
          if (a.enumValueId) {
            // handle multi enum vs single enum below when schema loads
            if (pAttrs[a.attributeDefinitionId]) {
              if (Array.isArray(pAttrs[a.attributeDefinitionId])) {
                pAttrs[a.attributeDefinitionId].push(a.enumValueId);
              } else {
                pAttrs[a.attributeDefinitionId] = [pAttrs[a.attributeDefinitionId], a.enumValueId];
              }
            } else {
              pAttrs[a.attributeDefinitionId] = a.enumValueId;
            }
          }
          if (a.textValue) pAttrs[a.attributeDefinitionId] = a.textValue;
          if (a.numberValue) pAttrs[a.attributeDefinitionId] = a.numberValue;
          if (a.boolValue !== undefined) pAttrs[a.attributeDefinitionId] = a.boolValue;
        });
      }

      const commonImgs: any[] = [];
      const colorImgs: Record<string, any[]> = {};
      if (p.images) {
        p.images.forEach((img: any) => {
          const mapped = { id: img.id, url: img.imageUrl, sortOrder: img.sortOrder || 0, isMain: !!img.isMain, isReady: img.renditionUrl != null };
          if (img.colorId) {
            if (!colorImgs[img.colorId]) colorImgs[img.colorId] = [];
            colorImgs[img.colorId].push(mapped);
          } else {
            commonImgs.push(mapped);
          }
        });
      }
      
      const vShades: Record<string, string> = {};
      const vColors = new Set<string>();
      const vSizes = new Set<string>();
      const mappedVariants = (p.variants || []).map((v: any) => {
        if (v.colorId) vColors.add(v.colorId);
        if (v.sizeValueId) vSizes.add(v.sizeValueId);
        if (v.shadeName && v.colorId) vShades[v.colorId] = v.shadeName;
        
        const vAttrMap: Record<string, any> = {};
        if (v.attributes) {
           // wait backend doesn't return variant attributes yet, but if it did:
           v.attributes.forEach((a: any) => {
             if (a.enumValueId) vAttrMap[a.attributeDefinitionId] = a.enumValueId;
             if (a.textValue) vAttrMap[a.attributeDefinitionId] = a.textValue;
             if (a.numberValue) vAttrMap[a.attributeDefinitionId] = a.numberValue;
             if (a.boolValue !== undefined) vAttrMap[a.attributeDefinitionId] = a.boolValue;
           });
        }
        
        return {
          id: v.id,
          colorId: v.colorId || undefined,
          sizeValueId: v.sizeValueId || undefined,
          sellerSku: v.sellerSku || '',
          barcode: v.barcode || '',
          priceCents: v.priceCents,
          active: v.isActive !== false,
          attributes: Object.keys(vAttrMap).length > 0 ? vAttrMap : undefined
        };
      });

      const sChartRows: Record<string, any> = {};
      if (p.sizeChart?.rows) {
        p.sizeChart.rows.forEach((r: any) => {
          sChartRows[r.sizeValueId] = r.measurements;
        });
      }


      const newState = {
        title: p.title || '',
        description: p.description || '',
        id: p.id,
        categoryId: p.categoryId || '',
        materialComposition: p.materialComposition || [],
        productAttributes: pAttrs,
        selectedColorIds: Array.from(vColors),
        shadeNamesByColor: vShades,
        selectedSizeSystemId: p.sizeChart?.rows?.[0]?.sizeSystemId || '',
        selectedSizeValueIds: Array.from(vSizes),
        variants: mappedVariants,
        sizeChartRows: sChartRows,
        commonImages: commonImgs,
        colorImages: colorImgs
      };
      setState(newState);
      setLastSavedStateStr(normalizeState(newState));

    }
  }, [isEdit, initialData]);


  useEffect(() => {
    if (!isEdit) {
      setLastSavedStateStr(normalizeState(state));
    }
  }, []);

  useEffect(() => {
    getSellerCategories().then(setCategories).catch(console.error);
  }, []);

  const loadSchema = () => {
    if (state.categoryId) {
      setSchemaLoading(true);
      setSchemaError(false);
      getSellerCategorySchema(state.categoryId)
        .then(res => {
          setSchema(res);
          setSchemaLoading(false);
        })
        .catch(err => {
          console.error(err);
          setSchemaError(true);
          setSchemaLoading(false);
        });
    }
  };

  useEffect(() => {
    loadSchema();
  }, [state.categoryId]);

  const updateState = (update: Partial<WizardState>) => setState(prev => ({ ...prev, ...update }));

  const steps = [
    'Основное',
    'Категория',
    'Характеристики',
    'Варианты',
    'Таблица размеров',
    'Фото',
    'Цена и идентификаторы',
    'Проверка'
  ];


  const buildPayload = () => {
    // Flatten product level attributes
    const mappedAttrs: any[] = [];
    Object.entries(state.productAttributes).forEach(([defId, val]) => {
      const attrDef = schema?.attributes.find(a => a.id === defId);
      if (!attrDef) return;

      if (attrDef.valueSource === 'DICTIONARY' && attrDef.valueType === 'MULTI_ENUM') {
        const arr = Array.isArray(val) ? val : [val];
        arr.forEach(enumVal => {
          if (enumVal) mappedAttrs.push({ attributeDefinitionId: defId, enumValueId: enumVal });
        });
      } else {
        mappedAttrs.push({
          attributeDefinitionId: defId,
          textValue: attrDef.valueType === 'TEXT' ? val : undefined,
          numberValue: attrDef.valueType === 'NUMBER' ? val : undefined,
          boolValue: attrDef.valueType === 'BOOLEAN' ? val : undefined,
          enumValueId: attrDef.valueSource === 'DICTIONARY' ? val : undefined
        });
      }
    });

    const mappedVariants = state.variants.filter(v => v.active).map(v => {
      const vAttrs: any[] = [];
      if (v.attributes) {
        Object.entries(v.attributes).forEach(([defId, val]) => {
          const attrDef = schema?.attributes.find(a => a.id === defId);
          if (!attrDef) return;
          if (attrDef.valueSource === 'DICTIONARY' && attrDef.valueType === 'MULTI_ENUM') {
            const arr = Array.isArray(val) ? val : [val];
            arr.forEach(enumVal => {
              if (enumVal) vAttrs.push({ attributeDefinitionId: defId, enumValueId: enumVal });
            });
          } else {
            vAttrs.push({
              attributeDefinitionId: defId,
              textValue: attrDef.valueType === 'TEXT' ? val : undefined,
              numberValue: attrDef.valueType === 'NUMBER' ? val : undefined,
              boolValue: attrDef.valueType === 'BOOLEAN' ? val : undefined,
              enumValueId: attrDef.valueSource === 'DICTIONARY' ? val : undefined
            });
          }
        });
      }

      return {
        id: v.id,
        colorId: v.colorId,
        sizeValueId: v.sizeValueId,
        sellerSku: v.sellerSku || undefined,
        barcode: v.barcode || undefined,
        priceCents: v.priceCents || undefined,
        shadeName: v.colorId && state.shadeNamesByColor[v.colorId] ? state.shadeNamesByColor[v.colorId] : undefined,
        attributes: vAttrs.length > 0 ? vAttrs : undefined
      };
    });

    const mappedImages = [
      ...state.commonImages.map(ci => ({ id: ci.id, imageUrl: ci.url, sortOrder: ci.sortOrder, colorId: undefined, isMain: ci.isMain })),
      ...Object.entries(state.colorImages).flatMap(([colorId, imgs]) => 
        imgs.map(ci => ({ id: ci.id, imageUrl: ci.url, sortOrder: ci.sortOrder, colorId, isMain: ci.isMain }))
      )
    ];

    const sizeChartRows = Object.entries(state.sizeChartRows)
      .filter(([_, measures]) => measures && Object.keys(measures).length > 0)
      .map(([sizeId, measures]) => ({
        sizeValueId: sizeId,
        measurements: measures
      }));

    return {
      title: state.title || 'Новый товар',
      currency: 'RUB',
      priceCents: 0,
      description: state.description,
      categoryId: state.categoryId || undefined,
      materialComposition: state.materialComposition.length > 0 ? state.materialComposition : undefined,
      attributes: mappedAttrs.length > 0 ? mappedAttrs : undefined,
      variants: mappedVariants.length > 0 ? mappedVariants : undefined,
      images: mappedImages.length > 0 ? mappedImages : undefined,
      sizeChartRows: sizeChartRows.length > 0 ? sizeChartRows : undefined
    };
  };

  const syncStateFromProduct = (p: any, partialUpdate: Partial<WizardState> = {}) => {
    const updatedVariants = [...state.variants];
    if (p.variants && Array.isArray(p.variants)) {
      p.variants.forEach((pv: any) => {
        const idx = updatedVariants.findIndex(v =>
          (v.id && v.id === pv.id) ||
          (v.colorId === pv.colorId && v.sizeValueId === pv.sizeValueId) ||
          (v.sellerSku && pv.sellerSku && v.sellerSku.trim().toLowerCase() === pv.sellerSku.trim().toLowerCase())
        );
        if (idx !== -1) {
          updatedVariants[idx] = {
            ...updatedVariants[idx],
            id: pv.id,
            barcode: pv.barcode || updatedVariants[idx].barcode,
            sellerSku: pv.sellerSku || updatedVariants[idx].sellerSku,
            priceCents: pv.priceCents !== undefined ? pv.priceCents : updatedVariants[idx].priceCents
          };
        }
      });
    }

    let nextCommon = state.commonImages;
    let nextColorImages = state.colorImages;
    if (p.images && Array.isArray(p.images) && p.images.length > 0) {
      const commonImgs: any[] = [];
      const colorImgs: Record<string, any[]> = {};
      p.images.forEach((img: any) => {
        const mapped = {
          id: img.id,
          url: img.imageUrl,
          sortOrder: img.sortOrder || 0,
          isMain: !!img.isMain,
          isReady: img.renditionUrl != null || (img.cropWidth != null && img.cropHeight != null)
        };
        if (img.colorId) {
          if (!colorImgs[img.colorId]) colorImgs[img.colorId] = [];
          colorImgs[img.colorId].push(mapped);
        } else {
          commonImgs.push(mapped);
        }
      });
      nextCommon = commonImgs;
      nextColorImages = colorImgs;
    }

    const nextState: WizardState = {
      ...state,
      ...partialUpdate,
      id: p.id || state.id,
      variants: updatedVariants,
      commonImages: nextCommon,
      colorImages: nextColorImages
    };
    setState(nextState);
    setLastSavedStateStr(normalizeState(nextState));
    return nextState;
  };

  const handleSaveDraft = async (skipNavigate = false): Promise<string | undefined> => {
    if (savingDraft || submitting) return state.id;
    try {
      setSavingDraft(true);
      const payload = buildPayload();

      if (state.id) {
        const p = await updateSellerProduct(state.id, payload);
        syncStateFromProduct(p);
        showToast('Черновик обновлен', 'success');
        return state.id;
      } else {
        const p = await createSellerProduct(payload);
        syncStateFromProduct(p, { id: p.id });
        showToast('Черновик сохранен', 'success');
        if (!skipNavigate) {
          navigate(`/products/${p.id}/edit`, { replace: true });
        }
        return p.id;
      }
    } catch (err: any) {
      showToast('Ошибка сохранения: ' + (err.message || 'Не удалось сохранить товар'), 'error');
      return undefined;
    } finally {
      setSavingDraft(false);
    }
  };

  const handleInitialSubmit = async () => {
    if (submitting || savingDraft) return;

    if (initialData?.status === 'PUBLISHED') {
      if (priceDirty && !contentDirty) {
        // Price only flow!
        try {
          setSubmitting(true);
          const saved = JSON.parse(lastSavedStateStr);
          const changedVariants = state.variants.filter(v => v.active).filter(v => {
            if (!v.id) return false;
            const sv = saved.variants.find((svv:any) => svv.id === v.id);
            if (!sv) return false;
            return sv.priceCents !== v.priceCents;
          });
          const variantsPayload = changedVariants.map(v => ({ id: v.id, priceCents: v.priceCents || 0 }));
          await updateSellerProductPrices(initialData.id, { variants: variantsPayload });

          showToast('Цены успешно обновлены (модерация не требуется)', 'success');
          setLastSavedStateStr(normalizeState(state));
        } catch (err: any) {
          showToast('Ошибка обновления цен: ' + err.message, 'error');
        } finally {
          setSubmitting(false);
        }
        return;
      }
      setShowPublishedModal(true);
    } else {
      executeSubmit('HIDE');
    }
  };

  const executeSubmit = async (strategy: 'CONTINUE_SELLING' | 'HIDE') => {
    if (submitting) return;
    try {
      setSubmitting(true);
      const payload = buildPayload();

      let targetId = state.id;

      if (initialData?.status === 'PUBLISHED') {
        if (priceDirty) {
          const saved = JSON.parse(lastSavedStateStr);
          const changedVariants = state.variants.filter(v => v.active).filter(v => {
            if (!v.id) return false;
            const sv = saved.variants.find((svv:any) => svv.id === v.id);
            if (!sv) return false;
            return sv.priceCents !== v.priceCents;
          });
          const variantsPayload = changedVariants.map(v => ({ id: v.id, priceCents: v.priceCents || 0 }));
          await updateSellerProductPrices(initialData.id, { variants: variantsPayload });
          
          state.variants.forEach(v => {
            if (!v.id) return;
            const sv = saved.variants.find((svv:any) => svv.id === v.id);
            if (sv) sv.priceCents = v.priceCents;
          });
          setLastSavedStateStr(JSON.stringify(saved));
        }

        const isContinue = strategy === 'CONTINUE_SELLING';
        const p = await updateSellerProduct(initialData.id, { ...payload, continueSelling: isContinue });
        syncStateFromProduct(p);
        targetId = initialData.id;
      } else if (targetId) {
        const p = await updateSellerProduct(targetId, payload);
        syncStateFromProduct(p);
      } else {
        const p = await createSellerProduct(payload);
        syncStateFromProduct(p, { id: p.id });
        targetId = p.id;
      }

      if (!targetId) {
        throw new Error('Не удалось получить идентификатор товара');
      }
      
      await submitSellerProductModeration(targetId, undefined);
      setShowPublishedModal(false);
      showToast('Товар отправлен на модерацию!', 'success');
      navigate('/products');
    } catch (err: any) {
      showToast('Ошибка модерации: ' + (err.message || 'Не удалось отправить на модерацию'), 'error');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-5xl mx-auto py-8">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center space-x-4">
          <button onClick={handleBackNavigation} className="text-gray-500 hover:text-gray-900">
            <ArrowLeft className="w-6 h-6" />
          </button>
          <h1 className="text-3xl font-semibold tracking-tight">
            {isEdit ? 'Редактировать товар' : 'Новый товар'}
          </h1>
        </div>
        <div className="flex items-center space-x-3">
          <button
            onClick={() => handleSaveDraft(false)}
            disabled={savingDraft || submitting || !state.title}
            className="px-4 py-2 border rounded-md shadow-sm text-sm font-medium hover:bg-gray-50 disabled:opacity-50"
          >
            {savingDraft ? 'Сохранение...' : 'Сохранить черновик'}
          </button>
        </div>
      </div>

      <div className="flex">
        <div className="w-64 pr-8">
          <nav aria-label="Progress">
            <ol role="list" className="overflow-hidden">
              {steps.map((stepName, index) => {
                const stepNumber = index + 1;
                const isActive = step === stepNumber;
                const isCompleted = step > stepNumber;

                return (
                  <li key={stepName} className="relative pb-10 last:pb-0">
                    <div className="flex items-center">
                      <span
                        className={`h-8 w-8 rounded-full flex items-center justify-center border-2 ${
                          isActive
                            ? 'border-black bg-black text-white'
                            : isCompleted
                            ? 'border-black bg-black text-white'
                            : 'border-gray-300 bg-white text-gray-500'
                        }`}
                      >
                        {isCompleted ? <CheckCircle2 className="w-5 h-5" /> : stepNumber}
                      </span>
                      <span
                        className={`ml-4 text-sm font-medium ${
                          isActive || isCompleted ? 'text-gray-900' : 'text-gray-500'
                        }`}
                      >
                        {stepName}
                      </span>
                    </div>
                  </li>
                );
              })}
            </ol>
          </nav>
        </div>

        <div className="flex-1 min-w-0 bg-white shadow rounded-lg p-8">
          {step === 1 && <Step1Basics state={state} updateState={updateState} onNext={() => setStep(2)} />}
          {step === 2 && <Step2Category state={state} updateState={updateState} categories={categories} onNext={() => setStep(3)} onPrev={() => setStep(1)} />}
          {step === 3 && <Step3Attributes state={state} updateState={updateState} schema={schema} schemaLoading={schemaLoading} schemaError={schemaError} onRetrySchema={loadSchema} onNext={() => setStep(4)} onPrev={() => setStep(2)} />}
          {step === 4 && <Step4Variants state={state} updateState={updateState} schema={schema} onNext={() => setStep(5)} onPrev={() => setStep(3)} />}
          {step === 5 && <Step5SizeChart state={state} updateState={updateState} schema={schema} onNext={() => setStep(6)} onPrev={() => setStep(4)} />}
          {step === 6 && <Step6Media state={state} updateState={updateState} schema={schema} onNext={() => setStep(7)} onPrev={() => setStep(5)} onError={(msg) => showToast(msg, 'error')} onSaveDraft={() => handleSaveDraft(true)} />}
          {step === 7 && <Step7Pricing state={state} updateState={updateState} onNext={() => setStep(8)} onPrev={() => setStep(6)} onError={(msg) => showToast(msg, 'error')} />}
          {step === 8 && <Step8Review state={state} submitting={submitting} onPrev={() => setStep(7)} onSubmit={handleInitialSubmit} />}
        </div>
      </div>
    
      {showPublishedModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded-lg p-6 max-w-md w-full">
            <h3 className="text-lg font-medium mb-4">Что делать с товаром, пока изменения проходят модерацию?</h3>
            <div className="space-y-3">
              <label className="flex items-start gap-2 cursor-pointer">
                <input type="radio" name="strategy" defaultChecked className="mt-1" />
                <span>Продолжать продавать текущую версию</span>
              </label>
              <label className="flex items-start gap-2 cursor-pointer">
                <input type="radio" name="strategy" className="mt-1" />
                <span>Снять товар с публикации до проверки</span>
              </label>
            </div>
            <div className="mt-6 flex justify-end gap-3">
              <button onClick={() => setShowPublishedModal(false)} className="px-4 py-2 border rounded">Отмена</button>
              <button onClick={() => executeSubmit('CONTINUE_SELLING')} disabled={submitting} className="px-4 py-2 bg-black text-white rounded">Отправить изменения</button>
            </div>
          </div>
        </div>
      )}


      {toast && (
        <div className={`fixed bottom-4 right-4 p-4 rounded shadow-lg text-white ${toast.type === 'error' ? 'bg-red-600' : 'bg-green-600'} z-50 transition-all`}>
          {toast.message}
        </div>
      )}

      {showLeaveModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-[60]">
          <div className="bg-white rounded-lg p-6 max-w-sm w-full">
            <h3 className="text-lg font-medium">Есть несохранённые изменения</h3>
            <p className="mt-2 text-sm text-gray-500">
              Если вы уйдёте со страницы, изменения будут потеряны.
            </p>
            <div className="mt-4 flex justify-end gap-3">
              <button onClick={cancelLeave} className="px-3 py-1.5 border rounded">Остаться</button>
              <button onClick={confirmLeave} className="px-3 py-1.5 bg-red-600 text-white rounded">Выйти без сохранения</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
