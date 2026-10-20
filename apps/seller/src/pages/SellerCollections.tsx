import { useCallback, useEffect, useState } from 'react';
import {
  createSellerCollectionSubmission,
  getSellerCollectionOptions,
  getSellerCollectionSubmissions,
  getSellerProductsPaginated,
} from '@zamk/api-client/src/seller';
import type { CollectionSubmission, CollectionSubmissionStatus, SellerCollectionOption, SellerProduct } from '@zamk/api-client/src/types';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface } from '../components/SellerSurface';

const MAX_COMMENT = 300;

export const SUBMISSION_STATUS: Record<CollectionSubmissionStatus, { label: string; badge: string }> = {
  pending: { label: 'На рассмотрении', badge: 'bg-amber-50 text-amber-700 border-amber-200' },
  accepted: { label: 'Принята', badge: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
  rejected: { label: 'Отклонена', badge: 'bg-red-50 text-red-700 border-red-200' },
};

const inputClass =
  'w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 focus:border-gray-400 focus:outline-none focus:ring-1 focus:ring-gray-300';

export function SellerCollections() {
  const [collections, setCollections] = useState<SellerCollectionOption[]>([]);
  const [productsList, setProductsList] = useState<SellerProduct[]>([]);
  const [submissions, setSubmissions] = useState<CollectionSubmission[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState('');

  const [collectionId, setCollectionId] = useState('');
  const [productId, setProductId] = useState('');
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formError, setFormError] = useState('');
  const [formOk, setFormOk] = useState('');

  const loadSubmissions = useCallback(async () => {
    setSubmissions(await getSellerCollectionSubmissions());
  }, []);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      getSellerCollectionOptions(),
      getSellerProductsPaginated({ limit: 100 }),
      getSellerCollectionSubmissions(),
    ])
      .then(([cols, prods, subs]) => {
        if (cancelled) return;
        setCollections(cols);
        setProductsList((prods.items || []).filter((p) => p.status === 'published'));
        setSubmissions(subs);
      })
      .catch(() => {
        if (!cancelled) setLoadError('Не удалось загрузить данные. Обновите страницу.');
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isSubmitting) return;
    setFormError('');
    setFormOk('');
    if (!collectionId) return setFormError('Выберите подборку.');
    if (!productId) return setFormError('Выберите товар.');
    setIsSubmitting(true);
    try {
      await createSellerCollectionSubmission({ collectionId, productId, comment: comment.trim() || undefined });
      setFormOk('Заявка отправлена. Решение принимает редакция ZAMK.');
      setProductId('');
      setComment('');
      await loadSubmissions().catch(() => undefined);
    } catch (err) {
      setFormError((err as { message?: string })?.message || 'Не удалось отправить заявку.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <SellerPageFrame>
      <SellerPageHeader
        eyebrow="Маркетинг"
        title="Редакторские подборки"
        description="Предложите свои опубликованные товары для тематических подборок ZAMK. Участие бесплатное, решение принимает редакция."
      />

      {isLoading ? (
        <p className="mt-8 text-sm text-gray-500">Загрузка…</p>
      ) : loadError ? (
        <p className="mt-8 text-sm text-red-600" role="alert">{loadError}</p>
      ) : (
        <div className="mt-8 space-y-8">
          <SellerSurface className="p-5 sm:p-6" data-testid="submission-form-card">
            <h2 className="text-base font-semibold text-gray-900">Новая заявка</h2>
            {collections.length === 0 ? (
              <p className="mt-3 text-sm text-gray-500" data-testid="no-collections">
                Сейчас нет опубликованных подборок. Загляните позже.
              </p>
            ) : productsList.length === 0 ? (
              <p className="mt-3 text-sm text-gray-500" data-testid="no-products">
                У вас пока нет опубликованных товаров, которые можно предложить.
              </p>
            ) : (
              <form onSubmit={submit} className="mt-4 space-y-4">
                <div>
                  <label htmlFor="sub-collection" className="mb-1.5 block text-xs font-medium text-gray-700">Подборка</label>
                  <select id="sub-collection" className={inputClass} value={collectionId} onChange={(e) => setCollectionId(e.target.value)}>
                    <option value="">Выберите подборку</option>
                    {collections.map((c) => (
                      <option key={c.id} value={c.id}>{c.title}</option>
                    ))}
                  </select>
                  {collectionId && collections.find((c) => c.id === collectionId)?.description && (
                    <p className="mt-1.5 text-xs text-gray-500">{collections.find((c) => c.id === collectionId)?.description}</p>
                  )}
                </div>
                <div>
                  <label htmlFor="sub-product" className="mb-1.5 block text-xs font-medium text-gray-700">Товар</label>
                  <select id="sub-product" className={inputClass} value={productId} onChange={(e) => setProductId(e.target.value)}>
                    <option value="">Выберите товар</option>
                    {productsList.map((p) => (
                      <option key={p.id} value={p.id}>{p.title}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label htmlFor="sub-comment" className="mb-1.5 block text-xs font-medium text-gray-700">
                    Комментарий о вещи (необязательно)
                  </label>
                  <textarea
                    id="sub-comment"
                    className={`${inputClass} min-h-[80px] resize-y`}
                    maxLength={MAX_COMMENT}
                    value={comment}
                    onChange={(e) => setComment(e.target.value)}
                    placeholder="Коротко: чем эта вещь подходит подборке"
                  />
                  <div className="mt-1 text-right text-xs text-gray-400">{comment.length}/{MAX_COMMENT}</div>
                </div>
                {formError && <p className="text-sm text-red-600" role="alert">{formError}</p>}
                {formOk && <p className="text-sm text-emerald-700" role="status">{formOk}</p>}
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="inline-flex items-center rounded-lg bg-black px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50 cursor-pointer"
                  data-testid="submission-submit"
                >
                  {isSubmitting ? 'Отправляем…' : 'Отправить заявку'}
                </button>
              </form>
            )}
          </SellerSurface>

          <section>
            <h2 className="mb-3 text-base font-semibold text-gray-900">Мои заявки</h2>
            {submissions.length === 0 ? (
              <SellerSurface className="p-6 text-sm text-gray-500" data-testid="no-submissions">Вы ещё не отправляли заявок.</SellerSurface>
            ) : (
              <SellerSurface className="overflow-x-auto">
                <table className="w-full text-left text-sm" data-testid="submissions-table">
                  <thead className="border-b border-gray-200 bg-gray-50 text-xs font-semibold uppercase tracking-wider text-gray-500">
                    <tr>
                      <th className="px-4 py-3">Товар</th>
                      <th className="px-4 py-3">Подборка</th>
                      <th className="px-4 py-3">Статус</th>
                      <th className="px-4 py-3">Дата</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-200">
                    {submissions.map((s) => (
                      <tr key={s.id} data-testid={`submission-row-${s.id}`}>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-3">
                            {s.product.imageUrl ? (
                              <img src={s.product.imageUrl} alt="" className="h-10 w-10 rounded object-cover" />
                            ) : (
                              <div className="h-10 w-10 rounded bg-gray-100" />
                            )}
                            <div>
                              <div className="font-medium text-gray-900">{s.product.title}</div>
                              {s.sellerComment && <div className="text-xs text-gray-500">«{s.sellerComment}»</div>}
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-gray-700">{s.collectionTitle}</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2.5 py-1 text-xs font-medium ${SUBMISSION_STATUS[s.status].badge}`} data-testid={`submission-status-${s.id}`}>
                            {SUBMISSION_STATUS[s.status].label}
                          </span>
                          {s.status === 'rejected' && s.rejectionReason && (
                            <div className="mt-1 text-xs text-gray-600" data-testid={`submission-reason-${s.id}`}>
                              Причина: {s.rejectionReason}
                            </div>
                          )}
                          {s.status === 'accepted' && (
                            <div className="mt-1 text-xs text-gray-500">Редакция может изменить состав подборки позже.</div>
                          )}
                        </td>
                        <td className="px-4 py-3 text-xs text-gray-500">{new Date(s.createdAt).toLocaleDateString('ru-RU')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </SellerSurface>
            )}
          </section>
        </div>
      )}
    </SellerPageFrame>
  );
}
