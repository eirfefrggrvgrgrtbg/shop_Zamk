import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, CheckCircle2 } from 'lucide-react';
import { getSellerCategories, getSellerCategorySchema, createSellerProduct, updateSellerProduct, submitSellerProductModeration, type SellerCategory, type SellerCategorySchema } from '@zamk/api-client/src/seller';

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



  // Add dirtiness check
  const [initialStateStr] = useState(() => JSON.stringify(initialData || {}));
  useEffect(() => {
    const isDirty = JSON.stringify(state) !== initialStateStr;
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isDirty) {
        e.preventDefault();
        e.returnValue = '';
      }
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [state, initialStateStr]);

  const [categories, setCategories] = useState<SellerCategory[]>([]);
  const [schema, setSchema] = useState<SellerCategorySchema | null>(null);

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
          const mapped = { url: img.imageUrl, sortOrder: img.sortOrder || 0 };
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

      setState({
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
      });
    }
  }, [isEdit, initialData]);

  useEffect(() => {
    getSellerCategories().then(setCategories).catch(console.error);
  }, []);

  useEffect(() => {
    if (state.categoryId) {
      getSellerCategorySchema(state.categoryId).then(setSchema).catch(console.error);
    }
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
      ...state.commonImages.map(ci => ({ imageUrl: ci.url, sortOrder: ci.sortOrder, colorId: undefined })),
      ...Object.entries(state.colorImages).flatMap(([colorId, imgs]) => 
        imgs.map(ci => ({ imageUrl: ci.url, sortOrder: ci.sortOrder, colorId }))
      )
    ];

    const sizeChartRows = Object.entries(state.sizeChartRows).map(([sizeId, measures]) => ({
      sizeValueId: sizeId,
      measurements: measures
    }));

    return {
      title: state.title,
      description: state.description,
      categoryId: state.categoryId || undefined,
      materialComposition: state.materialComposition.length > 0 ? state.materialComposition : undefined,
      attributes: mappedAttrs.length > 0 ? mappedAttrs : undefined,
      variants: mappedVariants.length > 0 ? mappedVariants : undefined,
      images: mappedImages.length > 0 ? mappedImages : undefined,
      sizeChartRows: sizeChartRows.length > 0 ? sizeChartRows : undefined
    };
  };

  const handleSaveDraft = async () => {
    try {
      setSavingDraft(true);
      const payload = buildPayload();
      
      if (isEdit && initialData?.id) {
        await updateSellerProduct(initialData.id, payload);
        showToast('Черновик обновлен', 'success');
      } else {
        const p = await createSellerProduct(payload);
        showToast('Черновик сохранен', 'success');
        navigate(`/products/${p.id}/edit`, { replace: true });
      }
    } catch (err: any) {
      showToast('Ошибка сохранения: ' + err.message, 'error');
    } finally {
      setSavingDraft(false);
    }
  };

  
  const handleInitialSubmit = () => {
    if (!isEdit || !initialData?.id) {
      showToast('Сначала сохраните черновик перед модерацией.', 'error');
      return;
    }
    if (initialData?.status === 'PUBLISHED') {
      setShowPublishedModal(true);
    } else {
      executeSubmit('HIDE');
    }
  };


  const executeSubmit = async (strategy: 'CONTINUE_SELLING' | 'HIDE') => {
    try {
      setSubmitting(true);
      if (initialData?.status === 'PUBLISHED') {
        const isContinue = strategy === 'CONTINUE_SELLING';
        await updateSellerProduct(initialData.id, { continueSelling: isContinue });
      }
      await submitSellerProductModeration(initialData.id, undefined);
      setShowPublishedModal(false);
      showToast('Товар отправлен на модерацию!', 'success');
      navigate('/products');
    } catch (err: any) {
      showToast('Ошибка модерации: ' + err.message, 'error');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-5xl mx-auto py-8">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center space-x-4">
          <button onClick={() => navigate('/products')} className="text-gray-500 hover:text-gray-900">
            <ArrowLeft className="w-6 h-6" />
          </button>
          <h1 className="text-3xl font-semibold tracking-tight">
            {isEdit ? 'Редактировать товар' : 'Новый товар'}
          </h1>
        </div>
        <div className="flex items-center space-x-3">
          <button
            onClick={handleSaveDraft}
            disabled={savingDraft || !state.title}
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
          {step === 3 && <Step3Attributes state={state} updateState={updateState} schema={schema} onNext={() => setStep(4)} onPrev={() => setStep(2)} />}
          {step === 4 && <Step4Variants state={state} updateState={updateState} schema={schema} onNext={() => setStep(5)} onPrev={() => setStep(3)} />}
          {step === 5 && <Step5SizeChart state={state} updateState={updateState} schema={schema} onNext={() => setStep(6)} onPrev={() => setStep(4)} />}
          {step === 6 && <Step6Media state={state} updateState={updateState} schema={schema} onNext={() => setStep(7)} onPrev={() => setStep(5)} />}
          {step === 7 && <Step7Pricing state={state} updateState={updateState} onNext={() => setStep(8)} onPrev={() => setStep(6)} />}
          {step === 8 && <Step8Review state={state} onPrev={() => setStep(7)} onSubmit={handleInitialSubmit} />}
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
    </div>
  );
};
