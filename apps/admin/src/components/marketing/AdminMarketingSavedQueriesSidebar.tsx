import { useState, useEffect } from 'react';
import { X, Trash2, Edit2, Play, AlertCircle, RefreshCw } from 'lucide-react';
import { listAdminMarketingSavedQueries, deleteAdminMarketingSavedQuery } from '@zamk/api-client/src/admin';
import type { SavedQuery } from '@zamk/api-client/src/types';
import { mapSavedQueryError } from '../../utils/savedQueryError';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (query: SavedQuery) => void;
  onRename: (query: SavedQuery) => void;
  onDeleted: (query: SavedQuery) => void;
  activeQueryId: string | null;
  refreshKey: number;
}

export function AdminMarketingSavedQueriesSidebar({
  isOpen,
  onClose,
  onSelect,
  onRename,
  onDeleted,
  activeQueryId,
  refreshKey,
}: Props) {
  const [queries, setQueries] = useState<SavedQuery[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmingQuery, setConfirmingQuery] = useState<SavedQuery | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const fetchQueries = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await listAdminMarketingSavedQueries();
      setQueries(data);
    } catch (err: unknown) {
      setError(mapSavedQueryError(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      fetchQueries();
    }
  }, [isOpen, refreshKey]);

  const handleConfirmDelete = async () => {
    if (!confirmingQuery) return;
    setIsDeleting(true);
    setError(null);
    try {
      await deleteAdminMarketingSavedQuery(confirmingQuery.id);
      const deleted = confirmingQuery;
      setConfirmingQuery(null);
      onDeleted(deleted);
      await fetchQueries();
    } catch (err: unknown) {
      setError(mapSavedQueryError(err));
    } finally {
      setIsDeleting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-hidden" data-testid="saved-queries-sidebar">
      <div className="absolute inset-0 bg-gray-500 bg-opacity-75 transition-opacity" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 max-w-sm w-full bg-white shadow-xl flex flex-col">
        <div className="px-4 py-6 bg-gray-50 flex items-center justify-between border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">Сохранённые запросы</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-500">
            <span className="sr-only">Закрыть</span>
            <X className="h-6 w-6" aria-hidden="true" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {error && (
            <div className="mb-4 bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-md flex items-start">
              <AlertCircle className="h-5 w-5 mr-2 flex-shrink-0 mt-0.5" />
              <div className="flex-1 text-sm">{error}</div>
              <button
                onClick={fetchQueries}
                className="ml-2 text-red-700 hover:text-red-800 text-sm font-medium"
              >
                Повторить
              </button>
            </div>
          )}

          {/* In-product compact delete confirmation dialog */}
          {confirmingQuery && (
            <div className="mb-4 bg-amber-50 border border-amber-200 rounded-lg p-4" data-testid="delete-confirmation-dialog">
              <p className="text-sm font-medium text-amber-900 mb-3">
                Удалить «{confirmingQuery.name}»?
              </p>
              <div className="flex items-center space-x-2">
                <button
                  onClick={handleConfirmDelete}
                  disabled={isDeleting}
                  className="px-3 py-1.5 bg-red-600 text-white text-xs font-medium rounded hover:bg-red-700 disabled:opacity-50"
                >
                  {isDeleting ? 'Удаление...' : 'Удалить'}
                </button>
                <button
                  onClick={() => setConfirmingQuery(null)}
                  disabled={isDeleting}
                  className="px-3 py-1.5 bg-white border border-gray-300 text-gray-700 text-xs font-medium rounded hover:bg-gray-50 disabled:opacity-50"
                >
                  Отмена
                </button>
              </div>
            </div>
          )}

          {loading ? (
            <div className="flex justify-center py-8">
              <RefreshCw className="h-6 w-6 text-indigo-600 animate-spin" />
            </div>
          ) : queries.length === 0 && !error ? (
            <div className="text-center py-8">
              <p className="text-sm text-gray-500 mb-1">Сохранённых запросов пока нет</p>
              <p className="text-xs text-gray-400">Настройте отчёт и сохраните его, чтобы вернуться к нему позже.</p>
            </div>
          ) : (
            <div className="space-y-3">
              {queries.map((q) => (
                <div
                  key={q.id}
                  data-testid={`saved-query-item-${q.id}`}
                  className={`bg-white border rounded-lg p-4 transition-colors ${
                    activeQueryId === q.id ? 'border-indigo-500 ring-1 ring-indigo-500' : 'border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div className="flex justify-between items-start mb-2">
                    <h3 className="text-sm font-medium text-gray-900 break-words flex-1 pr-2">
                      {q.name}
                    </h3>
                    <div className="flex items-center space-x-1 flex-shrink-0">
                      <button
                        onClick={() => onRename(q)}
                        className="text-gray-400 hover:text-gray-600 p-1"
                        title="Переименовать"
                      >
                        <Edit2 className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setConfirmingQuery(q)}
                        disabled={confirmingQuery?.id === q.id}
                        className="text-gray-400 hover:text-red-600 p-1 disabled:opacity-50"
                        title="Удалить"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>
                  {q.description && (
                    <p className="text-xs text-gray-500 mb-3 line-clamp-2 break-words">
                      {q.description}
                    </p>
                  )}
                  <div className="flex items-center justify-between mt-3 pt-3 border-t border-gray-100">
                    <div className="text-xs text-gray-400">
                      {new Date(q.updatedAt).toLocaleDateString('ru-RU', {
                        day: '2-digit',
                        month: '2-digit',
                        year: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </div>
                    <button
                      onClick={() => onSelect(q)}
                      className="inline-flex items-center text-xs font-medium text-indigo-600 hover:text-indigo-800"
                    >
                      <Play className="h-3 w-3 mr-1" />
                      Выбрать
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
