import { useState, useEffect, useCallback } from 'react';
import { listAuditLogs } from '@zamk/api-client/src/admin';
import { AlertCircle, ChevronDown, ChevronRight, ClipboardList, Search, Filter } from 'lucide-react';

interface AuditLog {
  id: string;
  actorUserId?: string;
  actorEmail?: string;
  actorRole?: string;
  permission?: string;
  action: string;
  entityType?: string;
  entityId?: string;
  requestId?: string;
  ip?: string;
  userAgent?: string;
  metadata: Record<string, any>;
  createdAt: string;
}

const PAGE_SIZE = 50;

function MetadataCell({ metadata }: { metadata: Record<string, any> }) {
  const [open, setOpen] = useState(false);
  const keys = Object.keys(metadata ?? {});
  if (keys.length === 0) return <span className="text-gray-400 text-xs">—</span>;

  return (
    <div>
      <button onClick={() => setOpen(o => !o)} className="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800">
        {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        Детали ({keys.length})
      </button>
      {open && (
        <pre className="mt-1 text-xs bg-gray-50 border border-gray-200 rounded p-2 overflow-auto max-w-xs max-h-40">
          {JSON.stringify(metadata, null, 2)}
        </pre>
      )}
    </div>
  );
}

export function AdminAuditLogs() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [filterQ, setFilterQ] = useState('');
  const [filterAction, setFilterAction] = useState('');
  const [filterEntityType, setFilterEntityType] = useState('');

  const [activeFilters, setActiveFilters] = useState({ q: '', action: '', entityType: '' });

  const loadLogs = useCallback(async (pageNum: number, filters: typeof activeFilters) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await listAuditLogs({
        limit: PAGE_SIZE,
        offset: pageNum * PAGE_SIZE,
        q: filters.q,
        action: filters.action,
        entityType: filters.entityType,
      });
      setLogs(data.items ?? []);
      setTotal(data.totalCount ?? 0);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить журнал действий');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => { loadLogs(page, activeFilters); }, [loadLogs, page, activeFilters]);

  const applyFilters = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(0);
    setActiveFilters({ q: filterQ, action: filterAction, entityType: filterEntityType });
  };

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Журнал действий</h1>
          <p className="mt-1 text-sm text-gray-500">
            {total > 0 ? `${total} записей` : 'Записей нет'}
          </p>
        </div>
      </div>

      <div className="bg-white p-4 rounded-lg shadow border border-gray-200">
        <form onSubmit={applyFilters} className="flex flex-col sm:flex-row gap-4 items-end">
          <div className="flex-1">
            <label className="block text-sm font-medium text-gray-700 mb-1">Поиск</label>
            <div className="relative rounded-md shadow-sm">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <Search className="h-4 w-4 text-gray-400" />
              </div>
              <input
                type="text"
                className="focus:ring-indigo-500 focus:border-indigo-500 block w-full pl-10 sm:text-sm border-gray-300 rounded-md py-2 px-3 border"
                placeholder="Почта, действие, тип..."
                value={filterQ}
                onChange={e => setFilterQ(e.target.value)}
              />
            </div>
          </div>
          <div className="w-full sm:w-48">
            <label className="block text-sm font-medium text-gray-700 mb-1">Действие</label>
            <input
              type="text"
              className="focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border-gray-300 rounded-md py-2 px-3 border"
              placeholder="Точное совпадение..."
              value={filterAction}
              onChange={e => setFilterAction(e.target.value)}
            />
          </div>
          <div className="w-full sm:w-48">
            <label className="block text-sm font-medium text-gray-700 mb-1">Тип сущности</label>
            <input
              type="text"
              className="focus:ring-indigo-500 focus:border-indigo-500 block w-full sm:text-sm border-gray-300 rounded-md py-2 px-3 border"
              placeholder="Например, orders"
              value={filterEntityType}
              onChange={e => setFilterEntityType(e.target.value)}
            />
          </div>
          <button
            type="submit"
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            <Filter className="h-4 w-4 mr-2" />
            Фильтр
          </button>
        </form>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2 shrink-0" />
          {error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-2 text-sm text-gray-500">Загрузка журнала...</p>
        </div>
      ) : logs.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow border border-gray-200">
          <ClipboardList className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Записей нет</h3>
          <p className="mt-1 text-sm text-gray-500">По вашему запросу ничего не найдено.</p>
        </div>
      ) : (
        <>
          <div className="shadow overflow-hidden border border-gray-200 sm:rounded-lg">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Дата / Время</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Действие</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Тип записи</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Актор</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">IP</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Детали</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {logs.map((log) => (
                  <tr key={log.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-gray-500">
                      {new Date(log.createdAt).toLocaleString('ru-RU')}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <span className="text-xs font-mono text-gray-800 bg-gray-100 px-2 py-1 rounded">{log.action}</span>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-gray-500">
                      {log.entityType ? <span className="font-medium text-gray-700">{log.entityType}</span> : '—'}
                      {log.entityId && (
                        <div className="text-gray-400 font-mono text-[10px] mt-0.5" title={log.entityId}>
                          {log.entityId.substring(0, 8)}…
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-gray-600">
                      <div className="font-medium">{log.actorEmail ?? 'Система'}</div>
                      {log.actorRole && <div className="text-gray-400 mt-0.5">{log.actorRole}</div>}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-gray-500">
                      {log.ip ?? '—'}
                    </td>
                    <td className="px-4 py-3">
                      <MetadataCell metadata={log.metadata} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-4">
              <p className="text-sm text-gray-500">
                Страница {page + 1} из {totalPages}
              </p>
              <div className="flex gap-2">
                <button
                  disabled={page === 0}
                  onClick={() => setPage(p => p - 1)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50"
                >
                  Назад
                </button>
                <button
                  disabled={page >= totalPages - 1}
                  onClick={() => setPage(p => p + 1)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50"
                >
                  Вперёд
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
