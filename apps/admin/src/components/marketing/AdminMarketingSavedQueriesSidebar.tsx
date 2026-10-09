import { useState, useEffect, useRef } from 'react';
import { Trash2, Edit2, Play, AlertCircle, RefreshCw } from 'lucide-react';
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


  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      // Allow clicking the trigger button itself without auto-closing immediately,
      // but since we only have the popoverRef here, we might close if they click outside.
      // Usually, the trigger button click toggles state. If they click the trigger button,
      // it might close the popover. We can just rely on standard outside click.
      if (isOpen && popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        // Find if they clicked the trigger button (which has the word Сохранённые)
        const target = e.target as HTMLElement;
        if (!target.closest('.m-button')) {
          onClose();
        }
      }
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) onClose();
    };
    if (isOpen) {
      document.addEventListener('mousedown', handleOutsideClick);
      document.addEventListener('keydown', handleEscape);
    }
    return () => {
      document.removeEventListener('mousedown', handleOutsideClick);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;


  return (
    <div
      className="absolute right-0 mt-2 w-[400px] bg-white rounded-lg shadow-xl ring-1 ring-black ring-opacity-5 z-20 flex flex-col max-h-[600px]"
      data-testid="saved-queries-sidebar"
      ref={popoverRef}
    >
        <div className="px-4 py-3 bg-gray-50 border-b border-gray-100 rounded-t-lg flex items-center justify-between">
          <h2 className="text-sm font-semibold text-gray-900">Сохранённые запросы</h2>
        </div>

        <div className="flex-1 overflow-y-auto p-3">
          {error && (
            <div className="mb-3 bg-red-50 border border-red-200 text-red-600 px-3 py-2 rounded flex items-start">
              <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0 mt-0.5" />
              <div className="flex-1 text-xs">{error}</div>
              <button onClick={fetchQueries} className="ml-2 text-red-700 hover:text-red-800 text-xs font-bold uppercase tracking-widest">
                Повторить
              </button>
            </div>
          )}

          {confirmingQuery && (
            <div className="mb-3 bg-amber-50 border border-amber-200 rounded p-3" data-testid="delete-confirmation-dialog">
              <p className="text-xs font-medium text-amber-900 mb-2">
                Удалить «{confirmingQuery.name}»?
              </p>
              <div className="flex items-center space-x-2">
                <button
                  onClick={handleConfirmDelete}
                  disabled={isDeleting}
                  className="px-2 py-1 bg-red-600 text-white text-[11px] font-bold uppercase tracking-widest rounded hover:bg-red-700 disabled:opacity-50"
                >
                  {isDeleting ? 'Удаление...' : 'Удалить'}
                </button>
                <button
                  onClick={() => setConfirmingQuery(null)}
                  disabled={isDeleting}
                  className="px-2 py-1 bg-white border border-gray-300 text-gray-700 text-[11px] font-bold uppercase tracking-widest rounded hover:bg-gray-50 disabled:opacity-50"
                >
                  Отмена
                </button>
              </div>
            </div>
          )}

          {loading ? (
            <div className="flex justify-center py-6">
              <RefreshCw className="h-5 w-5 text-indigo-600 animate-spin" />
            </div>
          ) : queries.length === 0 && !error ? (
            <div className="text-center py-6 px-2">
              <p className="text-sm text-gray-500 font-medium mb-1">Запросов пока нет</p>
              <p className="text-xs text-gray-400">Настройте отчёт и сохраните его, чтобы вернуться к нему позже.</p>
            </div>
          ) : (
            <div className="space-y-2">
              {queries.map((q) => (
                <div
                  key={q.id}
                  data-testid={`saved-query-item-${q.id}`}
                  className={`bg-white border rounded p-3 transition-colors ${
                    activeQueryId === q.id ? 'border-indigo-500 ring-1 ring-indigo-500' : 'border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <div className="flex justify-between items-start mb-1.5">
                    <h3 className="text-sm font-semibold text-gray-900 break-words flex-1 pr-2">
                      {q.name}
                    </h3>
                    <div className="flex items-center space-x-1 flex-shrink-0">
                      <button
                        onClick={() => onRename(q)}
                        className="text-gray-400 hover:text-gray-600 p-1"
                        title="Переименовать"
                      >
                        <Edit2 className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => setConfirmingQuery(q)}
                        disabled={confirmingQuery?.id === q.id}
                        className="text-gray-400 hover:text-red-600 p-1 disabled:opacity-50"
                        title="Удалить" data-testid="sidebar-query-item-delete"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                  {q.description && (
                    <p className="text-[11px] text-gray-500 mb-2 line-clamp-2 break-words">
                      {q.description}
                    </p>
                  )}
                  <div className="flex items-center justify-between mt-2 pt-2 border-t border-gray-50">
                    <div className="text-[10px] uppercase tracking-widest text-gray-400 font-semibold">
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
                      className="inline-flex items-center text-[11px] font-bold uppercase tracking-widest text-indigo-600 hover:text-indigo-800"
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
  );
}
