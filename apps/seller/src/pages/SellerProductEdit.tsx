import { useEffect, useMemo, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
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
import { getSellerMe, getSellerProduct, updateSellerProduct, uploadSellerProductImage, getModerationHistory, deleteSellerProductImage } from '@zamk/api-client/src/seller';
import { request } from '@zamk/api-client/src/client';
import type { SellerMe } from '@zamk/api-client/src/types';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const formatCurrency = (value: number) => currencyFormatter.format(value);

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
  variants: VariantDraft[];
  photos: { id: string, url: string, isNew: boolean }[];
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
  variants: [
    { id: '1', optionValues: { 'Размер': 'Единый' }, sku: `ZMK-${Date.now().toString().slice(-5)}-OS`, barcode: '' }
  ],
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
  disabled,
  required,
  helpText,
  error,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  textarea?: boolean;
  disabled?: boolean;
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
        <textarea disabled={disabled} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className={cn(className, 'min-h-[132px] resize-none py-4', disabled && 'opacity-60 cursor-not-allowed')} />
      ) : (
        <input disabled={disabled} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className={cn(className, 'h-12', disabled && 'opacity-60 cursor-not-allowed')} />
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

export function SellerProductEdit() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [draft, setDraft] = useState<DraftProduct>(initialDraft);
  const [activeStep, setActiveStep] = useState<StepId>('base');
  const [photoFiles, setPhotoFiles] = useState<File[]>([]);
  const [deletedPhotoIds, setDeletedPhotoIds] = useState<string[]>([]);
  const [savedStatus, setSavedStatus] = useState<SellerProductStatus | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [duplicateSku, setDuplicateSku] = useState<string | null>(null);
  const [sellerMe, setSellerMe] = useState<SellerMe | null>(null);
  const [productStatus, setProductStatus] = useState<string>('');
  const [rejectionReason, setRejectionReason] = useState<string>('');
  const [moderationLogs, setModerationLogs] = useState<any[]>([]);
  const [categories, setCategories] = useState<{id: string, name: string}[]>([]);
  const [brands, setBrands] = useState<{id: string, name: string}[]>([]);

  const isReadOnly = ['pending_moderation', 'approved', 'published', 'hidden', 'blocked'].includes(productStatus);

  useEffect(() => {
    getSellerMe().then(setSellerMe).catch(console.error);
    request('GET', '/public/categories').then((res: any) => setCategories(res.items || [])).catch(console.error);
    request('GET', '/public/brands').then((res: any) => setBrands(res.items || [])).catch(console.error);
  }, []);
  
  useEffect(() => {
    async function load() {
      if (!id) return;
      try {
        const [product, history] = await Promise.all([
          getSellerProduct(id),
          getModerationHistory(id).catch(() => ({ items: [] }))
        ]);
        
        setProductStatus(product.status);
        if (product.moderationComment) {
          setRejectionReason(product.moderationComment);
        }
        setModerationLogs(history.items || []);
        
        const variants = product.variants?.map((v: any) => ({
          id: v.id || Math.random().toString(),
          optionValues: v.optionValues || {},
          sku: v.sku || '',
          barcode: v.barcode || '',
        })) || [];
        
        if (variants.length === 0) {
            variants.push({ id: '1', optionValues: { 'Размер': 'Единый' }, sku: `ZMK-${Date.now().toString().slice(-5)}-OS`, barcode: '' });
        }
        
        setDraft({
          title: product.title,
          categoryId: product.categoryId || '',
          categoryName: (product as any).categoryName || 'Одежда',
          brand: product.brandId || 'ZAMK',
          description: product.description || '',
          price: (product.priceCents / 100).toString(),
          material: product.material || '',
          color: product.color || '',
          careInstructions: (product as any).careInstructions || '',
          photos: product.images?.map((img: any) => ({ id: img.id, url: img.imageUrl, isNew: false })) || [],
          variants
        });
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки товара');
      } finally {
        setIsLoading(false);
      }
    }
    load();
  }, [id]);

  const quality = useMemo(() => calculateQuality(draft), [draft]);
  const price = asNumber(draft.price);

  if (isLoading) {
    return <div className="min-h-screen pt-24 pb-24 md:pt-28 md:pb-20 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

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
      variants: [...curr.variants, { id: `0.${Date.now()}`, sku: '', barcode: '', optionValues: {} }]
    }));
  };

  const removeVariant = (id: string) => {
    if (isReadOnly || draft.variants.length <= 1) return;
    setDraft(curr => ({
      ...curr,
      variants: curr.variants.filter(v => v.id !== id)
    }));
  };

  const removePhoto = (photoId: string, isNew: boolean) => {
    if (isReadOnly) return;
    updateDraft('photos', draft.photos.filter((item) => item.id !== photoId));
    if (isNew) {
      setPhotoFiles((current) => current.filter((f) => f.name !== photoId));
    } else {
      setDeletedPhotoIds((current) => [...current, photoId]);
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (isReadOnly) return;
    const files = Array.from(e.target.files || []);
    if (files.length === 0) return;
    
    const validFiles = files.filter(f => ['image/jpeg', 'image/png', 'image/webp'].includes(f.type));
    if (validFiles.length < files.length) {
      setError('Некоторые файлы пропущены. Разрешены только JPG, PNG, WEBP.');
    } else {
      setError('');
    }

    setPhotoFiles(curr => [...curr, ...validFiles]);
    updateDraft('photos', [
      ...draft.photos, 
      ...validFiles.map(f => ({ id: f.name, url: URL.createObjectURL(f), isNew: true }))
    ]);
  };

  const saveProduct = async (status: SellerProductStatus) => {
    setIsSaving(true);
    setError('');
    try {
      const priceCents = asNumber(draft.price) * 100;
      
      const variants = draft.variants.map(v => ({
        id: v.id.startsWith('0.') ? undefined : v.id, // Only send ID if it exists
        optionValues: v.optionValues || {},
        sku: v.sku,
        barcode: v.barcode,
        isActive: true
      }));

      const payload = {
        title: draft.title || 'Новый товар',
        categoryId: (draft.categoryId && draft.categoryId.length === 36) ? draft.categoryId : undefined,
        brandId: (draft.brand && draft.brand.length === 36) ? draft.brand : undefined,
        slug: draft.variants[0]?.sku || `slug-${Date.now()}`,
        description: draft.description,
        priceCents,
        currency: 'RUB',
        material: draft.material,
        color: draft.color,
        careInstructions: draft.careInstructions,
        variants
      };

      await updateSellerProduct(id as string, payload);

      for (const deletedId of deletedPhotoIds) {
        try {
          await deleteSellerProductImage(id as string, deletedId);
        } catch (imgErr) {
          console.error('Failed to delete image', imgErr);
        }
      }

      for (const file of photoFiles) {
        try {
          await uploadSellerProductImage(id as string, file);
        } catch (imgErr) {
          console.error('Image upload failed', imgErr);
        }
      }

      if (status === 'pending_moderation') {
        await request('POST', `/seller/products/${id}/submit-moderation`);
      }

      setSavedStatus(status);
      if (status === 'pending_moderation') {
        setTimeout(() => navigate('/products'), 1500);
      }
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
              <Field disabled={isReadOnly} label="Название товара" required value={draft.title} onChange={(value) => updateDraft('title', value)} placeholder="Например, Жакет мягкой линии" helpText="Отображается в каталоге" />
            </div>
            <div className="md:col-span-2">
              <div className="flex flex-col gap-1">
              <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Бренд *</label>
              <select 
                disabled={isReadOnly}
                className={cn('seller-setting-input h-12 mt-2 w-full rounded-2xl border bg-white/78 px-4 text-sm outline-none transition-all focus:bg-white dark:bg-black/24 dark:focus:bg-black/32 border-border-lighter text-graphite focus:border-graphite/30 dark:border-white/16 dark:text-white dark:focus:border-white/32', isReadOnly && 'opacity-60 cursor-not-allowed')}
                value={draft.brand}
                onChange={(e) => updateDraft('brand', e.target.value)}
              >
                <option value="">Выберите бренд</option>
                {brands.map(b => <option key={b.id} value={b.id}>{b.name}</option>)}
              </select>
            </div>
            </div>
            <div className="md:col-span-2">
              <Field disabled={isReadOnly} label="Описание" required value={draft.description} onChange={(value) => updateDraft('description', value)} textarea placeholder="Опишите посадку, материал, сценарии носки и отличие товара." />
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
                disabled={isReadOnly}
                className={cn('seller-setting-input h-12 mt-2 w-full rounded-2xl border bg-white/78 px-4 text-sm outline-none transition-all focus:bg-white dark:bg-black/24 dark:focus:bg-black/32 border-border-lighter text-graphite focus:border-graphite/30 dark:border-white/16 dark:text-white dark:focus:border-white/32', isReadOnly && 'opacity-60 cursor-not-allowed')}
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

      case 'variants':
        return (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <p className="text-sm text-graphite-light dark:text-white/60">Каждый вариант будет отдельной единицей на складе.</p>
              {!isReadOnly && (
                <button type="button" onClick={addVariant} className="inline-flex items-center gap-2 text-sm text-graphite hover:text-black dark:text-white/80 dark:hover:text-white">
                  <ListPlus className="h-4 w-4" /> Добавить вариант
                </button>
              )}
            </div>
            
            <div className="space-y-4">
              {draft.variants.map((variant, index) => (
                <div key={variant.id} className="relative rounded-2xl border border-border-lighter bg-white/72 p-5 dark:border-white/16 dark:bg-black/24">
                  {!isReadOnly && draft.variants.length > 1 && (
                    <button type="button" onClick={() => removeVariant(variant.id)} className="absolute right-4 top-4 text-ash hover:text-red-500">
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                  <h4 className="text-sm font-semibold mb-4 text-graphite dark:text-white">Вариант {index + 1}</h4>
                  
                  {variant.optionValues && Object.keys(variant.optionValues).length > 0 && (
                    <div className="flex flex-wrap gap-4 mb-4">
                      {Object.entries(variant.optionValues).map(([k, v]) => (
                        <div key={k} className="flex-1 min-w-[120px]">
                          <Field 
                            label={k} 
                            value={v} 
                            disabled={isReadOnly}
                            onChange={(newVal) => {
                              const newVariants = [...draft.variants];
                              newVariants[index].optionValues[k] = newVal;
                              setDraft({ ...draft, variants: newVariants });
                            }} 
                            placeholder={`Значение для ${k}`} 
                          />
                        </div>
                      ))}
                    </div>
                  )}

                  <div className="grid gap-4 md:grid-cols-2">
                    <Field 
                      disabled={isReadOnly} 
                      label="Артикул (SKU)" 
                      required 
                      value={variant.sku} 
                      onChange={(val) => handleVariantChange(variant.id, 'sku', val)} 
                      placeholder="ZMK-12345" 
                      helpText="Идентификатор варианта"
                      error={variant.sku === duplicateSku ? "SKU уже используется в другом товаре" : undefined}
                    />
                    <Field disabled={isReadOnly} label="Штрихкод (Barcode)" value={variant.barcode} onChange={(val) => handleVariantChange(variant.id, 'barcode', val)} placeholder="Например, 460000000000" helpText="Для приемки на складе" />
                  </div>
                </div>
              ))}
            </div>
          </div>
        );

      case 'media':
        return (
          <div>
            {!isReadOnly && (
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
            )}
            <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {draft.photos.map((photo) => (
                <div key={photo.id} className="flex items-center justify-between gap-3 rounded-2xl border border-border-lighter bg-white/72 p-4 dark:border-white/16 dark:bg-black/24">
                  <div className="flex items-center gap-3 overflow-hidden">
                    {photo.url.startsWith('blob:') || photo.url.startsWith('http') || photo.url.startsWith('/') ? (
                       <img src={photo.url} alt="thumbnail" className="h-10 w-10 object-cover rounded-lg" />
                    ) : null}
                    <span className="text-sm text-graphite dark:text-white truncate max-w-[120px]" title={photo.id}>{photo.id}</span>
                  </div>
                  {!isReadOnly && (
                    <button type="button" onClick={() => removePhoto(photo.id, photo.isNew)} className="text-ash hover:text-red-500 dark:hover:text-red-300 shrink-0">
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                </div>
              ))}
              {draft.photos.length === 0 && <p className="text-sm text-graphite-light dark:text-white/68">Добавьте минимум 3 фото: товар, модель, деталь.</p>}
            </div>
          </div>
        );

      case 'attributes':
        return (
          <div className="grid gap-4 md:grid-cols-2">
            <Field disabled={isReadOnly} label="Цвет" required value={draft.color} onChange={(value) => updateDraft('color', value)} placeholder="Например, Синий" />
            <Field disabled={isReadOnly} label="Материал" required value={draft.material} onChange={(value) => updateDraft('material', value)} placeholder="Например, 100% Хлопок" />
            <div className="md:col-span-2">
              <Field disabled={isReadOnly} label="Инструкция по уходу" value={draft.careInstructions} onChange={(value) => updateDraft('careInstructions', value)} textarea placeholder="Как стирать и ухаживать за товаром" />
            </div>
          </div>
        );

      case 'price':
        return (
          <div className="grid gap-4 md:grid-cols-2">
            <div className="md:col-span-2">
              <Field disabled={isReadOnly} label="Цена продажи (₽)" required value={draft.price} onChange={(value) => updateDraft('price', value)} placeholder="14900" helpText="Итоговая цена, которую увидит покупатель" />
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
        {savedStatus === 'pending_moderation' && (
          <div className="mb-8 rounded-2xl bg-sky-50 p-6 dark:bg-sky-400/10 border border-sky-100 dark:border-sky-400/20">
            <h3 className="text-lg font-semibold text-sky-800 dark:text-sky-300">Товар отправлен на модерацию ZAMK</h3>
            <p className="mt-2 text-sky-700 dark:text-sky-400">Мы проверим карточку в течение 24 часов.</p>
          </div>
        )}
        <Link to="/seller-products" className="inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white">
          <ArrowLeft className="h-4 w-4" />
          Мои товары
        </Link>

        <section className="mt-6 glass-panel-strong p-7 md:p-10">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="studio-label">Карточка товара</p>
              <h1 className="mt-3 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-5xl">Редактировать товар</h1>
              <p className="studio-subtitle mt-4 max-w-3xl">
                Пройдите все шаги для создания товара. Товар появится в каталоге после модерации и приемки на склад ZAMK.
              </p>
              {isReadOnly && (
                <div className="mt-4 rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-900/30 dark:bg-blue-900/20 dark:text-blue-200 w-full md:max-w-2xl">
                  <span className="mb-1 block font-semibold">Режим просмотра</span>
                  Товар находится в статусе «{productStatus}». В этом статусе редактирование заблокировано. Изменение опубликованного товара требует отдельного процесса повторной модерации.
                </div>
              )}
              {productStatus === 'rejected' && rejectionReason && (
                <div className="mt-4 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200 w-full md:max-w-2xl">
                  <span className="mb-1 block font-semibold">Причина отклонения:</span>
                  {rejectionReason}
                </div>
              )}
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
              ) : isReadOnly ? (
                <div className="rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 text-center">
                  Редактирование заблокировано в статусе «{productStatus}».
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
                    onClick={() => saveProduct('pending_moderation')}
                    disabled={isSaving || quality < 80}
                    className={cn(
                      "inline-flex h-12 w-full items-center justify-center gap-2 rounded-full px-6 text-sm font-semibold transition-all shadow-sm focus:outline-none focus:ring-2 focus:ring-graphite focus:ring-offset-2",
                      quality >= 80 
                        ? "bg-graphite text-white hover:bg-black dark:bg-white dark:text-black dark:hover:bg-white/90" 
                        : "bg-ash/50 text-white cursor-not-allowed"
                    )}
                  >
                    <Rocket className="h-4 w-4" />
                    {isSaving ? 'Сохранение...' : 'Отправить на модерацию'}
                  </button>
                </>
              )}
              
              {savedStatus && savedStatus !== 'pending_moderation' && (
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

        {moderationLogs && moderationLogs.length > 0 && (
          <section className="mt-6 glass-panel-strong p-7 md:p-10 mb-10">
            <h2 className="text-2xl font-serif text-graphite dark:text-white">История модерации</h2>
            <div className="mt-6 space-y-4">
              {moderationLogs.map((log: any) => (
                <div key={log.id} className="rounded-2xl border border-border-lighter bg-white/72 p-5 dark:border-white/16 dark:bg-black/24">
                  <div className="flex justify-between items-center mb-2">
                    <span className="text-sm font-semibold text-graphite dark:text-white">Переход в статус: {log.toStatus}</span>
                    <span className="text-xs text-ash dark:text-white/62">{new Date(log.createdAt).toLocaleString('ru-RU')}</span>
                  </div>
                  {log.comment && (
                    <div className="mt-3 p-3 bg-red-50 text-red-800 text-sm rounded-lg border border-red-100 dark:bg-red-900/20 dark:text-red-200 dark:border-red-900/30">
                      <strong>Комментарий:</strong> {log.comment}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
