import { useState, useEffect } from 'react';
import { X, AlertCircle } from 'lucide-react';
import { createAdminMarketingSavedQuery, updateAdminMarketingSavedQuery } from '@zamk/api-client/src/admin';
import type { SavedQuery, QueryRequest, SavedQueryUpdateRequest } from '@zamk/api-client/src/types';
import { mapSavedQueryError } from '../../utils/savedQueryError';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (query: SavedQuery) => void;
  initialQuery?: SavedQuery;
  querySpec: QueryRequest | null;
  mode: 'create' | 'rename';
}

export function AdminMarketingSaveQueryModal({ isOpen, onClose, onSuccess, initialQuery, querySpec, mode }: Props) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      if (initialQuery) {
        setName(initialQuery.name);
        setDescription(initialQuery.description || '');
      } else {
        setName('');
        setDescription('');
      }
      setError(null);
    }
  }, [isOpen, initialQuery]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError('Введите название запроса');
      return;
    }
    if (name.length > 120) {
      setError('Название не должно превышать 120 символов');
      return;
    }
    if (description.length > 500) {
      setError('Описание не должно превышать 500 символов');
      return;
    }

    setLoading(true);
    setError(null);

    try {
      let result: SavedQuery;

      if (mode === 'create') {
        if (!querySpec) throw new Error('Query spec is missing');
        result = await createAdminMarketingSavedQuery({
          name: trimmedName,
          description: description.trim() || undefined,
          querySpec,
        });
      } else {
        if (!initialQuery) throw new Error('Initial query is missing');
        const updateData: SavedQueryUpdateRequest = {
          name: trimmedName,
        };
        result = await updateAdminMarketingSavedQuery(initialQuery.id, updateData);
      }

      onSuccess(result);
    } catch (err: unknown) {
      setError(mapSavedQueryError(err));
    } finally {
      setLoading(false);
    }
  };

  const title = mode === 'create' ? 'Сохранить запрос' : 'Переименовать запрос';
  const buttonText = mode === 'create' ? 'Сохранить' : 'Переименовать';

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" aria-labelledby="modal-title" role="dialog" aria-modal="true">
      <div className="flex items-center justify-center min-h-screen p-4 text-center sm:p-0">
        <div className="fixed inset-0 bg-black/50 transition-opacity" aria-hidden="true" onClick={onClose}></div>

        <div className="relative z-10 bg-white rounded-lg px-4 pt-5 pb-4 text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:max-w-lg sm:w-full sm:p-6">
          <div className="flex justify-between items-center mb-5">
            <h3 className="text-lg leading-6 font-medium text-gray-900" id="modal-title">
              {title}
            </h3>
            <button onClick={onClose} disabled={loading} className="text-gray-400 hover:text-gray-500 disabled:opacity-50">
              <span className="sr-only">Закрыть</span>
              <X className="h-6 w-6" aria-hidden="true" />
            </button>
          </div>

          <form onSubmit={handleSubmit}>
            <div className="space-y-4">
              <div>
                <label htmlFor="query-name" className="block text-sm font-medium text-gray-700">
                  Название *
                </label>
                <div className="mt-1">
                  <input
                    type="text"
                    name="query-name"
                    id="query-name"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    maxLength={120}
                    disabled={loading}
                    className="shadow-sm focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border-gray-300 rounded-md"
                    placeholder="Например, Продажи по источникам"
                  />
                </div>
              </div>

              {mode === 'create' && (
                <div>
                  <label htmlFor="query-desc" className="block text-sm font-medium text-gray-700">
                    Описание
                  </label>
                  <div className="mt-1">
                    <textarea
                      id="query-desc"
                      name="query-desc"
                      rows={3}
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      maxLength={500}
                      disabled={loading}
                      className="shadow-sm focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border border-gray-300 rounded-md"
                      placeholder="Необязательное описание..."
                    />
                  </div>
                </div>
              )}

              {error && (
                <div className="rounded-md bg-red-50 p-4">
                  <div className="flex">
                    <div className="flex-shrink-0">
                      <AlertCircle className="h-5 w-5 text-red-400" aria-hidden="true" />
                    </div>
                    <div className="ml-3">
                      <h3 className="text-sm font-medium text-red-800">{error}</h3>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="mt-6 sm:flex sm:flex-row-reverse">
              <button
                type="submit"
                disabled={loading}
                className="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-indigo-600 text-base font-medium text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:ml-3 sm:w-auto sm:text-sm disabled:opacity-50"
              >
                {loading ? 'Сохранение...' : buttonText}
              </button>
              <button
                type="button"
                onClick={onClose}
                disabled={loading}
                className="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:mt-0 sm:w-auto sm:text-sm disabled:opacity-50"
              >
                Отмена
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
