import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  acceptAdminCollectionSubmission,
  getAdminCollectionSubmissions,
  rejectAdminCollectionSubmission,
} from '@zamk/api-client/src/admin';
import type { AdminCollectionSubmission, CollectionSubmissionStatus } from '@zamk/api-client/src/types';
import '../components/marketing/marketing.css';

type Filter = CollectionSubmissionStatus | 'all';

const FILTER_LABELS: Record<Filter, string> = {
  pending: 'Ожидают рассмотрения',
  accepted: 'Приняты',
  rejected: 'Отклонены',
  all: 'Вся история',
};

const STATUS_LABELS: Record<CollectionSubmissionStatus, string> = {
  pending: 'Ожидает',
  accepted: 'Принята',
  rejected: 'Отклонена',
};

const inputClass =
  'w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white';

export function AdminCollectionSubmissions() {
  const [filter, setFilter] = useState<Filter>('pending');
  const [items, setItems] = useState<AdminCollectionSubmission[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [rejectingId, setRejectingId] = useState<string | null>(null);
  const [reason, setReason] = useState('');
  const [rowError, setRowError] = useState<{ id: string; text: string } | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async (f: Filter) => {
    try {
      setItems(await getAdminCollectionSubmissions(f));
      setLoadError(null);
    } catch (e) {
      setLoadError((e as { message?: string })?.message || 'Не удалось загрузить заявки');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    setIsLoading(true);
    load(filter);
  }, [filter, load]);

  const act = async (id: string, kind: 'accept' | 'reject') => {
    if (busyId) return;
    setBusyId(id);
    setRowError(null);
    setNotice(null);
    try {
      if (kind === 'accept') {
        await acceptAdminCollectionSubmission(id);
        setNotice('Заявка принята, товар добавлен в подборку.');
      } else {
        await rejectAdminCollectionSubmission(id, reason.trim());
        setNotice('Заявка отклонена.');
        setRejectingId(null);
        setReason('');
      }
    } catch (e) {
      setRowError({ id, text: (e as { message?: string })?.message || 'Не удалось выполнить действие' });
    } finally {
      setBusyId(null);
      await load(filter); // the list always reflects the server state, also after a conflict
    }
  };

  return (
    <div className="marketing-workspace space-y-6 pb-12" data-testid="admin-collection-submissions">
      <div className="m-header">
        <div>
          <h1>Заявки в подборки</h1>
          <p>Продавцы предлагают свои товары для редакторских подборок. Участие бесплатное, решение за редакцией.</p>
        </div>
        <Link to="/collections" className="m-button">К подборкам</Link>
      </div>

      <div className="flex items-center gap-3">
        <label htmlFor="sub-filter" className="text-sm text-gray-600">Статус</label>
        <select id="sub-filter" className={`${inputClass} max-w-xs`} value={filter} onChange={(e) => setFilter(e.target.value as Filter)}>
          {(Object.keys(FILTER_LABELS) as Filter[]).map((k) => (
            <option key={k} value={k}>{FILTER_LABELS[k]}</option>
          ))}
        </select>
      </div>

      {notice && <p className="text-sm text-green-700" role="status">{notice}</p>}

      {isLoading ? (
        <div className="p-12 text-center text-sm text-gray-500">Загрузка заявок...</div>
      ) : loadError ? (
        <p className="text-sm text-red-600" role="alert">{loadError}</p>
      ) : items.length === 0 ? (
        <div className="m-panel">
          <div className="m-empty">
            <strong className="text-base text-gray-900 font-medium" data-testid="submissions-empty">
              {filter === 'pending' ? 'Нет заявок, ожидающих рассмотрения' : 'Заявок нет'}
            </strong>
          </div>
        </div>
      ) : (
        <ul className="space-y-3" data-testid="submissions-list">
          {items.map((s) => (
            <li key={s.id} className="m-panel p-4" data-testid={`submission-${s.id}`}>
              <div className="flex flex-col gap-4 sm:flex-row">
                {s.product.imageUrl ? (
                  <img src={s.product.imageUrl} alt={s.product.title} className="h-28 w-24 rounded-lg object-cover shrink-0" />
                ) : (
                  <div className="flex h-28 w-24 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-xs text-gray-400">Нет фото</div>
                )}
                <div className="min-w-0 flex-1 space-y-1 text-sm">
                  <div className="text-xs uppercase tracking-wider text-gray-500">{s.brandName}</div>
                  <div className="text-base font-medium text-gray-900 break-words">{s.product.title}</div>
                  <div className="text-gray-700">
                    Подборка: <span className="font-medium">{s.collectionTitle}</span>
                  </div>
                  {s.sellerComment && <div className="text-gray-600">Комментарий продавца: «{s.sellerComment}»</div>}
                  <div className="text-xs text-gray-500">
                    Подана {new Date(s.createdAt).toLocaleDateString('ru-RU')} ·{' '}
                    <Link to={`/products/${s.product.id}`} className="text-purple-700 underline" target="_blank" rel="noreferrer">
                      Открыть карточку товара
                    </Link>
                    {s.product.status !== 'published' && (
                      <span className="ml-2 text-amber-700">Товар сейчас не опубликован</span>
                    )}
                  </div>
                  {s.status !== 'pending' && (
                    <div className="text-xs text-gray-600" data-testid={`submission-review-${s.id}`}>
                      {STATUS_LABELS[s.status]}
                      {s.reviewedAt ? ` ${new Date(s.reviewedAt).toLocaleDateString('ru-RU')}` : ''}
                      {s.reviewerName ? `, рассмотрел(а): ${s.reviewerName}` : ''}
                      {s.rejectionReason ? ` · причина: ${s.rejectionReason}` : ''}
                    </div>
                  )}
                  {rowError?.id === s.id && <p className="text-sm text-red-600" role="alert">{rowError.text}</p>}
                </div>

                {s.status === 'pending' && (
                  <div className="flex shrink-0 flex-col gap-2 sm:w-56">
                    {rejectingId === s.id ? (
                      <>
                        <textarea
                          className={`${inputClass} min-h-[70px] resize-y`}
                          placeholder="Причина (видна продавцу, необязательно)"
                          maxLength={300}
                          aria-label="Причина отклонения"
                          value={reason}
                          onChange={(e) => setReason(e.target.value)}
                        />
                        <button type="button" className="m-button m-primary" disabled={busyId === s.id} onClick={() => act(s.id, 'reject')} data-testid={`confirm-reject-${s.id}`}>
                          Отклонить заявку
                        </button>
                        <button type="button" className="m-button" onClick={() => { setRejectingId(null); setReason(''); }}>Отмена</button>
                      </>
                    ) : (
                      <>
                        <button type="button" className="m-button m-primary" disabled={busyId === s.id} onClick={() => act(s.id, 'accept')} data-testid={`accept-${s.id}`}>
                          Принять в подборку
                        </button>
                        <button type="button" className="m-button" disabled={busyId === s.id} onClick={() => { setRejectingId(s.id); setRowError(null); }} data-testid={`reject-${s.id}`}>
                          Отклонить
                        </button>
                      </>
                    )}
                  </div>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
