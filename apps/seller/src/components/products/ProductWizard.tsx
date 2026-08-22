import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  CheckCircle2,
} from 'lucide-react';
import {
  createSellerProduct,
  updateSellerProduct,
} from '@zamk/api-client/src/seller';



export type ProductWizardProps = {
  initialData?: any; // If editing
  isEdit?: boolean;
};

export const ProductWizard: React.FC<ProductWizardProps> = ({ initialData, isEdit }) => {
  const navigate = useNavigate();
  const [step, setStep] = useState(1);
    const [savingDraft, setSavingDraft] = useState(false);

  // Form State
  const [title, setTitle] = useState(initialData?.title || '');
  const [description, setDescription] = useState(initialData?.description || '');
  
  const [categoryId] = useState<string>(initialData?.categoryId || '');


  // Steps Configuration
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

  // Helper to handle Save Draft
  const handleSaveDraft = async () => {
    try {
      setSavingDraft(true);
      // Construct payload
      const payload = {
        title,
        description,
        categoryId: categoryId || undefined,
        // TODO: Map other fields
      };
      
      if (isEdit && initialData?.id) {
        await updateSellerProduct(initialData.id, payload);
        console.log('Черновик обновлен');
      } else {
        const p = await createSellerProduct(payload);
        console.log('Черновик сохранен');
        navigate(`/products/${p.id}/edit`, { replace: true });
      }
    } catch (err: any) {
      console.error('Ошибка сохранения черновика', { description: err.message });
    } finally {
      setSavingDraft(false);
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
            disabled={savingDraft || !title}
            className="px-4 py-2 border rounded-md shadow-sm text-sm font-medium hover:bg-gray-50 disabled:opacity-50"
          >
            {savingDraft ? 'Сохранение...' : 'Сохранить черновик'}
          </button>
        </div>
      </div>

      <div className="flex">
        {/* Stepper Sidebar */}
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

        {/* Form Content */}
        <div className="flex-1 min-w-0 bg-white shadow rounded-lg p-8">
          {step === 1 && (
            <div className="space-y-6">
              <h2 className="text-xl font-medium">Основное</h2>
              <div>
                <label className="block text-sm font-medium text-gray-700">Название *</label>
                <input
                  type="text"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-black focus:ring-black sm:text-sm"
                  placeholder="Например: Футболка базовая"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Описание *</label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={6}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-black focus:ring-black sm:text-sm"
                  placeholder="Подробное описание товара..."
                />
              </div>
              
              <div className="pt-5 flex justify-end">
                <button
                  onClick={() => setStep(2)}
                  disabled={!title || !description}
                  className="px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-black hover:bg-gray-800 disabled:opacity-50"
                >
                  Далее
                </button>
              </div>
            </div>
          )}
          
          {step > 1 && (
             <div className="space-y-6">
                <h2 className="text-xl font-medium">{steps[step - 1]}</h2>
                <p className="text-gray-500">В разработке (для демонстрации)</p>
                <div className="pt-5 flex justify-between">
                  <button
                    onClick={() => setStep(step - 1)}
                    className="px-4 py-2 border rounded-md shadow-sm text-sm font-medium hover:bg-gray-50"
                  >
                    Назад
                  </button>
                  <button
                    onClick={() => setStep(step + 1)}
                    disabled={step === 8}
                    className="px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-black hover:bg-gray-800 disabled:opacity-50"
                  >
                    Далее
                  </button>
                </div>
             </div>
          )}
        </div>
      </div>
    </div>
  );
};
