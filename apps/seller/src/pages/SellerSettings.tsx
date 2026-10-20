import { useEffect, useRef, useState } from 'react';
import { Store, Upload, CheckCircle, AlertCircle, Clock, Ban, Archive, ExternalLink, Image, Trash2 } from 'lucide-react';
import { getSellerMe, updateSellerMe, uploadSellerLogo, uploadSellerCover } from '@zamk/api-client/src/seller';
import type { SellerMe, UpdateSellerProfileRequest } from '@zamk/api-client/src/types';
import { SellerPageFrame, SellerPageHeader, SellerGrid } from '../components/SellerPageFrame';

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
  const logoInputRef = useRef<HTMLInputElement>(null);

  const [coverPreview, setCoverPreview] = useState<string | null>(null);
  const [isUploadingCover, setIsUploadingCover] = useState(false);
  const [coverError, setCoverError] = useState('');
  const coverInputRef = useRef<HTMLInputElement>(null);

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
            history: data.seller.history ?? '',
            contactEmail: data.seller.contactEmail ?? '',
            contactPhone: data.seller.contactPhone ?? '',
            slug: data.seller.slug ?? '',
            logoUrl: data.seller.logoUrl ?? '',
            coverUrl: data.seller.coverUrl ?? '',
          });
          if (data.seller.logoUrl) {
            setLogoPreview(data.seller.logoUrl);
          }
          if (data.seller.coverUrl) {
            setCoverPreview(data.seller.coverUrl);
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
      const payload: UpdateSellerProfileRequest = {
        ...form,
        logoUrl: logoPreview || '',
        coverUrl: coverPreview || '',
      };
      const updated = await updateSellerMe(payload);
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
      setForm(prev => ({ ...prev, logoUrl: result.logoUrl }));
      setSellerData(prev => prev ? { ...prev, seller: { ...prev.seller, logoUrl: result.logoUrl } } : prev);
    } catch {
      setLogoPreview(sellerData?.seller.logoUrl ?? null);
      setLogoError('Не удалось загрузить логотип. Допустимые форматы: JPG, PNG, WebP.');
    } finally {
      setIsUploadingLogo(false);
      if (logoInputRef.current) logoInputRef.current.value = '';
    }
  };

  const handleRemoveLogo = () => {
    setLogoPreview(null);
    setForm(prev => ({ ...prev, logoUrl: '' }));
  };

  const handleCoverSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setCoverError('');
    setIsUploadingCover(true);

    const previewUrl = URL.createObjectURL(file);
    setCoverPreview(previewUrl);

    try {
      const result = await uploadSellerCover(file);
      setCoverPreview(result.coverUrl);
      setForm(prev => ({ ...prev, coverUrl: result.coverUrl }));
      setSellerData(prev => prev ? { ...prev, seller: { ...prev.seller, coverUrl: result.coverUrl } } : prev);
    } catch {
      setCoverPreview(sellerData?.seller.coverUrl ?? null);
      setCoverError('Не удалось загрузить обложку. Допустимые форматы: JPG, PNG, WebP.');
    } finally {
      setIsUploadingCover(false);
      if (coverInputRef.current) coverInputRef.current.value = '';
    }
  };

  const handleRemoveCover = () => {
    setCoverPreview(null);
    setForm(prev => ({ ...prev, coverUrl: '' }));
  };

  const generateSlugFromName = () => {
    const name = form.brandName ?? '';
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    handleChange('slug', slug);
  };

  const storefrontSlug = form.slug || sellerData?.seller.slug || sellerData?.seller.id || '';
  const shopBaseUrl = (typeof window !== 'undefined' && window.location.port === '3005')
    ? 'http://localhost:3005'
    : 'http://localhost:3000';
  const storefrontUrl = `${shopBaseUrl}/brand/${storefrontSlug}`;

  if (isLoading) {
    return (
      <SellerPageFrame variant="form">
        <div className="flex h-64 items-center justify-center">
          <p className="text-gray-500">Загрузка профиля магазина...</p>
        </div>
      </SellerPageFrame>
    );
  }

  if (loadError) {
    return (
      <SellerPageFrame variant="form">
        <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{loadError}</div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="form">
      <SellerPageHeader
        eyebrow="Настройки"
        title="Профиль магазина"
        description="Управление публичной витриной бренда в Shop ZAMK и служебными контактами."
        action={sellerData ? <StatusBadge status={sellerData.seller.status} /> : undefined}
      />

      <SellerGrid>
        <div className="lg:col-span-8 xl:col-span-9 space-y-8">
          {/* Статус-подсказка */}
          {sellerData?.seller.status === 'pending' && (
            <div className="rounded-xl border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800">
              <strong>Магазин на проверке.</strong> Заполните витрину — после одобрения администратором ваш бренд появится в публичном каталоге.
            </div>
          )}
          {sellerData?.seller.status === 'blocked' && (
            <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
              <strong>Магазин заблокирован.</strong> Обратитесь к администратору для выяснения причин.
            </div>
          )}

          {/* Блок «Публичная витрина» */}
          <div className="rounded-2xl border border-gray-200 bg-white p-6 space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-gray-100 pb-4">
              <div>
                <h2 className="text-lg font-semibold text-gray-900">Публичная витрина</h2>
                <p className="text-xs text-gray-500 mt-0.5">
                  Внешний вид страницы бренда и карточек товаров для покупателей в Shop ZAMK.
                </p>
              </div>
              {storefrontSlug && (
                <a
                  href={storefrontUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-100 hover:text-gray-900 transition-colors shrink-0"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  Открыть витрину в Shop
                </a>
              )}
            </div>

            {/* Медиа витрины: Логотип и Обложка */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Логотип */}
              <div className="rounded-xl border border-gray-100 bg-gray-50/50 p-4">
                <label className="block text-xs font-semibold uppercase tracking-wider text-gray-700 mb-3">
                  Логотип бренда
                </label>
                <div className="flex items-center gap-4">
                  <div className="flex h-20 w-20 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs">
                    {logoPreview ? (
                      <img src={logoPreview} alt="Логотип" className="h-full w-full object-contain" />
                    ) : (
                      <Store className="h-8 w-8 text-gray-300" />
                    )}
                  </div>
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        disabled={isUploadingLogo}
                        onClick={() => logoInputRef.current?.click()}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                      >
                        <Upload className="h-3.5 w-3.5" />
                        {isUploadingLogo ? 'Загрузка...' : 'Загрузить логотип'}
                      </button>
                      {logoPreview && (
                        <button
                          type="button"
                          onClick={handleRemoveLogo}
                          className="p-1.5 text-gray-400 hover:text-red-600 rounded-lg transition-colors"
                          title="Удалить логотип"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </div>
                    <p className="text-[11px] text-gray-400">JPG, PNG или WebP, до 5 МБ</p>
                  </div>
                </div>
                {logoError && <p className="mt-2 text-xs text-red-600">{logoError}</p>}
                <input
                  ref={logoInputRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  className="hidden"
                  onChange={handleLogoSelect}
                />
              </div>

              {/* Обложка */}
              <div className="rounded-xl border border-gray-100 bg-gray-50/50 p-4">
                <label className="block text-xs font-semibold uppercase tracking-wider text-gray-700 mb-3">
                  Обложка витрины (баннер)
                </label>
                <div className="flex flex-col gap-3">
                  <div className="relative h-20 w-full overflow-hidden rounded-xl border border-gray-200 bg-white flex items-center justify-center shadow-xs">
                    {coverPreview ? (
                      <img src={coverPreview} alt="Обложка" className="h-full w-full object-cover" />
                    ) : (
                      <div className="flex items-center gap-2 text-xs text-gray-400">
                        <Image className="h-5 w-5" />
                        <span>Баннер не установлен</span>
                      </div>
                    )}
                  </div>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        disabled={isUploadingCover}
                        onClick={() => coverInputRef.current?.click()}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                      >
                        <Upload className="h-3.5 w-3.5" />
                        {isUploadingCover ? 'Загрузка...' : 'Загрузить обложку'}
                      </button>
                      {coverPreview && (
                        <button
                          type="button"
                          onClick={handleRemoveCover}
                          className="p-1.5 text-gray-400 hover:text-red-600 rounded-lg transition-colors"
                          title="Удалить обложку"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </div>
                    <p className="text-[11px] text-gray-400">Рекомендуется 1400×400 px</p>
                  </div>
                </div>
                {coverError && <p className="mt-2 text-xs text-red-600">{coverError}</p>}
                <input
                  ref={coverInputRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  className="hidden"
                  onChange={handleCoverSelect}
                />
              </div>
            </div>

            {/* Текстовые поля витрины */}
            <div className="space-y-4 pt-2">
              <div>
                <label htmlFor="seller-brand-name" className="block text-sm font-medium text-gray-700 mb-1">
                  Публичное название бренда <span className="text-red-500">*</span>
                </label>
                <input
                  id="seller-brand-name"
                  type="text"
                  value={form.brandName ?? ''}
                  onChange={e => handleChange('brandName', e.target.value)}
                  placeholder="Например: Atelier Monochrome"
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500"
                />
              </div>

              <div>
                <label htmlFor="seller-slug" className="block text-sm font-medium text-gray-700 mb-1">
                  Адрес витрины (slug)
                </label>
                <div className="flex items-center rounded-lg border border-gray-300 overflow-hidden focus-within:border-gray-500 focus-within:ring-1 focus-within:ring-gray-500">
                  <span className="select-none bg-gray-50 px-3 py-2 text-sm text-gray-400 border-r border-gray-300">
                    zamk.ru/brand/
                  </span>
                  <input
                    id="seller-slug"
                    type="text"
                    value={form.slug ?? ''}
                    onChange={e => handleChange('slug', e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                    placeholder="my-brand"
                    className="flex-1 px-3 py-2 text-sm focus:outline-none"
                  />
                </div>
                <div className="mt-1 flex items-center justify-between">
                  <p className="text-xs text-gray-400">Только латиница, цифры и дефис.</p>
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
                <label htmlFor="seller-desc" className="block text-sm font-medium text-gray-700 mb-1">
                  Краткое описание витрины
                </label>
                <textarea
                  id="seller-desc"
                  value={form.description ?? ''}
                  onChange={e => handleChange('description', e.target.value)}
                  placeholder="1–2 предложения о бренде для карточек каталога и шапки витрины..."
                  rows={2}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500 resize-none"
                />
              </div>

              <div>
                <label htmlFor="seller-history" className="block text-sm font-medium text-gray-700 mb-1">
                  История и философия бренда
                </label>
                <textarea
                  id="seller-history"
                  value={form.history ?? ''}
                  onChange={e => handleChange('history', e.target.value)}
                  placeholder="Расскажите историю создания марки, о концепции коллекций, используемых материалах и эстетике..."
                  rows={4}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-500 focus:outline-none focus:ring-1 focus:ring-gray-500 resize-none"
                />
                <p className="mt-1 text-xs text-gray-400">
                  Отображается на публичной странице бренда в отдельном блоке истории.
                </p>
              </div>
            </div>
          </div>

          {/* Блок «Контакты магазина» */}
          <div className="rounded-2xl border border-gray-200 bg-white p-6 space-y-4">
            <div>
              <h2 className="text-base font-semibold text-gray-900">Контакты магазина</h2>
              <p className="text-xs text-gray-500 mt-0.5">
                Служебные контакты для связи с платформой ZAMK и уведомлений.
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2 pt-1">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Контактная почта <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  value={form.contactEmail ?? ''}
                  onChange={e => handleChange('contactEmail', e.target.value)}
                  placeholder="brand@example.com"
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
          </div>

          {/* Уведомления */}
          {saveSuccess && (
            <div data-testid="seller-save-success" className="flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">
              <CheckCircle className="h-4 w-4 flex-shrink-0" />
              Профиль и публичная витрина сохранены
            </div>
          )}
          {saveError && (
            <div data-testid="seller-save-error" className="flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              {saveError}
            </div>
          )}

          <div className="flex items-center justify-between pt-2">
            {storefrontSlug ? (
              <a
                href={storefrontUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-gray-500 hover:text-gray-900 inline-flex items-center gap-1 transition-colors"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Предпросмотр витрины в Shop
              </a>
            ) : <div />}

            <button
              type="button"
              disabled={isSaving}
              onClick={handleSave}
              className="rounded-lg bg-gray-900 px-6 py-2.5 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-60 transition-colors shadow-xs"
            >
              {isSaving ? 'Сохраняется...' : 'Сохранить профиль'}
            </button>
          </div>
        </div>
      </SellerGrid>
    </SellerPageFrame>
  );
}
