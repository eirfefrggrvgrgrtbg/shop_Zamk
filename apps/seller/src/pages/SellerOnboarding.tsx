import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Store,
  FileText,
  Building2,
  ArrowRight,
  ShieldCheck,
} from 'lucide-react';
import { updateSellerMe, completeSellerOnboarding as apiCompleteSellerOnboarding } from '@zamk/api-client/src/seller';

export function SellerOnboarding() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Step 1: Profile
  const [brandName, setBrandName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [contactPhone, setContactPhone] = useState('');

  const completeOnboarding = async () => {
    setLoading(true);
    setError('');
    try {
      await updateSellerMe({
        brandName: brandName || 'Новый магазин',
        slug: slug || `shop-${Date.now()}`,
        description,
        contactEmail,
        contactPhone,
      });

      await apiCompleteSellerOnboarding();

      navigate('/dashboard', { replace: true });
    } catch (err: any) {
      setError(err.message || 'Произошла ошибка при сохранении профиля');
    } finally {
      setLoading(false);
    }
  };

  const isFormValid = brandName.length >= 2 && slug.length >= 3 && contactEmail.includes('@');

  return (
    <div className="relative z-10 min-h-screen pt-24 pb-24 md:pt-28 md:pb-20">
      <div className="container mx-auto max-w-[1000px] px-4 sm:px-6">
        <section className="glass-panel-strong p-7 md:p-10 mb-8 text-center">
          <p className="studio-label justify-center">Onboarding</p>
          <h1 className="mt-3 text-4xl font-serif leading-tight text-graphite dark:text-white md:text-5xl">
            Добро пожаловать в ZAMK
          </h1>
          <p className="studio-subtitle mt-4 mx-auto max-w-2xl">
            Для старта продаж необходимо настроить базовый профиль бренда.
            Остальные шаги (договор, реквизиты) можно будет заполнить позже.
          </p>
        </section>

        <div className="grid gap-6 md:grid-cols-[280px_1fr]">
          <aside className="flex flex-col gap-3">
            <div className="rounded-2xl border border-graphite/30 bg-white p-4 text-graphite shadow-sm dark:border-white/32 dark:bg-white/10 dark:text-white">
              <div className="flex items-center gap-3">
                <Store className="h-5 w-5" />
                <span className="text-sm font-semibold">Профиль бренда</span>
              </div>
              <p className="mt-2 text-xs opacity-70">Заполняется сейчас</p>
            </div>
            
            <div className="rounded-2xl border border-transparent bg-white/40 p-4 text-graphite-light dark:bg-white/5 dark:text-white/40">
              <div className="flex items-center gap-3">
                <Building2 className="h-5 w-5" />
                <span className="text-sm font-semibold">Реквизиты компании</span>
              </div>
              <p className="mt-2 text-xs opacity-60">Доступно после модерации профиля</p>
            </div>

            <div className="rounded-2xl border border-transparent bg-white/40 p-4 text-graphite-light dark:bg-white/5 dark:text-white/40">
              <div className="flex items-center gap-3">
                <FileText className="h-5 w-5" />
                <span className="text-sm font-semibold">Подписание оферты</span>
              </div>
              <p className="mt-2 text-xs opacity-60">Требуется для первых выплат</p>
            </div>
          </aside>

          <section className="glass-panel-strong p-6 md:p-8">
            <h2 className="text-2xl font-serif text-graphite dark:text-white mb-6">
              Информация о бренде
            </h2>

            {error && (
              <div className="mb-6 rounded-xl bg-red-50 p-4 text-sm text-red-800 dark:bg-red-900/20 dark:text-red-200">
                {error}
              </div>
            )}

            <div className="space-y-5">
              <label className="block">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">
                  Название бренда <span className="text-red-500">*</span>
                </span>
                <input 
                  type="text" 
                  value={brandName} 
                  onChange={e => setBrandName(e.target.value)}
                  placeholder="Например, ZAMK Selected"
                  className="seller-setting-input mt-2 h-12 w-full rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none focus:border-graphite/30 dark:border-white/16 dark:bg-black/24 dark:text-white"
                />
              </label>

              <label className="block">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">
                  URL магазина (slug) <span className="text-red-500">*</span>
                </span>
                <input 
                  type="text" 
                  value={slug} 
                  onChange={e => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                  placeholder="zamk-selected"
                  className="seller-setting-input mt-2 h-12 w-full rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none focus:border-graphite/30 dark:border-white/16 dark:bg-black/24 dark:text-white"
                />
                <p className="mt-1 text-xs text-graphite-light dark:text-white/40">zamk.ru/brands/{slug || '...'}</p>
              </label>

              <label className="block">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">
                  Контактный Email <span className="text-red-500">*</span>
                </span>
                <input 
                  type="email" 
                  value={contactEmail} 
                  onChange={e => setContactEmail(e.target.value)}
                  placeholder="brand@example.com"
                  className="seller-setting-input mt-2 h-12 w-full rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none focus:border-graphite/30 dark:border-white/16 dark:bg-black/24 dark:text-white"
                />
              </label>

              <label className="block">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">
                  Телефон для связи
                </span>
                <input 
                  type="text" 
                  value={contactPhone} 
                  onChange={e => setContactPhone(e.target.value)}
                  placeholder="+7 (999) 000-00-00"
                  className="seller-setting-input mt-2 h-12 w-full rounded-2xl border border-border-lighter bg-white/78 px-4 text-sm text-graphite outline-none focus:border-graphite/30 dark:border-white/16 dark:bg-black/24 dark:text-white"
                />
              </label>

              <label className="block">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-ash dark:text-white/62">
                  Краткое описание
                </span>
                <textarea 
                  value={description} 
                  onChange={e => setDescription(e.target.value)}
                  rows={3}
                  placeholder="Расскажите о вашем бренде..."
                  className="seller-setting-input mt-2 w-full resize-none rounded-2xl border border-border-lighter bg-white/78 px-4 py-3 text-sm text-graphite outline-none focus:border-graphite/30 dark:border-white/16 dark:bg-black/24 dark:text-white"
                />
              </label>

              <div className="mt-8 rounded-2xl bg-ice/50 p-5 dark:bg-white/5">
                <div className="flex items-start gap-3">
                  <ShieldCheck className="h-5 w-5 text-graphite dark:text-white shrink-0 mt-0.5" />
                  <div>
                    <h4 className="text-sm font-semibold text-graphite dark:text-white">Условия платформы</h4>
                    <p className="mt-1 text-xs text-graphite-light dark:text-white/60 leading-relaxed">
                      Нажимая кнопку ниже, вы соглашаетесь с условиями работы на платформе ZAMK. 
                      Комиссия платформы рассчитывается индивидуально после модерации ассортимента.
                    </p>
                  </div>
                </div>
              </div>

              <div className="pt-4 flex justify-end">
                <button
                  type="button"
                  onClick={completeOnboarding}
                  disabled={loading || !isFormValid}
                  className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-graphite px-8 text-sm font-semibold text-white transition-colors hover:bg-graphite-light disabled:opacity-50 dark:bg-white dark:text-black dark:hover:bg-white/86"
                >
                  {loading ? (
                    'Отправка...'
                  ) : (
                    <>
                      Завершить регистрацию
                      <ArrowRight className="h-4 w-4" />
                    </>
                  )}
                </button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
