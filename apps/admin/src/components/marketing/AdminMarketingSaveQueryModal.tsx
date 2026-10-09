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
        const updateData: SavedQueryUpdateRequest = { name: trimmedName };
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <div className="fixed inset-0 bg-gray-900/40 backdrop-blur-sm transition-opacity" onClick={onClose} />

      <div className="relative z-10 bg-white rounded-xl shadow-xl w-full max-w-[400px] overflow-hidden">
        <div className="px-5 py-4 border-b border-gray-100 flex justify-between items-center bg-gray-50/50">
          <h3 className="text-[15px] font-semibold text-gray-900" id="modal-title">{title}</h3>
          <button onClick={onClose} disabled={loading} className="text-gray-400 hover:text-gray-600 disabled:opacity-50">
            <X className="h-5 w-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5">
          <div className="space-y-4">
            <div>
              <label htmlFor="query-name" className="block text-[11px] font-bold uppercase tracking-widest text-gray-500 mb-1.5">
                Название *
              </label>
              <input
                type="text"
                id="query-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={120}
                disabled={loading}
                autoFocus
                className="w-full px-3 py-2 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
                placeholder="Например, Продажи по источникам"
              />
            </div>

            {mode === 'create' && (
              <div>
                <label htmlFor="query-desc" className="block text-[11px] font-bold uppercase tracking-widest text-gray-500 mb-1.5">
                  Описание
                </label>
                <textarea
                  id="query-desc"
                  rows={2}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  maxLength={500}
                  disabled={loading}
                  className="w-full px-3 py-2 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 resize-none"
                  placeholder="Необязательное описание..."
                />
              </div>
            )}

            {error && (
              <div className="bg-red-50 p-3 rounded-lg flex items-start border border-red-100">
                <AlertCircle className="h-4 w-4 text-red-500 mr-2 flex-shrink-0 mt-0.5" />
                <p className="text-xs text-red-700 font-medium">{error}</p>
              </div>
            )}
          </div>

          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              disabled={loading}
              className="px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 border border-gray-200 rounded-lg transition-colors disabled:opacity-50"
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-xs font-semibold text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors disabled:opacity-50 shadow-sm"
            >
              {loading ? 'Сохранение...' : buttonText}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
