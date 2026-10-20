import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { createAdminCollection, getAdminCollections } from '@zamk/api-client/src/admin';
import type { EditorialCollection } from '@zamk/api-client/src/types';
import { SLUG_PATTERN, slugifyTitle } from '../lib/collectionSlug';
import '../components/marketing/marketing.css';

const inputClass =
  'w-full text-sm border border-gray-200 rounded-lg px-3.5 py-2.5 focus:outline-none focus:border-purple-600 focus:ring-1 focus:ring-purple-600 text-gray-900 bg-white';

export function collectionStateLabel(c: Pick<EditorialCollection, 'isActive' | 'startsAt' | 'endsAt'>, now = new Date()): string {
  if (!c.isActive) return 'Черновик';
  if (c.startsAt && new Date(c.startsAt) > now) return 'Запланирована';
  if (c.endsAt && new Date(c.endsAt) <= now) return 'Завершена';
  return 'Опубликована';
}

export function AdminCollections() {
  const navigate = useNavigate();
  const [collections, setCollections] = useState<EditorialCollection[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [isCreating, setIsCreating] = useState(false);
  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');
  const [slugTouched, setSlugTouched] = useState(false);
  const [description, setDescription] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    getAdminCollections()
      .then((data) => {
        if (active) setCollections(data);
      })
      .catch((e) => {
        if (active) setError(e?.message || 'Не удалось загрузить подборки');
      })
      .finally(() => {
        if (active) setIsLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const handleTitle = (value: string) => {
    setTitle(value);
    if (!slugTouched) setSlug(slugifyTitle(value));
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isSaving) return;
    if (!title.trim()) return setFormError('Укажите название подборки.');
    if (!SLUG_PATTERN.test(slug)) return setFormError('Адрес: только строчные латинские буквы, цифры и дефисы.');
    setIsSaving(true);
    setFormError(null);
    try {
      // New collections start as drafts: nothing is visible to customers until published.
      const created = await createAdminCollection({
        title: title.trim(),
        slug,
        description: description.trim() || null,
        isActive: false,
      });
      navigate(`/collections/${created.id}`);
    } catch (err) {
      const e2 = err as { code?: string; message?: string };
      setFormError(e2.code === 'duplicate_slug' ? 'Подборка с таким адресом уже есть.' : e2.message || 'Не удалось создать подборку');
      setIsSaving(false);
    }
  };

  return (
    <div className="marketing-workspace space-y-6 pb-12" data-testid="admin-collections-page">
      <div className="m-header">
        <div>
          <h1>Подборки</h1>
          <p>Редакторские подборки товаров для витрины. Это не скидки и не рекламные кампании.</p>
        </div>
        <div className="flex items-center gap-3">
          <Link to="/collection-submissions" className="m-button">Заявки продавцов</Link>
          <button type="button" className="m-button m-primary" onClick={() => setIsCreating((v) => !v)}>
            {isCreating ? 'Отмена' : 'Создать подборку'}
          </button>
        </div>
      </div>

      {isCreating && (
        <form onSubmit={submit} className="m-panel p-5 space-y-4" data-testid="collection-create-form">
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="col-title">Название</label>
            <input id="col-title" className={inputClass} value={title} onChange={(e) => handleTitle(e.target.value)} />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="col-slug">Адрес (slug)</label>
            <input
              id="col-slug"
              className={inputClass}
              value={slug}
              onChange={(e) => {
                setSlugTouched(true);
                setSlug(e.target.value);
              }}
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1.5" htmlFor="col-desc">Краткое описание</label>
            <textarea id="col-desc" className={`${inputClass} min-h-[90px] resize-y`} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>
          {formError && <p className="text-sm text-red-600" role="alert">{formError}</p>}
          <button type="submit" className="m-button m-primary" disabled={isSaving}>
            {isSaving ? 'Создаём…' : 'Создать черновик'}
          </button>
        </form>
      )}

      {isLoading ? (
        <div className="p-12 text-center text-sm text-gray-500">Загрузка подборок...</div>
      ) : error ? (
        <div className="p-6 text-sm text-red-600" role="alert">{error}</div>
      ) : collections.length === 0 ? (
        <div className="m-panel">
          <div className="m-empty">
            <strong className="text-base text-gray-900 font-medium">Подборок пока нет</strong>
            <p className="text-gray-500 text-sm max-w-md mx-auto mt-2">Создайте первую подборку и добавьте в неё товары из каталога.</p>
          </div>
        </div>
      ) : (
        <div className="m-panel">
          <div className="m-table-scroll">
            <table className="m-table" role="table">
              <thead>
                <tr>
                  <th>Подборка</th>
                  <th>Статус</th>
                  <th>Адрес</th>
                </tr>
              </thead>
              <tbody>
                {collections.map((c) => (
                  <tr key={c.id} data-testid={`collection-row-${c.id}`}>
                    <td><Link to={`/collections/${c.id}`} className="text-purple-700 font-medium">{c.title}</Link></td>
                    <td>{collectionStateLabel(c)}</td>
                    <td className="text-gray-500">/collections/{c.slug}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
