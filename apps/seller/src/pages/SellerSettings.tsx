import { useEffect, useRef, useState } from 'react';
import { Store, Upload, CheckCircle, AlertCircle, Clock, Ban, Archive } from 'lucide-react';
import { getSellerMe, updateSellerMe, uploadSellerLogo } from '@zamk/api-client/src/seller';
import type { SellerMe, UpdateSellerProfileRequest } from '@zamk/api-client/src/types';

const STATUS_LABELS: Record<string, { label: string; color: string; icon: typeof CheckCircle }> = {
  active:   { label: 'Активен',     color: 'bg-green-100 text-green-800',  icon: CheckCircle },
  pending:  { label: 'На проверке', color: 'bg-yellow-100 text-yellow-800', icon: Clock },
  blocked:  { label: 'Заблокирован', color: 'bg-red-100 text-red-800',     icon: Ban },
  archived: { label: 'Архивирован', color: 'bg-gray-100 text-gray-700',    icon: Archive },
};

function StatusBadge({ status }: { status: string }) {
  const meta = STATUS_LABELS[status] ?? { label: status, color: 'bg-gray-100 text-gray-700', icon: AlertCircle };
  const Icon = meta.icon;
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium ${meta.color}`}>
      <Icon className="h-3.5 w-3.5" />
      {meta.label}
    </span>
  );
}

export function SellerSettings() {
  const [sellerData, setSellerData] = useState<SellerMe | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState('');

  const [form, setForm] = useState<UpdateSellerProfileRequest>({});
  const [isSaving, setIsSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState('');

  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  const [isUploadingLogo, setIsUploadingLogo] = useState(false);
  const [logoError, setLogoError] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setIsLoading(true);
      setLoadError('');
      try {
        const data = await getSellerMe();
        if (!cancelled) {
          setSellerData(data);
          setForm({
            brandName: data.seller.brandName ?? '',
            description: data.seller.description ?? '',
            contactEmail: data.seller.contactEmail ?? '',
            contactPhone: data.seller.contactPhone ?? '',
            slug: data.seller.slug ?? '',
          });
          if (data.seller.logoUrl) {
            setLogoPreview(data.seller.logoUrl);
          }
        }
      } catch (err: any) {
        if (!cancelled) {
          if (err?.status === 401 || err?.code === 'unauthorized') {
            setLoadError('Сессия истекла. Обновите страницу и войдите снова.');
          } else {
            setLoadError('Не удалось загрузить профиль магазина. Проверьте, запущен ли backend.');
          }
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  const handleChange = (field: keyof UpdateSellerProfileRequest, value: string) => {
    setForm(prev => ({ ...prev, [field]: value }));
    setSaveSuccess(false);
    setSaveError('');
  };

  const handleSave = async () => {
    setSaveSuccess(false);
    setSaveError('');
    setIsSaving(true);
    try {
      const updated = await updateSellerMe(form);
      setSellerData(updated);
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 4000);
    } catch (err: any) {
      if (err?.status === 409) {
        setSaveError('Этот адрес магазина (slug) уже занят. Выберите другой.');
      } else {
        setSaveError('Не удалось сохранить профиль. Проверьте данные и попробуйте снова.');
      }
    } finally {
      setIsSaving(false);
    }
  };

  const handleLogoSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setLogoError('');
    setIsUploadingLogo(true);

    const previewUrl = URL.createObjectURL(file);
    setLogoPreview(previewUrl);

    try {
      const result = await uploadSellerLogo(file);
      setLogoPreview(result.logoUrl);
      setSellerData(prev => prev ? { ...prev, seller: { ...prev.seller, logoUrl: result.logoUrl } } : prev);
    } catch {
      setLogoPreview(sellerData?.seller.logoUrl ?? null);
      setLogoError('Не удалось загрузить логотип. Допустимые форматы: JPG, PNG, WebP.');
    } finally {
      setIsUploadingLogo(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const isProfileEmpty = sellerData && !sellerData.seller.description && !sellerData.seller.contactPhone;

  const generateSlugFromName = () => {
    const name = form.brandName ?? '';
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    handleChange('slug', slug);
  };

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-gray-500">Загрузка профиля магазина...</p>
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="p-8">
        <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{loadError}</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="mx-auto max-w-2xl space-y-8">

        {/* Заголовок */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium uppercase tracking-wide text-gray-500">Настройки</p>
            <h1 className="mt-1 text-3xl font-bold text-gray-900">Профиль магазина</h1>
          </div>
          {sellerData && <StatusBadge status={sellerData.seller.status} />}
        </div>

        {/* Статус-подсказка для не-активных */}
        {sellerData?.seller.status === 'pending' && (
          <div className="rounded-xl border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800">
            <strong>Магазин на проверке.</strong> Заполните профиль — это поможет администратору быстрее вас одобрить.
          </div>
        )}
        {sellerData?.seller.status === 'blocked' && (
          <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
            <strong>Магазин заблокирован.</strong> Обратитесь к администратору для выяснения причин.
          </div>
        )}

        {/* Пустой профиль */}
        {isProfileEmpty && (
          <div className="rounded-xl border border-dashed border-gray-300 bg-white p-5 text-sm text-gray-600">
            <Store className="mb-2 h-5 w-5 text-gray-400" />
            Заполните профиль магазина, чтобы покупатели понимали, кто продаёт товар.
          </div>
        )}

        {/* Логотип */}
        <div className="rounded-2xl border border-gray-200 bg-white p-6">
          <h2 className="text-base font-semibold text-gray-900">Логотип магазина</h2>
          <div className="mt-4 flex items-center gap-5">
            <div className="flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border border-gray-200 bg-gray-50">
              {logoPreview ? (
                <img src={logoPreview} alt="Логотип" className="h-full w-full object-cover" />
              ) : (
                <Store className="h-8 w-8 text-gray-300" />
              )}
            </div>
            <div className="flex flex-col gap-2">
              <button
                type="button"
                disabled={isUploadingLogo}
                onClick={() => fileInputRef.current?.click()}
                className="inline-flex items-center gap-2 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                <Upload className="h-4 w-4" />
                {isUploadingLogo ? 'Загружается...' : 'Загрузить логотип'}
              </button>
              <p className="text-xs text-gray-400">JPG, PNG или WebP, до 5 МБ</p>
            </div>
          </div>
          {logoError && <p className="mt-2 text-sm text-red-600">{logoError}</p>}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="hidden"
            onChange={handleLogoSelect}
          />
        </div>

        {/* Форма */}
        <div className="rounded-2xl border border-gray-200 bg-white p-6 space-y-5">
          <h2 className="text-base font-semibold text-gray-900">Данные магазина</h2>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Название магазина <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              value={form.brandName ?? ''}
              onChange={e => handleChange('brandName', e.target.value)}
              placeholder="Например: ZAMK Style"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Адрес магазина (slug)
            </label>
            <div className="flex items-center rounded-lg border border-gray-300 overflow-hidden focus-within:border-gray-500 focus-within:ring-1 focus-within:ring-gray-500">
              <span className="select-none bg-gray-50 px-3 py-2 text-sm text-gray-400 border-r border-gray-300">
                zamk.ru/
              </span>
              <input
                type="text"
                value={form.slug ?? ''}
                onChange={e => handleChange('slug', e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                placeholder="my-shop"
                className="flex-1 px-3 py-2 text-sm focus:outline-none"
              />
            </div>
            <div className="mt-1 flex items-center justify-between">
              <p className="text-xs text-gray-400">Только латиница, цифры и дефис. Используется в публичном адресе магазина.</p>
              <button
                type="button"
                onClick={generateSlugFromName}
                className="text-xs text-gray-500 underline hover:text-gray-700"
              >
                Сгенерировать из названия
              </button>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Описание магазина
            </label>
            <textarea
              value={form.description ?? ''}
              onChange={e => handleChange('description', e.target.value)}
              placeholder="Расскажите покупателям о вашем магазине..."
              rows={4}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500 resize-none"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Контактная почта <span className="text-red-500">*</span>
              </label>
              <input
                type="email"
                value={form.contactEmail ?? ''}
                onChange={e => handleChange('contactEmail', e.target.value)}
                placeholder="shop@example.com"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Контактный телефон
              </label>
              <input
                type="tel"
                value={form.contactPhone ?? ''}
                onChange={e => handleChange('contactPhone', e.target.value)}
                placeholder="+7 (999) 123-45-67"
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500"
              />
            </div>
          </div>

          {/* Уведомления */}
          {saveSuccess && (
            <div className="flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">
              <CheckCircle className="h-4 w-4 flex-shrink-0" />
              Профиль магазина сохранён
            </div>
          )}
          {saveError && (
            <div className="flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              {saveError}
            </div>
          )}

          <div className="flex justify-end pt-2">
            <button
              type="button"
              disabled={isSaving}
              onClick={handleSave}
              className="rounded-lg bg-gray-900 px-6 py-2.5 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-60"
            >
              {isSaving ? 'Сохраняется...' : 'Сохранить профиль'}
            </button>
          </div>
        </div>

        {/* Info block */}
        <div className="rounded-xl border border-gray-100 bg-white p-4 text-xs text-gray-400">
          ID магазина: {sellerData?.seller.id ?? '—'} · Роль: {sellerData?.sellerUser.role ?? '—'}
        </div>

      </div>
    </div>
  );
}
