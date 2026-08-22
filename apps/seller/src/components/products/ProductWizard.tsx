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
    const [state, setState] = useState<WizardState>({ ...initialWizardState, ...initialData });

  const [categories, setCategories] = useState<SellerCategory[]>([]);
  const [schema, setSchema] = useState<SellerCategorySchema | null>(null);

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
    return {
      title: state.title,
      description: state.description,
      categoryId: state.categoryId || undefined,
      materialComposition: state.materialComposition,
      variants: state.variants.filter(v => v.active).map(v => ({
        id: v.id,
        colorId: v.colorId,
        sizeValueId: v.sizeValueId,
        sellerSku: v.sellerSku,
        barcode: v.barcode,
        priceCents: v.priceCents
      })),
      // Mapped generic attributes
      attributes: Object.entries(state.productAttributes).map(([defId, val]) => {
        const attrDef = schema?.attributes.find(a => a.id === defId);
        return {
          attributeDefinitionId: defId,
          textValue: attrDef?.valueType === 'TEXT' ? val : undefined,
          numberValue: attrDef?.valueType === 'NUMBER' ? val : undefined,
          boolValue: attrDef?.valueType === 'BOOLEAN' ? val : undefined,
          enumValueId: attrDef?.valueType === 'DICTIONARY' ? val : undefined
        };
      })
    };
  };

  const handleSaveDraft = async () => {
    try {
      setSavingDraft(true);
      const payload = buildPayload();
      
      if (isEdit && initialData?.id) {
        await updateSellerProduct(initialData.id, payload);
        alert('Черновик обновлен');
      } else {
        const p = await createSellerProduct(payload);
        alert('Черновик сохранен');
        navigate(`/products/${p.id}/edit`, { replace: true });
      }
    } catch (err: any) {
      alert('Ошибка сохранения черновика: ' + err.message);
    } finally {
      setSavingDraft(false);
    }
  };

  
  const handleInitialSubmit = () => {
    if (!isEdit || !initialData?.id) {
      alert('Сначала сохраните черновик перед модерацией.');
      return;
    }
    if (initialData?.status === 'PUBLISHED') {
      setShowPublishedModal(true);
    } else {
      executeSubmit('HIDE');
    }
  };

  const executeSubmit = async (_strategy: 'CONTINUE_SELLING' | 'HIDE') => {
    try {
      setSubmitting(true);
      // In a real app we'd pass the strategy. The API currently just takes comment.
      await submitSellerProductModeration(initialData.id);
      setShowPublishedModal(false);
      alert('Товар отправлен на модерацию!');
      navigate('/products');
    } catch (err: any) {
      alert('Ошибка модерации: ' + err.message);
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

    </div>
  );
};
