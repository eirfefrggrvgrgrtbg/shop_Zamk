import { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
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
} from 'lucide-react';
import { type SellerProductSize, type SellerProductStatus } from '../lib/seller-products';
import { getSellerProduct, updateSellerProduct, uploadSellerProductImage } from '@zamk/api-client/src/seller';
import { request } from '@zamk/api-client/src/client';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const formatCurrency = (value: number) => currencyFormatter.format(value);

type DraftProduct = {
  title: string;
  category: string;
  brand: string;
  sku: string;
  description: string;
  price: string;
  oldPrice: string;
  cost: string;
  adsBid: string;
  material: string;
  color: string;
  season: string;
  photos: string[];
  sizes: SellerProductSize[];
};

const initialDraft: DraftProduct = {
  title: '',
  category: 'Одежда',
  brand: 'ZAMK Selected',
  sku: `ZMK-${Date.now().toString().slice(-5)}`,
  description: '',
  price: '',
  oldPrice: '',
  cost: '',
  adsBid: '900',
  material: '',
  color: '',
  season: 'Всесезон',
  photos: [],
  sizes: [
    { size: 'XS', stock: 0 },
    { size: 'S', stock: 0 },
    { size: 'M', stock: 0 },
    { size: 'L', stock: 0 },
  ],
};

const steps = [
  { id: 'base', label: 'Основное', icon: Shirt },
  { id: 'media', label: 'Фото', icon: ImagePlus },
  { id: 'price', label: 'Цена', icon: Wallet },
  { id: 'stock', label: 'Остатки', icon: PackageCheck },
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
    draft.category.trim().length > 0,
    draft.sku.trim().length > 0,
    draft.description.trim().length >= 120,
    draft.photos.length >= 3,
    asNumber(draft.price) > 0,
    asNumber(draft.cost) > 0 && asNumber(draft.price) > asNumber(draft.cost),
    draft.material.trim().length > 0,
    draft.color.trim().length > 0,
    draft.sizes.some((item) => item.stock > 0),
  ];

  return Math.round((checks.filter(Boolean).length / checks.length) * 100);
}

// removed local sellerProductFromDraft

function Field({
  label,
  value,
  onChange,
  placeholder,
  textarea,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  textarea?: boolean;
}) {
  const className = 'seller-setting-input mt-2 w-full rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none transition-all focus:border-graphite/30 focus:bg-white dark:border-white/16 dark:bg-black/24 dark:text-white dark:focus:border-white/32 dark:focus:bg-black/32';

  return (
    <label className="block">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">{label}</span>
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

export function SellerProductEdit() {
  const { id } = useParams();
  const [draft, setDraft] = useState<DraftProduct>(initialDraft);
  const [activeStep, setActiveStep] = useState<StepId>('base');
  const [photoFiles, setPhotoFiles] = useState<File[]>([]);
  const [savedStatus, setSavedStatus] = useState<SellerProductStatus | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  
  useEffect(() => {
    async function load() {
      if (!id) return;
      try {
        const product = await getSellerProduct(id);
        const sizes = product.variants?.map((v: any) => ({
          size: v.size || 'Единый',
          stock: v.inStock ? 10 : 0
        })) || [];
        if (sizes.length === 0) sizes.push({ size: 'Единый', stock: 10 });
        
        setDraft({
          title: product.title,
          category: product.categoryId || 'Одежда',
          brand: product.brandId || 'ZAMK',
          sku: product.slug || product.id,
          description: product.description || '',
          price: (product.priceCents / 100).toString(),
          oldPrice: product.oldPriceCents ? (product.oldPriceCents / 100).toString() : '',
          cost: '0',
          adsBid: '900',
          material: product.material || '',
          color: product.color || '',
          season: 'Всесезон',
          photos: product.images?.map((img: any) => img.imageUrl) || [],
          sizes
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
  const cost = asNumber(draft.cost);
  const adsBid = asNumber(draft.adsBid);
  const netProfit = Math.max(0, price - cost - Math.round(price * 0.1) - adsBid);

  if (isLoading) {
    return <div className="min-h-screen pt-24 pb-24 md:pt-28 md:pb-20 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

  const updateDraft = (key: keyof DraftProduct, value: DraftProduct[keyof DraftProduct]) => {
    setDraft((current) => ({ ...current, [key]: value }));
    setSavedStatus(null);
  };

  const updateSize = (index: number, stock: number) => {
    setDraft((current) => ({
      ...current,
      sizes: current.sizes.map((item, itemIndex) => (itemIndex === index ? { ...item, stock: Math.max(0, stock) } : item)),
    }));
    setSavedStatus(null);
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
      alert('Некоторые файлы пропущены. Разрешены только JPG, PNG, WEBP.');
    }

    setPhotoFiles(curr => [...curr, ...validFiles]);
    updateDraft('photos', [...draft.photos, ...validFiles.map(f => f.name)]);
  };

  const saveProduct = async (status: SellerProductStatus) => {
    setIsSaving(true);
    setError('');
    try {
      const priceCents = asNumber(draft.price) * 100;
      const oldPriceCents = asNumber(draft.oldPrice) ? asNumber(draft.oldPrice) * 100 : undefined;
      
      const variants = draft.sizes
        .filter(s => s.size.trim() !== '')
        .map(s => ({
          size: s.size,
          sku: `${draft.sku}-${s.size}`,
          color: draft.color,
          inStock: s.stock > 0,
          isActive: true
        }));

      const payload = {
        title: draft.title || 'Новый товар',
        slug: draft.sku || `slug-${Date.now()}`,
        description: draft.description,
        priceCents,
        oldPriceCents,
        currency: 'RUB',
        material: draft.material,
        color: draft.color,
        variants
      };

      await updateSellerProduct(id as string, payload);

      for (const file of photoFiles) {
        try {
          await uploadSellerProductImage(id as string, file);
        } catch (imgErr) {
          console.error('Image upload failed', imgErr);
        }
      }

      if (status === 'moderation') {
        await request('POST', `/seller/products/${id}/submit-moderation`);
      }

      setSavedStatus(status);
    } catch (err: any) {
      setError(err.message || 'Ошибка сохранения');
    } finally {
      setIsSaving(false);
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
              <h1 className="mt-3 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-6xl">Редактировать товар</h1>
              <p className="studio-subtitle mt-4 max-w-3xl">
                Заполните карточку по шагам. Черновик можно сохранить сразу, а на модерацию лучше отправлять после проверки качества.
              </p>
            </div>
            <div className="rounded-[2rem] border border-border-lighter bg-white/72 p-5 dark:border-white/16 dark:bg-black/26">
              <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Готовность карточки</p>
              <p className="mt-2 text-4xl font-semibold text-graphite dark:text-white">{quality}%</p>
              <div className="mt-3 h-2 overflow-hidden rounded-full bg-ice dark:bg-white/10">
                <div className={cn('h-full rounded-full', quality >= 80 ? 'bg-emerald-500 dark:bg-emerald-300' : quality >= 60 ? 'bg-amber-500 dark:bg-amber-300' : 'bg-red-500 dark:bg-red-300')} style={{ width: `${quality}%` }} />
              </div>
            </div>
          </div>
        </section>

        <section className="mt-6 grid gap-3 md:grid-cols-5">
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
                    : 'border-border-lighter bg-white/70 text-graphite-light hover:bg-white dark:border-white/16 dark:bg-black/24 dark:text-white/68 dark:hover:bg-white/8'
                )}
              >
                <Icon className="h-5 w-5 shrink-0" />
                <span className="text-sm font-semibold">{step.label}</span>
              </button>
            );
          })}
        </section>

        <div className="mt-6 grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
          <section className="glass-panel-strong p-6 md:p-8">
            {activeStep === 'base' && (
              <div className="grid gap-4 md:grid-cols-2">
                <Field label="Название товара" value={draft.title} onChange={(value) => updateDraft('title', value)} placeholder="Например, Жакет мягкой линии" />
                <Field label="Артикул" value={draft.sku} onChange={(value) => updateDraft('sku', value)} />
                <Field label="Категория" value={draft.category} onChange={(value) => updateDraft('category', value)} />
                <Field label="Бренд" value={draft.brand} onChange={(value) => updateDraft('brand', value)} />
                <div className="md:col-span-2">
                  <Field label="Описание" value={draft.description} onChange={(value) => updateDraft('description', value)} textarea placeholder="Опишите посадку, материал, сценарии носки и отличие товара." />
                </div>
              </div>
            )}

            {activeStep === 'media' && (
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
                      <span className="text-sm text-graphite dark:text-white">{photo}</span>
                      <button type="button" onClick={() => removePhoto(photo)} className="text-ash hover:text-red-500 dark:hover:text-red-300">
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ))}
                  {draft.photos.length === 0 && <p className="text-sm text-graphite-light dark:text-white/68">Добавьте минимум 3 фото: товар, модель, деталь.</p>}
                </div>
              </div>
            )}

            {activeStep === 'price' && (
              <div className="grid gap-4 md:grid-cols-2">
                <Field label="Цена" value={draft.price} onChange={(value) => updateDraft('price', value)} placeholder="14900" />
                <Field label="Старая цена" value={draft.oldPrice} onChange={(value) => updateDraft('oldPrice', value)} placeholder="17900" />
                <Field label="Себестоимость" value={draft.cost} onChange={(value) => updateDraft('cost', value)} placeholder="7100" />
                <Field label="Стартовая рекламная ставка" value={draft.adsBid} onChange={(value) => updateDraft('adsBid', value)} placeholder="900" />
              </div>
            )}

            {activeStep === 'stock' && (
              <div>
                <div className="grid gap-4 md:grid-cols-3">
                  <Field label="Материал" value={draft.material} onChange={(value) => updateDraft('material', value)} />
                  <Field label="Цвет" value={draft.color} onChange={(value) => updateDraft('color', value)} />
                  <Field label="Сезон" value={draft.season} onChange={(value) => updateDraft('season', value)} />
                </div>
                <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  {draft.sizes.map((item, index) => (
                    <label key={item.size} className="rounded-2xl border border-border-lighter bg-white/72 p-4 dark:border-white/16 dark:bg-black/24">
                      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Размер {item.size}</span>
                      <input
                        type="number"
                        min="0"
                        value={item.stock}
                        onChange={(event) => updateSize(index, Number(event.target.value))}
                        className="seller-setting-input mt-2 h-11 w-full rounded-xl border border-border-lighter bg-white/78 px-3 text-sm text-graphite outline-none dark:border-white/16 dark:bg-black/24 dark:text-white"
                      />
                    </label>
                  ))}
                </div>
              </div>
            )}

            {activeStep === 'review' && (
              <div className="grid gap-3">
                <QualityCheck label="Название заполнено и понятно покупателю" passed={draft.title.trim().length >= 8} />
                <QualityCheck label="Описание длиннее 120 символов" passed={draft.description.trim().length >= 120} />
                <QualityCheck label="Добавлено минимум 3 фото" passed={draft.photos.length >= 3} />
                <QualityCheck label="Цена выше себестоимости" passed={price > 0 && price > cost} />
                <QualityCheck label="Есть остатки хотя бы в одном размере" passed={draft.sizes.some((item) => item.stock > 0)} />
              </div>
            )}
          </section>

          <aside className="glass-panel-strong p-6 md:p-8">
            <p className="studio-label">Превью решения</p>
            <h2 className="studio-title mt-2">{draft.title || 'Новый товар'}</h2>
            <p className="studio-subtitle mt-2">{draft.category} · {draft.brand}</p>

            <div className="mt-7 grid gap-3 sm:grid-cols-2">
              <div className="rounded-2xl border border-border-lighter bg-white/72 p-4 dark:border-white/16 dark:bg-black/24">
                <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Цена</p>
                <p className="mt-2 text-xl font-semibold text-graphite dark:text-white">{price ? formatCurrency(price) : 'Не задана'}</p>
              </div>
              <div className="rounded-2xl border border-border-lighter bg-white/72 p-4 dark:border-white/16 dark:bg-black/24">
                <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Прогноз прибыли</p>
                <p className="mt-2 text-xl font-semibold text-graphite dark:text-white">{price && cost ? formatCurrency(netProfit) : 'Заполните цену'}</p>
              </div>
            </div>

            <div className="mt-5 rounded-2xl border border-border-lighter bg-white/72 p-4 dark:border-white/16 dark:bg-black/24">
              <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">Что улучшить</p>
              <p className="mt-2 text-sm leading-relaxed text-graphite-light dark:text-white/68">
                {quality >= 80
                  ? 'Карточка выглядит готовой к модерации. Проверьте фото и остатки перед отправкой.'
                  : 'Добавьте фото, усилите описание и заполните характеристики, чтобы карточка не стартовала слабой.'}
              </p>
            </div>

            {savedStatus && (
              <div className="mt-5 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700 dark:border-emerald-300/16 dark:bg-emerald-400/10 dark:text-emerald-300">
                Товар сохранён: {savedStatus === 'draft' ? 'черновик' : 'отправлен на модерацию'}.
              </div>
            )}

            {error && (
              <div className="mt-5 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-300/16 dark:bg-red-400/10 dark:text-red-300">
                {error}
              </div>
            )}

            <div className="mt-6 flex flex-col gap-3">
              <button
                type="button"
                disabled={isSaving}
                onClick={() => saveProduct('draft')}
                className="inline-flex h-12 items-center justify-center gap-2 rounded-full border border-border-lighter bg-white/75 px-6 text-sm font-semibold text-graphite transition-colors hover:bg-white disabled:opacity-50 dark:border-white/16 dark:bg-white/8 dark:text-white dark:hover:bg-white/12"
              >
                <Save className="h-4 w-4" />
                Сохранить черновик
              </button>
              <button
                type="button"
                disabled={isSaving}
                onClick={() => saveProduct('moderation')}
                className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-graphite px-6 text-sm font-semibold text-white transition-colors hover:bg-graphite-light disabled:opacity-50 dark:bg-white dark:text-black dark:hover:bg-white/86"
              >
                <Rocket className="h-4 w-4" />
                Отправить на модерацию
              </button>
              <Link to="/seller-products" className="inline-flex h-12 items-center justify-center gap-2 rounded-full px-6 text-sm font-semibold text-graphite-light transition-colors hover:text-graphite dark:text-white/64 dark:hover:text-white">
                Перейти к списку товаров
              </Link>
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}
