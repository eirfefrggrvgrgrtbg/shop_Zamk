import { useState, useEffect, useCallback } from 'react';
import { listAuditLogs } from '@zamk/api-client/src/admin';
import { AlertCircle, ChevronDown, ChevronRight, ClipboardList } from 'lucide-react';

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

  const loadLogs = useCallback(async (pageNum: number) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await listAuditLogs(PAGE_SIZE, pageNum * PAGE_SIZE);
      setLogs(data.items ?? []);
      setTotal(data.total ?? 0);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить журнал действий');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => { loadLogs(page); }, [loadLogs, page]);

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
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <ClipboardList className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Записей нет</h3>
          <p className="mt-1 text-sm text-gray-500">Журнал действий пока пуст.</p>
        </div>
      ) : (
        <>
          <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
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
                      <span className="text-xs font-mono text-gray-800">{log.action}</span>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-gray-500">
                      {log.entityType ?? '—'}
                      {log.entityId && (
                        <div className="text-gray-400 font-mono" title={log.entityId}>
                          {log.entityId.substring(0, 8)}…
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-gray-600">
                      {log.actorEmail ?? '—'}
                      {log.actorRole && <div className="text-gray-400">{log.actorRole}</div>}
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

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-gray-500">
                Страница {page + 1} из {totalPages}
              </p>
              <div className="flex gap-2">
                <button
                  disabled={page === 0}
                  onClick={() => setPage(p => p - 1)}
                  className="px-3 py-1 border border-gray-300 rounded text-sm disabled:opacity-50 hover:bg-gray-50"
                >
                  Назад
                </button>
                <button
                  disabled={page >= totalPages - 1}
                  onClick={() => setPage(p => p + 1)}
                  className="px-3 py-1 border border-gray-300 rounded text-sm disabled:opacity-50 hover:bg-gray-50"
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
