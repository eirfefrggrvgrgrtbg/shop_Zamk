import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  getAdminCollection,
  getAdminProducts,
  replaceAdminCollectionItems,
  updateAdminCollection,
} from '@zamk/api-client/src/admin';
import type { AdminCollectionItem, AdminProduct } from '@zamk/api-client/src/types';
import { SLUG_PATTERN, moveItem } from '../lib/collectionSlug';
import { collectionStateLabel } from './AdminCollections';
import '../components/marketing/marketing.css';

const inputClass =
  'w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white';

const productStatusLabels: Record<string, string> = {
  published: 'Опубликован',
  approved: 'Одобрен, не опубликован',
  hidden: 'Скрыт',
  draft: 'Черновик',
  pending_review: 'На модерации',
  in_review: 'На модерации',
  rejected: 'Отклонён',
  blocked: 'Заблокирован',
};

/** Date-time in `datetime-local` format, in the browser's local time. */
function toLocalInput(iso?: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromLocalInput(value: string): string | null {
  return value ? new Date(value).toISOString() : null;
}

export function AdminCollectionDetail() {
  const { id = '' } = useParams<{ id: string }>();
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [coverImageUrl, setCoverImageUrl] = useState('');
  const [isActive, setIsActive] = useState(false);
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [items, setItems] = useState<AdminCollectionItem[]>([]);
  const [savedState, setSavedState] = useState<{ isActive: boolean; startsAt?: string | null; endsAt?: string | null } | null>(null);

  const [query, setQuery] = useState('');
  const [results, setResults] = useState<AdminProduct[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const searchSeq = useRef(0);

  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);

  const load = useCallback(async () => {
    setIsLoading(true);
    setLoadError(null);
    try {
      const c = await getAdminCollection(id);
      setTitle(c.title);
      setSlug(c.slug);
      setDescription(c.description ?? '');
      setCoverImageUrl(c.coverImageUrl ?? '');
      setIsActive(c.isActive);
      setStartsAt(toLocalInput(c.startsAt));
      setEndsAt(toLocalInput(c.endsAt));
      setItems([...c.items].sort((a, b) => a.sortOrder - b.sortOrder));
      setSavedState({ isActive: c.isActive, startsAt: c.startsAt, endsAt: c.endsAt });
    } catch (e) {
      const err = e as { status?: number; message?: string };
      setLoadError(err.status === 404 ? 'Подборка не найдена' : err.message || 'Не удалось загрузить подборку');
    } finally {
      setIsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  // Product search (published products only; the API also accepts others, but they would never be shown).
  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      searchSeq.current += 1;
      setResults([]);
      setIsSearching(false);
      return;
    }
    const seq = ++searchSeq.current;
    setIsSearching(true);
    const timer = setTimeout(() => {
      getAdminProducts({ q, status: 'published', limit: 8, page: 1 })
        .then((res) => {
          if (seq !== searchSeq.current) return;
          setResults(res.items);
          setSearchError(null);
        })
        .catch((e) => {
          if (seq === searchSeq.current) setSearchError(e?.message || 'Не удалось выполнить поиск');
        })
        .finally(() => {
          if (seq === searchSeq.current) setIsSearching(false);
        });
    }, 300);
    return () => clearTimeout(timer);
  }, [query]);

  const addProduct = (p: AdminProduct) => {
    setItems((prev) =>
      prev.some((i) => i.productId === p.id)
        ? prev
        : [...prev, { productId: p.id, title: p.title, status: p.status, sortOrder: prev.length }],
    );
    setMessage(null);
  };

  const removeProduct = (productId: string) => {
    setItems((prev) => prev.filter((i) => i.productId !== productId));
    setMessage(null);
  };

  const save = async () => {
    if (isSaving) return;
    if (!title.trim()) return setMessage({ kind: 'error', text: 'Укажите название.' });
    if (!SLUG_PATTERN.test(slug)) return setMessage({ kind: 'error', text: 'Адрес: только строчные латинские буквы, цифры и дефисы.' });
    if (startsAt && endsAt && new Date(startsAt) >= new Date(endsAt)) {
      return setMessage({ kind: 'error', text: 'Дата окончания должна быть позже даты начала.' });
    }
    setIsSaving(true);
    setMessage(null);
    try {
      await updateAdminCollection(id, {
        title: title.trim(),
        slug,
        description: description.trim() || null,
        coverImageUrl: coverImageUrl.trim() || null,
        isActive,
        startsAt: fromLocalInput(startsAt),
        endsAt: fromLocalInput(endsAt),
      });
      await replaceAdminCollectionItems(id, items.map((i) => i.productId));
      await load();
      setMessage({ kind: 'ok', text: 'Сохранено.' });
    } catch (err) {
      const e = err as { code?: string; message?: string };
      const text =
        e.code === 'duplicate_slug'
          ? 'Подборка с таким адресом уже есть.'
          : e.code === 'product_not_found'
            ? 'Один из товаров больше не существует. Обновите страницу.'
            : e.message || 'Не удалось сохранить';
      setMessage({ kind: 'error', text });
    } finally {
      setIsSaving(false);
    }
  };

  if (isLoading) return <div className="p-12 text-center text-sm text-gray-500">Загрузка подборки...</div>;
  if (loadError) {
    return (
      <div className="p-6 space-y-3">
        <p className="text-sm text-red-600" role="alert">{loadError}</p>
        <Link to="/collections" className="text-purple-700 text-sm">← К списку подборок</Link>
      </div>
    );
  }

  const hiddenCount = items.filter((i) => i.status !== 'published').length;

  return (
    <div className="marketing-workspace space-y-6 pb-12" data-testid="admin-collection-detail">
      <div className="m-header">
        <div>
          <Link to="/collections" className="text-sm text-gray-500">← Все подборки</Link>
          <h1>{title || 'Подборка'}</h1>
          <p>
            Статус: <strong data-testid="collection-state">{savedState ? collectionStateLabel(savedState) : ''}</strong>
            {' · '}/collections/{slug}
          </p>
        </div>
        <button type="button" className="m-button m-primary" onClick={save} disabled={isSaving} data-testid="collection-save">
          {isSaving ? 'Сохраняем…' : 'Сохранить'}
        </button>
      </div>

      {message && (
        <p className={`text-sm ${message.kind === 'ok' ? 'text-green-700' : 'text-red-600'}`} role={message.kind === 'error' ? 'alert' : 'status'}>
          {message.text}
        </p>
      )}

      <section className="m-panel p-5 space-y-4">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">Описание</h2>
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-title">Название</label>
          <input id="c-title" className={inputClass} value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-slug">Адрес (slug)</label>
          <input id="c-slug" className={inputClass} value={slug} onChange={(e) => setSlug(e.target.value)} />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-desc">Краткое описание</label>
          <textarea id="c-desc" className={`${inputClass} min-h-[90px] resize-y`} value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-cover">Обложка (необязательно, URL)</label>
          <input id="c-cover" className={inputClass} value={coverImageUrl} onChange={(e) => setCoverImageUrl(e.target.value)} placeholder="https://…" />
        </div>
      </section>

      <section className="m-panel p-5 space-y-4">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">Публикация</h2>
        <label className="flex items-center gap-2 cursor-pointer text-sm text-gray-900">
          <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} data-testid="collection-active" />
          Опубликована в магазине
        </label>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-start">Показывать с (необязательно)</label>
            <input id="c-start" type="datetime-local" className={inputClass} value={startsAt} onChange={(e) => setStartsAt(e.target.value)} />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-end">Показывать до (необязательно)</label>
            <input id="c-end" type="datetime-local" className={inputClass} value={endsAt} onChange={(e) => setEndsAt(e.target.value)} />
          </div>
        </div>
        <p className="text-xs text-gray-500">Покупатели видят только опубликованные товары, доступные к покупке (например, с остатком). Подборка не влияет на цены, скидки и промокоды.</p>
      </section>

      <section className="m-panel p-5 space-y-4">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-900">Товары ({items.length})</h2>

        <div className="relative">
          <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="c-search">Добавить товар (поиск среди опубликованных)</label>
          <input id="c-search" type="search" className={inputClass} value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Название товара…" />
          {isSearching && <p className="mt-2 text-xs text-gray-500">Поиск…</p>}
          {searchError && <p className="mt-2 text-xs text-red-600" role="alert">{searchError}</p>}
          {!isSearching && query.trim().length >= 2 && results.length === 0 && !searchError && (
            <p className="mt-2 text-xs text-gray-500">Ничего не найдено.</p>
          )}
          {results.length > 0 && (
            <ul className="mt-2 border border-gray-200 rounded-lg divide-y divide-gray-100" data-testid="collection-search-results">
              {results.map((p) => {
                const added = items.some((i) => i.productId === p.id);
                return (
                  <li key={p.id} className="flex items-center justify-between gap-3 px-3.5 py-2 text-sm">
                    <span className="text-gray-900 break-words min-w-0">{p.title}</span>
                    <button type="button" className="m-button" disabled={added} onClick={() => addProduct(p)}>
                      {added ? 'Добавлен' : 'Добавить'}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {items.length === 0 ? (
          <p className="text-sm text-gray-500">В подборке пока нет товаров.</p>
        ) : (
          <>
            {hiddenCount > 0 && (
              <p className="text-xs text-amber-700" role="note">
                Не опубликованные товары ({hiddenCount}) остаются в подборке, но покупатели их не увидят.
              </p>
            )}
            <ol className="border border-gray-200 rounded-lg divide-y divide-gray-100" data-testid="collection-items">
              {items.map((it, index) => (
                <li key={it.productId} className="flex items-center gap-3 px-3.5 py-2 text-sm" data-testid={`collection-item-${it.productId}`}>
                  <span className="w-6 text-gray-400 tabular-nums">{index + 1}</span>
                  <span className="flex-1 min-w-0 break-words text-gray-900">
                    {it.title}
                    {it.status !== 'published' && (
                      <span className="ml-2 text-xs text-amber-700">{productStatusLabels[it.status] ?? it.status}</span>
                    )}
                  </span>
                  <button type="button" className="m-button" aria-label={`Выше: ${it.title}`} disabled={index === 0} onClick={() => setItems((prev) => moveItem(prev, index, -1))}>↑</button>
                  <button type="button" className="m-button" aria-label={`Ниже: ${it.title}`} disabled={index === items.length - 1} onClick={() => setItems((prev) => moveItem(prev, index, 1))}>↓</button>
                  <button type="button" className="m-button" aria-label={`Убрать: ${it.title}`} onClick={() => removeProduct(it.productId)}>Убрать</button>
                </li>
              ))}
            </ol>
          </>
        )}
      </section>
    </div>
  );
}
