import { useMemo, useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  ImagePlus,
  PackageCheck,
  Rocket,
  Save,
  Shirt,
  Sparkles,
  Trash2,
  Wallet,
  Tags,
  Layers,
  CheckCircle2,
  ListPlus
} from 'lucide-react';
import { type SellerProductStatus } from '../lib/seller-products';
import { createSellerProduct, uploadSellerProductImage, getSellerMe, submitSellerProductModeration } from '@zamk/api-client/src/seller';
import { request } from '@zamk/api-client/src/client';
import type { SellerMe } from '@zamk/api-client/src/types';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const formatCurrency = (value: number) => currencyFormatter.format(value);

type OptionConfig = { name: string; values: string };

type VariantDraft = {
  id: string;
  sku: string;
  barcode: string;
  optionValues: Record<string, string>;
};

type DraftProduct = {
  title: string;
  brand: string;
  description: string;
  categoryId: string; // Using ID, simplified for now
  categoryName: string;
  options: OptionConfig[];
  variants: VariantDraft[];
  photos: string[];
  material: string;
  color: string;
  careInstructions: string;
  price: string;
};

const initialDraft: DraftProduct = {
  title: '',
  brand: 'ZAMK Selected',
  description: '',
  categoryId: '',
  categoryName: '',
  options: [{ name: 'Цвет', values: 'Черный, Белый' }, { name: 'Размер', values: 'S, M, L' }],
  variants: [],
  photos: [],
  material: '',
  color: '',
  careInstructions: '',
  price: '',
};

const steps = [
  { id: 'base', label: 'Основное', icon: Shirt },
  { id: 'category', label: 'Категория', icon: Layers },
  { id: 'variants', label: 'Варианты', icon: PackageCheck },
  { id: 'media', label: 'Фото', icon: ImagePlus },
  { id: 'attributes', label: 'Характеристики', icon: Tags },
  { id: 'price', label: 'Цена', icon: Wallet },
  { id: 'review', label: 'Проверка', icon: Sparkles },
] as const;

type StepId = (typeof steps)[number]['id'];

function asNumber(value: string) {
  const parsed = Number(value.replace(/\s/g, '').replace(',', '.'));
  return Number.isFinite(parsed) ? parsed : 0;
}

function calculateQuality(draft: DraftProduct) {
  const checks = [
    draft.title.trim().length >= 8,
    draft.brand.trim().length > 0,
    draft.description.trim().length >= 120,
    draft.categoryName.trim().length > 0,
    draft.variants.length > 0 && draft.variants.every(v => v.sku.trim().length > 0),
    draft.photos.length >= 3,
    draft.material.trim().length > 0,
    draft.color.trim().length > 0,
    asNumber(draft.price) > 0,
  ];

  return Math.round((checks.filter(Boolean).length / checks.length) * 100);
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  textarea,
  required,
  helpText,
  error,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  textarea?: boolean;
  required?: boolean;
  helpText?: string;
  error?: string;
}) {
  const className = cn(
    'seller-setting-input mt-2 w-full rounded-2xl border bg-white/78 px-4 text-sm outline-none transition-all focus:bg-white dark:bg-black/24 dark:focus:bg-black/32',
    error 
      ? 'border-red-500/50 text-red-900 focus:border-red-500 dark:border-red-500/50 dark:text-red-200' 
      : 'border-border-lighter text-graphite focus:border-graphite/30 dark:border-white/16 dark:text-white dark:focus:border-white/32'
  );

  return (
    <label className="block relative">
      <span className={cn("text-[11px] font-semibold uppercase tracking-[0.14em]", error ? "text-red-500" : "text-ash dark:text-white/62")}>
        {label} {required && <span className="text-red-500">*</span>}
      </span>
      {helpText && !error && <p className="mt-1 text-xs text-graphite-light dark:text-white/40">{helpText}</p>}
      {error && <p className="mt-1 text-xs font-medium text-red-500">{error}</p>}
      {textarea ? (
        <textarea value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className={cn(className, 'min-h-[132px] resize-none py-4')} />
      ) : (
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className={cn(className, 'h-12')} />
      )}
    </label>
  );
}

function QualityCheck({ label, passed }: { label: string; passed: boolean }) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-2xl border border-border-lighter bg-white/70 p-4 dark:border-white/16 dark:bg-black/24">
      <span className="text-sm text-graphite dark:text-white/82">{label}</span>
      <span className={cn('rounded-full px-3 py-1 text-xs font-semibold', passed ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300' : 'bg-amber-50 text-amber-700 dark:bg-amber-400/10 dark:text-amber-300')}>
        {passed ? 'Готово' : 'Доделать'}
      </span>
    </div>
  );
}

export function SellerProductNew() {
  const navigate = useNavigate();
  const [draft, setDraft] = useState<DraftProduct>(initialDraft);
  const [activeStep, setActiveStep] = useState<StepId>('base');
  const [photoFiles, setPhotoFiles] = useState<File[]>([]);
  const [savedStatus, setSavedStatus] = useState<SellerProductStatus | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState('');
  const [duplicateSku, setDuplicateSku] = useState<string | null>(null);
  const [sellerMe, setSellerMe] = useState<SellerMe | null>(null);
  const [categories, setCategories] = useState<{id: string, name: string}[]>([]);
  const [brands, setBrands] = useState<{id: string, name: string}[]>([]);

  useEffect(() => {
    getSellerMe().then(setSellerMe).catch(console.error);
    request('GET', '/public/categories').then((res: any) => setCategories(res.items || [])).catch(console.error);
    request('GET', '/public/brands').then((res: any) => setBrands(res.items || [])).catch(console.error);
  }, []);

  const quality = useMemo(() => calculateQuality(draft), [draft]);
  const price = asNumber(draft.price);

  const updateDraft = <K extends keyof DraftProduct>(key: K, value: DraftProduct[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
    setSavedStatus(null);
  };

  const handleVariantChange = (id: string, field: keyof VariantDraft, value: string) => {
    setDraft(curr => ({
      ...curr,
      variants: curr.variants.map(v => v.id === id ? { ...v, [field]: value } : v)
    }));
    if (field === 'sku') {
      setDuplicateSku(null);
    }
  };

  const addVariant = () => {
    setDraft(curr => ({
      ...curr,
      variants: [...curr.variants, { id: Math.random().toString(), sku: '', barcode: '', optionValues: {} }]
    }));
  };

  const removeVariant = (id: string) => {
    if (draft.variants.length <= 1) return;
    setDraft(curr => ({
      ...curr,
      variants: draft.variants.filter(v => v.id !== id)
    }));
  };

  const removePhoto = (photo: string) => {
    updateDraft('photos', draft.photos.filter((item) => item !== photo));
    setPhotoFiles((current) => current.filter((f) => f.name !== photo));
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length === 0) return;
    
    const validFiles = files.filter(f => ['image/jpeg', 'image/png', 'image/webp'].includes(f.type));
    if (validFiles.length < files.length) {
      setError('Некоторые файлы пропущены. Разрешены только JPG, PNG, WEBP.');
    } else {
      setError('');
    }

    setPhotoFiles(curr => [...curr, ...validFiles]);
    updateDraft('photos', [...draft.photos, ...validFiles.map(f => f.name)]);
  };

  const saveProduct = async (status: SellerProductStatus) => {
    setIsSaving(true);
    setError('');
    try {
      const priceCents = asNumber(draft.price) * 100;
      
      const variants = draft.variants.map(v => ({
        optionValues: v.optionValues || {},
        sku: v.sku,
        barcode: v.barcode,
        isActive: true
      }));

      const payload = {
        title: draft.title || 'Новый товар',
        slug: draft.variants[0]?.sku || `slug-${Date.now()}`,
        description: draft.description,
        priceCents,
        currency: 'RUB',
        material: draft.material,
        color: draft.color,
        careInstructions: draft.careInstructions,
        categoryId: (draft.categoryId && draft.categoryId.length === 36) ? draft.categoryId : undefined,
        brandId: (draft.brand && draft.brand.length === 36) ? draft.brand : undefined,
        // Optional attributes that were unused, mapped as empty for now or use default
        variants
      };

      const product = await createSellerProduct(payload);

      // Upload images
      for (const file of photoFiles) {
        try {
          await uploadSellerProductImage(product.id, file);
        } catch (imgErr) {
          console.error('Image upload failed', imgErr);
        }
      }

      if (status === 'pending_moderation') {
        await submitSellerProductModeration(product.id);
      }

      setSavedStatus(status);
      setTimeout(() => navigate('/products'), 1500);
    } catch (err: any) {
      if (err.code === 'duplicate_sku' && err.data?.sku) {
        setError(`SKU ${err.data.sku} уже используется в другом вашем товаре.`);
      } else {
        setError(err.message || 'Ошибка сохранения');
      }
    } finally {
      setIsSaving(false);
    }
  };

  const renderStep = () => {
    switch (activeStep) {
      case 'base':
        return (
          <div className="grid gap-4 md:grid-cols-2">
            <div className="md:col-span-2">
              <Field label="Название товара" required value={draft.title} onChange={(value) => updateDraft('title', value)} placeholder="Например, Жакет мягкой линии" helpText="Отображается в каталоге" />
            </div>
            <div className="md:col-span-2">
              <div className="flex flex-col gap-1">
                <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Бренд *</label>
                <select 
                  className="seller-setting-input h-12 mt-2 w-full rounded-2xl border bg-white/78 px-4 text-sm outline-none transition-all focus:bg-white dark:bg-black/24 dark:focus:bg-black/32 border-border-lighter text-graphite focus:border-graphite/30 dark:border-white/16 dark:text-white dark:focus:border-white/32"
                  value={draft.brand}
                  onChange={(e) => updateDraft('brand', e.target.value)}
                >
                  <option value="">Выберите бренд</option>
                  {brands.map(b => <option key={b.id} value={b.id}>{b.name}</option>)}
                </select>
              </div>
            </div>
            <div className="md:col-span-2">
              <Field label="Описание" required value={draft.description} onChange={(value) => updateDraft('description', value)} textarea placeholder="Опишите посадку, материал, сценарии носки и отличие товара." />
            </div>
          </div>
        );
      
      case 'category':
        return (
          <div className="grid gap-4">
            <p className="text-sm text-graphite-light dark:text-white/60 mb-2">Выберите категорию, в которой покупатели смогут найти ваш товар.</p>
            <div className="flex flex-col gap-1">
              <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Категория *</label>
              <select 
                className="seller-setting-input h-12 mt-2 w-full rounded-2xl border bg-white/78 px-4 text-sm outline-none transition-all focus:bg-white dark:bg-black/24 dark:focus:bg-black/32 border-border-lighter text-graphite focus:border-graphite/30 dark:border-white/16 dark:text-white dark:focus:border-white/32"
                value={draft.categoryId}
                onChange={(e) => {
                  const val = e.target.value;
                  updateDraft('categoryId', val);
                  const cat = categories.find(c => c.id === val);
                  updateDraft('categoryName', cat ? cat.name : '');
                }}
              >
                <option value="">Выберите категорию</option>
                {categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
            </div>
          </div>
        );

      case 'variants': {
        const generateVariants = () => {
          if (!draft.options || draft.options.length === 0) return;
          const dims = draft.options.map(opt => ({
            name: opt.name.trim(),
            values: opt.values.split(',').map(s => s.trim()).filter(Boolean)
          })).filter(d => d.name && d.values.length > 0);
          if (dims.length === 0) return;
          const cartesian = (arrays: any[]) => arrays.reduce((a, b) => a.flatMap((d: any) => b.map((e: any) => [d, e].flat())));
          const combinations = dims.length === 1 ? dims[0].values.map(v => [v]) : cartesian(dims.map(d => d.values));
          const newVariants = combinations.map((combo: any, i: number) => {
            const optionValues: Record<string, string> = {};
            dims.forEach((d, dimIdx) => {
              optionValues[d.name] = combo[dimIdx];
            });
            return {
              id: Math.random().toString(),
              sku: `ZMK-${Date.now().toString().slice(-5)}-${i}`,
              barcode: '',
              optionValues
            };
          });
          updateDraft('variants', newVariants);
        };

        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <p className="text-sm text-graphite-light dark:text-white/60">Каждый вариант будет отдельной единицей на складе.</p>
              <button type="button" onClick={addVariant} className="inline-flex items-center gap-2 text-sm text-graphite hover:text-black dark:text-white/80 dark:hover:text-white">
                <ListPlus className="h-4 w-4" /> Добавить вариант
              </button>
            </div>
            
            <div className="mb-6 p-5 border rounded-2xl bg-white/50 dark:bg-black/20 dark:border-white/10 space-y-4">
              <h4 className="text-sm font-semibold text-graphite dark:text-white">Генерация вариантов (Опции)</h4>
              {draft.options.map((opt, idx) => (
                <div key={idx} className="flex gap-4">
                  <Field label="Название опции" value={opt.name} onChange={(val) => {
                    const opts = [...draft.options]; opts[idx].name = val; updateDraft('options', opts);
                  }} placeholder="Например, Цвет" />
                  <Field label="Значения (через запятую)" value={opt.values} onChange={(val) => {
                    const opts = [...draft.options]; opts[idx].values = val; updateDraft('options', opts);
                  }} placeholder="Черный, Белый" />
                </div>
              ))}
              <div className="flex gap-4 pt-2">
                <button type="button" onClick={() => updateDraft('options', [...draft.options, { name: '', values: '' }])} className="text-sm text-blue-600 dark:text-blue-400">
                  + Добавить опцию
                </button>
                <button type="button" onClick={generateVariants} className="text-sm text-emerald-600 dark:text-emerald-400 font-semibold">
                  Сгенерировать ({draft.variants.length > 0 ? 'пересоздаст список' : 'создаст варианты'})
                </button>
              </div>
            </div>

            <div className="space-y-4">
              {draft.variants.map((variant, index) => (
                <div key={variant.id} className="relative rounded-2xl border border-border-lighter bg-white/72 p-5 dark:border-white/16 dark:bg-black/24">
                  {draft.variants.length > 1 && (
                    <button type="button" onClick={() => removeVariant(variant.id)} className="absolute right-4 top-4 text-ash hover:text-red-500">
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                  <h4 className="text-sm font-semibold mb-3 text-graphite dark:text-white">Вариант {index + 1}</h4>
                  
                  {variant.optionValues && Object.keys(variant.optionValues).length > 0 && (
                    <div className="flex flex-wrap gap-2 mb-4">
                      {Object.entries(variant.optionValues).map(([k, v]) => (
                        <span key={k} className="px-2 py-1 bg-black/5 dark:bg-white/10 rounded text-xs font-medium text-graphite dark:text-white/80">
                          {k}: {v}
                        </span>
                      ))}
                    </div>
                  )}

                  <div className="grid gap-4 md:grid-cols-2">
                    <Field 
                      label="Артикул (SKU)" 
                      required 
                      value={variant.sku} 
                      onChange={(val) => handleVariantChange(variant.id, 'sku', val)} 
                      placeholder="ZMK-12345" 
                      helpText="Идентификатор варианта"
                      error={variant.sku === duplicateSku ? "SKU уже используется в другом товаре" : undefined}
                    />
                    <Field label="Штрихкод (Barcode)" value={variant.barcode} onChange={(val) => handleVariantChange(variant.id, 'barcode', val)} placeholder="Например, 460000000000" helpText="Для приемки на складе" />
                  </div>
                </div>
              ))}
            </div>
          </div>
        );
      }

      case 'media':
        return (
          <div>
            <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
              <input
                type="file"
                multiple
                accept="image/jpeg, image/png, image/webp"
                onChange={handleFileChange}
                className="block w-full text-sm text-slate-500
                  file:mr-4 file:py-2 file:px-4
                  file:rounded-full file:border-0
                  file:text-sm file:font-semibold
                  file:bg-graphite file:text-white
                  hover:file:bg-black cursor-pointer dark:file:bg-white dark:file:text-black"
              />
            </div>
            <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {draft.photos.map((photo) => (
                <div key={photo} className="flex items-center justify-between gap-3 rounded-2xl border border-border-lighter bg-white/72 p-4 dark:border-white/16 dark:bg-black/24">
                  <span className="text-sm text-graphite dark:text-white truncate" title={photo}>{photo}</span>
                  <button type="button" onClick={() => removePhoto(photo)} className="text-ash hover:text-red-500 dark:hover:text-red-300 shrink-0">
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
              {draft.photos.length === 0 && <p className="text-sm text-graphite-light dark:text-white/68">Добавьте минимум 3 фото: товар, модель, деталь.</p>}
            </div>
          </div>
        );

      case 'attributes':
        return (
          <div className="grid gap-4 md:grid-cols-2">
            <Field label="Цвет" required value={draft.color} onChange={(value) => updateDraft('color', value)} placeholder="Например, Синий" />
            <Field label="Материал" required value={draft.material} onChange={(value) => updateDraft('material', value)} placeholder="Например, 100% Хлопок" />
            <div className="md:col-span-2">
              <Field label="Инструкция по уходу" value={draft.careInstructions} onChange={(value) => updateDraft('careInstructions', value)} textarea placeholder="Как стирать и ухаживать за товаром" />
            </div>
          </div>
        );

      case 'price':
        return (
          <div className="grid gap-4 md:grid-cols-2">
            <div className="md:col-span-2">
              <Field label="Цена продажи (₽)" required value={draft.price} onChange={(value) => updateDraft('price', value)} placeholder="14900" helpText="Итоговая цена, которую увидит покупатель" />
            </div>
          </div>
        );

      case 'review':
        return (
          <div className="grid gap-3">
            <QualityCheck label="Название и бренд заполнены" passed={draft.title.trim().length >= 8 && draft.brand.trim().length > 0} />
            <QualityCheck label="Описание длиннее 120 символов" passed={draft.description.trim().length >= 120} />
            <QualityCheck label="Категория выбрана" passed={draft.categoryName.trim().length > 0} />
            <QualityCheck label="Добавлены варианты со SKU" passed={draft.variants.length > 0 && draft.variants.every(v => v.sku.trim().length > 0)} />
            <QualityCheck label="Добавлено минимум 3 фото" passed={draft.photos.length >= 3} />
            <QualityCheck label="Заполнены характеристики (Цвет, Материал)" passed={draft.color.trim().length > 0 && draft.material.trim().length > 0} />
            <QualityCheck label="Цена установлена" passed={price > 0} />
          </div>
        );
    }
  };

  return (
    <div className="relative z-10 min-h-screen pt-24 pb-24 md:pt-28 md:pb-20">
      <div className="container mx-auto max-w-[1280px] px-4 sm:px-6">
        <Link to="/seller-products" className="inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white">
          <ArrowLeft className="h-4 w-4" />
          Мои товары
        </Link>

        <section className="mt-6 glass-panel-strong p-7 md:p-10">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="studio-label">Новая карточка</p>
              <h1 className="mt-3 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-5xl">Добавить товар</h1>
              <p className="studio-subtitle mt-4 max-w-3xl">
                Пройдите все шаги для создания товара. Товар появится в каталоге после модерации и приемки на склад ZAMK.
              </p>
            </div>
            <div className="rounded-[2rem] border border-border-lighter bg-white/72 p-5 dark:border-white/16 dark:bg-black/26 min-w-[200px]">
              <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Готовность карточки</p>
              <p className="mt-2 text-4xl font-semibold text-graphite dark:text-white">{quality}%</p>
              <div className="mt-3 h-2 overflow-hidden rounded-full bg-ice dark:bg-white/10">
                <div className={cn('h-full rounded-full', quality >= 80 ? 'bg-emerald-500 dark:bg-emerald-300' : quality >= 60 ? 'bg-amber-500 dark:bg-amber-300' : 'bg-red-500 dark:bg-red-300')} style={{ width: `${quality}%` }} />
              </div>
            </div>
          </div>
        </section>

        <div className="mt-6 grid gap-6 lg:grid-cols-[250px_1fr_300px]">
          {/* Sidebar Navigation */}
          <aside className="hidden lg:flex flex-col gap-2">
            {steps.map((step) => {
              const Icon = step.icon;
              const isActive = activeStep === step.id;

              return (
                <button
                  key={step.id}
                  type="button"
                  onClick={() => setActiveStep(step.id)}
                  className={cn(
                    'flex items-center gap-3 rounded-2xl border p-4 text-left transition-all',
                    isActive
                      ? 'border-graphite/30 bg-white text-graphite dark:border-white/32 dark:bg-white/10 dark:text-white'
                      : 'border-transparent text-graphite-light hover:bg-white/50 dark:text-white/68 dark:hover:bg-white/8'
                  )}
                >
                  <Icon className={cn("h-5 w-5 shrink-0", isActive ? "text-graphite dark:text-white" : "text-ash dark:text-white/40")} />
                  <span className="text-sm font-semibold">{step.label}</span>
                </button>
              );
            })}
          </aside>

          {/* Mobile Navigation */}
          <section className="lg:hidden flex overflow-x-auto pb-2 gap-2 snap-x">
            {steps.map((step) => {
              const Icon = step.icon;
              const isActive = activeStep === step.id;

              return (
                <button
                  key={step.id}
                  type="button"
                  onClick={() => setActiveStep(step.id)}
                  className={cn(
                    'flex items-center gap-2 rounded-xl border p-3 text-left transition-all snap-start whitespace-nowrap',
                    isActive
                      ? 'border-graphite/30 bg-white text-graphite dark:border-white/32 dark:bg-white/10 dark:text-white'
                      : 'border-border-lighter bg-white/70 text-graphite-light hover:bg-white dark:border-white/16 dark:bg-black/24 dark:text-white/68 dark:hover:bg-white/8'
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  <span className="text-xs font-semibold">{step.label}</span>
                </button>
              );
            })}
          </section>

          {/* Main Content Area */}
          <section className="glass-panel-strong p-6 md:p-8">
            <div className="mb-6 pb-6 border-b border-border-lighter dark:border-white/10 flex items-center justify-between">
              <h2 className="text-2xl font-serif text-graphite dark:text-white">
                {steps.find(s => s.id === activeStep)?.label}
              </h2>
            </div>
            
            {renderStep()}

            <div className="mt-8 pt-6 border-t border-border-lighter dark:border-white/10 flex justify-between">
              {activeStep !== steps[0].id ? (
                <button 
                  type="button" 
                  onClick={() => {
                    const currentIndex = steps.findIndex(s => s.id === activeStep);
                    setActiveStep(steps[currentIndex - 1].id);
                  }}
                  className="px-6 py-2 text-sm font-semibold text-graphite-light hover:text-graphite dark:text-white/60 dark:hover:text-white transition-colors"
                >
                  Назад
                </button>
              ) : <div></div>}

              {activeStep !== steps[steps.length - 1].id ? (
                <button 
                  type="button" 
                  onClick={() => {
                    const currentIndex = steps.findIndex(s => s.id === activeStep);
                    setActiveStep(steps[currentIndex + 1].id);
                  }}
                  className="px-6 py-2 bg-graphite text-white rounded-full text-sm font-semibold hover:bg-graphite-light transition-colors dark:bg-white dark:text-black dark:hover:bg-white/80"
                >
                  Далее
                </button>
              ) : <div></div>}
            </div>
          </section>

          {/* Right Sidebar - Preview & Actions */}
          <aside className="flex flex-col gap-6">
            <div className="glass-panel-strong p-6">
              <p className="studio-label mb-2">Превью</p>
              <h3 className="text-lg font-semibold text-graphite dark:text-white leading-tight">
                {draft.title || 'Название не указано'}
              </h3>
              <p className="text-sm text-graphite-light dark:text-white/60 mt-1">
                {draft.categoryName || 'Категория не выбрана'} · {draft.brand || 'Бренд'}
              </p>
              
              <div className="mt-4 pt-4 border-t border-border-lighter dark:border-white/10">
                <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Цена</p>
                <p className="mt-1 text-xl font-semibold text-graphite dark:text-white">
                  {price ? formatCurrency(price) : 'Не задана'}
                </p>
              </div>

              <div className="mt-4 flex flex-wrap gap-2">
                <span className="inline-flex items-center gap-1 rounded-full bg-ice/50 px-2 py-1 text-xs text-graphite-light dark:bg-white/5 dark:text-white/60">
                  <PackageCheck className="h-3 w-3" />
                  {draft.variants.length} вариант(ов)
                </span>
                <span className="inline-flex items-center gap-1 rounded-full bg-ice/50 px-2 py-1 text-xs text-graphite-light dark:bg-white/5 dark:text-white/60">
                  <ImagePlus className="h-3 w-3" />
                  {draft.photos.length} фото
                </span>
              </div>
            </div>

            <div className="glass-panel-strong p-6 flex flex-col gap-3">
              {sellerMe?.seller.status === 'blocked' || sellerMe?.seller.status === 'archived' ? (
                <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800 text-center">
                  Действия недоступны из-за статуса магазина.
                </div>
              ) : (
                <>
                  <button
                    type="button"
                    disabled={isSaving}
                    onClick={() => saveProduct('draft')}
                    className="inline-flex h-12 w-full items-center justify-center gap-2 rounded-full border border-border-lighter bg-white/75 px-6 text-sm font-semibold text-graphite transition-colors hover:bg-white disabled:opacity-50 dark:border-white/16 dark:bg-white/8 dark:text-white dark:hover:bg-white/12"
                  >
                    <Save className="h-4 w-4" />
                    Сохранить черновик
                  </button>
                  <button
                    type="button"
                    disabled={isSaving || sellerMe?.seller.status === 'pending'}
                    onClick={() => saveProduct('pending_moderation')}
                    className="inline-flex h-12 w-full items-center justify-center gap-2 rounded-full bg-graphite px-6 text-sm font-semibold text-white transition-colors hover:bg-graphite-light disabled:opacity-50 dark:bg-white dark:text-black dark:hover:bg-white/86"
                  >
                    <Rocket className="h-4 w-4" />
                    На модерацию
                  </button>
                </>
              )}
              
              {savedStatus && (
                <div className="mt-2 rounded-xl bg-emerald-50 p-3 text-xs text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300 text-center flex items-center justify-center gap-2">
                  <CheckCircle2 className="h-4 w-4" />
                  Сохранено успешно
                </div>
              )}
              {error && (
                <div className="mt-2 rounded-xl bg-red-50 p-3 text-xs text-red-700 dark:bg-red-400/10 dark:text-red-300 text-center">
                  {error}
                </div>
              )}
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}
