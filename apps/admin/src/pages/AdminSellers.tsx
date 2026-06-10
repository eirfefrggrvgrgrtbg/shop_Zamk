import { useState, useEffect, useCallback } from 'react';
import {
  getAdminSellers, createAdminSeller,
  updateAdminSellerStatus, getAdminSellerDetail,
  verifyAdminSeller, getSellerStatusHistory,
  listSellerWarnings, createSellerWarning, resolveSellerWarning, cancelSellerWarning,
  listSellerViolations, createSellerViolation, resolveSellerViolation, cancelSellerViolation,
} from '@zamk/api-client/src/admin';
import type { AdminSeller, SellerDetail, SellerStatusHistoryItem, SellerWarning, SellerViolation } from '@zamk/api-client/src/types';
import { AlertCircle, Plus, CheckCircle2, Store, X, ChevronRight, AlertTriangle } from 'lucide-react';
import { PermissionGuard } from '../components/PermissionGuard';
import { useAdminAuth } from '../contexts/AdminAuthContext';

// --- Constants ---

const STATUS_LABELS: Record<string, string> = {
  pending: '�?жидае�? ак�?ива�?ии',
  active: 'Ак�?ивен',
  blocked: '�?аблоки�?ован',
  archived: '�? а�?�?иве',
};

const STATUS_BADGE: Record<string, string> = {
  active: 'bg-green-100 text-green-800',
  blocked: 'bg-red-100 text-red-800',
  pending: 'bg-yellow-100 text-yellow-800',
  archived: 'bg-gray-100 text-gray-700',
};

const WARNING_TYPES: Record<string, string> = {
  late_shipment: '�?оздняя о�?п�?авка',
  wrong_item: 'Неве�?н�?й �?ова�?',
  no_shipment: 'Не�? о�?п�?авки',
  poor_packaging: '�?ло�?ая �?паковка',
  customer_complaint: '�?алоба пок�?па�?еля',
  moderation_issue: 'На�?�?�?ение моде�?а�?ии',
  return_problem: '�?�?облема с возв�?а�?ом',
  other: '�?�?�?гое',
};

const VIOLATION_TYPES: Record<string, string> = {
  no_shipment: 'Не�? о�?п�?авки',
  late_shipment: '�?оздняя о�?п�?авка',
  wrong_item: 'Неве�?н�?й �?ова�?',
  fake_product: '�?оддел�?н�?й �?ова�?',
  damaged_item_not_disclosed: 'Ск�?�?�?�?й де�?ек�?',
  repeated_customer_complaints: '�?ов�?о�?н�?е жалоб�?',
  return_abuse: '�?ло�?по�?�?ебление возв�?а�?ами',
  moderation_violation: 'На�?�?�?ение п�?авил моде�?а�?ии',
  other: '�?�?�?гое',
};

const SEVERITY_LABELS: Record<string, string> = { low: 'Низкая', medium: 'С�?едняя', high: '�?�?сокая' };
const SEVERITY_BADGE: Record<string, string> = {
  low: 'bg-blue-100 text-blue-700',
  medium: 'bg-yellow-100 text-yellow-800',
  high: 'bg-red-100 text-red-800',
};

const WARNING_STATUS_LABELS: Record<string, string> = {
  active: 'Ак�?ивно', resolved: 'Раз�?е�?ено', cancelled: '�?�?менено',
};
const WARNING_STATUS_BADGE: Record<string, string> = {
  active: 'bg-orange-100 text-orange-800',
  resolved: 'bg-green-100 text-green-800',
  cancelled: 'bg-gray-100 text-gray-600',
};

// --- Helper Components ---

function Badge({ label, className }: { label: string; className: string }) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${className}`}>
      {label}
    </span>
  );
}

// --- Tab: �?бзо�? ---
function OverviewTab({ detail, onVerify }: { detail: SellerDetail; onVerify: () => void }) {
  const { hasPermission } = useAdminAuth();
  const policy = detail.commissionPolicy;
  const penaltyBps2 = detail.counts.activePenaltyViolations >= 2;

  return (
    <div className="space-y-5">
      {penaltyBps2 && (
        <div className="flex items-start p-3 bg-amber-50 border border-amber-200 rounded-lg text-sm text-amber-800">
          <AlertTriangle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
          <span>
            У п�?одав�?а {detail.counts.activePenaltyViolations} или более ак�?ивн�?�? на�?�?�?ений. �? б�?д�?�?ем э�?о може�? пов�?си�?�? комисси�? до 18% на 1 меся�?.
          </span>
        </div>
      )}

      <div className="grid grid-cols-2 gap-4">
        <div>
          <p className="text-xs text-gray-500 uppercase tracking-wide">С�?а�?�?с</p>
          <Badge label={STATUS_LABELS[detail.status] ?? detail.status} className={STATUS_BADGE[detail.status] ?? 'bg-gray-100'} />
        </div>
        <div>
          <p className="text-xs text-gray-500 uppercase tracking-wide">�?а�?егис�?�?и�?ован</p>
          <p className="text-sm text-gray-900">{new Date(detail.createdAt).toLocaleDateString('ru-RU')}</p>
        </div>
      </div>

      <div>
        <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">�?ладеле�?</p>
        <p className="text-sm font-medium text-gray-900">{detail.owner.name}</p>
        <p className="text-sm text-gray-500">{detail.owner.email}</p>
        <Badge label={detail.owner.status === 'active' ? 'Ак�?ивен' : detail.owner.status} className={detail.owner.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-700'} />
      </div>

      {(detail.contactEmail || detail.contactPhone) && (
        <div>
          <p className="text-xs text-gray-500 uppercase tracking-wide mb-1">�?он�?ак�?�?</p>
          {detail.contactEmail && <p className="text-sm text-gray-900">{detail.contactEmail}</p>}
          {detail.contactPhone && <p className="text-sm text-gray-900">{detail.contactPhone}</p>}
        </div>
      )}

      <div className="border rounded-lg p-4 bg-gray-50">
        <p className="text-xs font-medium text-gray-700 mb-2">�?оли�?ика комиссий</p>
        <p className="text-sm text-gray-900">�?азовая комиссия �?? {policy.baseCommissionBps / 100}%</p>
        <p className="text-sm text-gray-500 mt-1">�?�?и 2 на�?�?�?ения�? возможно пов�?�?ение до {policy.penaltyCommissionBps / 100}% на 1 меся�?</p>
        <p className="text-sm text-gray-500 mt-1">Ав�?ома�?и�?еское п�?именение �?�?�?а�?ной комиссии пока не вкл�?�?ено</p>
      </div>

      <div className="grid grid-cols-3 gap-3 text-center">
        <div className="border rounded p-3">
          <p className="text-xl font-bold text-orange-600">{detail.counts.warningsActive}</p>
          <p className="text-xs text-gray-500">�?�?ед�?п�?еждений</p>
        </div>
        <div className="border rounded p-3">
          <p className="text-xl font-bold text-red-600">{detail.counts.violationsActive}</p>
          <p className="text-xs text-gray-500">На�?�?�?ений</p>
        </div>
        <div className="border rounded p-3">
          <p className="text-xl font-bold text-purple-600">{detail.counts.activePenaltyViolations}</p>
          <p className="text-xs text-gray-500">Ш�?�?а�?н�?�?</p>
        </div>
      </div>

      {detail.status === 'pending' && hasPermission('sellers.verify') && (
        <button
          onClick={onVerify}
          className="w-full px-4 py-2 bg-green-600 text-white rounded-md text-sm font-medium hover:bg-green-700"
        >
          �?�?ове�?и�?�? и ак�?иви�?ова�?�?
        </button>
      )}
    </div>
  );
}

// --- Tab: �?�?о�?ил�? ---
function ProfileTab({ detail }: { detail: SellerDetail }) {
  const missing: string[] = [];
  if (!detail.brandName) missing.push('Название магазина');
  if (!detail.slug) missing.push('URL-слаг');
  if (!detail.description || detail.description.length < 10) missing.push('�?писание (миним�?м 10 символов)');
  if (!detail.contactEmail && !detail.contactPhone) missing.push('�?он�?ак�?ная по�?�?а или �?еле�?он');

  return (
    <div className="space-y-4">
      {missing.length > 0 && (
        <div className="p-3 bg-yellow-50 border border-yellow-200 rounded text-sm text-yellow-800">
          <p className="font-medium mb-1">Незаполненн�?е поля:</p>
          <ul className="list-disc list-inside">
            {missing.map(f => <li key={f}>{f}</li>)}
          </ul>
        </div>
      )}
      {detail.logoUrl && (
        <img src={detail.logoUrl} alt="�?ого�?ип" className="h-16 w-16 rounded object-cover border" />
      )}
      <Field label="Название" value={detail.brandName} />
      <Field label="Слаг" value={detail.slug ? `/${detail.slug}` : undefined} />
      <Field label="�?писание" value={detail.description} />
      <Field label="�?он�?ак�?ная по�?�?а" value={detail.contactEmail} />
      <Field label="Теле�?он" value={detail.contactPhone} />
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string | null }) {
  return (
    <div>
      <p className="text-xs text-gray-500 uppercase tracking-wide">{label}</p>
      <p className="text-sm text-gray-900">{value || <span className="text-gray-400 italic">Не заполнено</span>}</p>
    </div>
  );
}

// --- Tab: С�?а�?�?с�? ---
function StatusTab({
  detail, history, onStatusUpdate, onVerify, verifyError,
}: {
  detail: SellerDetail;
  history: SellerStatusHistoryItem[];
  onStatusUpdate: (status: string, reason?: string) => Promise<void>;
  onVerify: () => Promise<void>;
  verifyError: string | null;
}) {
  const { hasPermission } = useAdminAuth();
  const [newStatus, setNewStatus] = useState(detail.status);
  const [reason, setReason] = useState('');
  const [isUpdating, setIsUpdating] = useState(false);
  const [updateError, setUpdateError] = useState<string | null>(null);

  const needsReason = newStatus === 'blocked' || newStatus === 'archived';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsUpdating(true);
    setUpdateError(null);
    try {
      await onStatusUpdate(newStatus, reason || undefined);
      setReason('');
    } catch (err: any) {
      setUpdateError(err.message || '�?�?ибка обновления с�?а�?�?са');
    } finally {
      setIsUpdating(false);
    }
  };

  return (
    <div className="space-y-5">
      {verifyError && (
        <div className="p-3 bg-red-50 border border-red-200 rounded text-sm text-red-800">
          {verifyError}
        </div>
      )}

      {hasPermission('sellers.update_status') && (
        <form onSubmit={handleSubmit} className="space-y-3 border rounded-lg p-4">
          <p className="text-sm font-medium text-gray-700">�?змени�?�? с�?а�?�?с</p>
          {updateError && <p className="text-xs text-red-600">{updateError}</p>}
          <select
            value={newStatus}
            onChange={e => setNewStatus(e.target.value)}
            className="block w-full px-3 py-2 border border-gray-300 rounded-md text-sm focus:ring-indigo-500 focus:border-indigo-500"
          >
            <option value="pending">�?жидае�? ак�?ива�?ии</option>
            <option value="active">Ак�?ивен</option>
            <option value="blocked">�?аблоки�?ован</option>
            <option value="archived">�? а�?�?иве</option>
          </select>
          {needsReason && (
            <div>
              <label className="block text-xs text-gray-600 mb-1">�?�?и�?ина (обяза�?ел�?но)</label>
              <input
                required
                type="text"
                value={reason}
                onChange={e => setReason(e.target.value)}
                placeholder="Укажи�?е п�?и�?ин�?"
                className="block w-full px-3 py-2 border border-gray-300 rounded-md text-sm"
              />
            </div>
          )}
          <button
            type="submit"
            disabled={isUpdating}
            className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
          >
            {isUpdating ? 'Со�?�?анение...' : 'Со�?�?ани�?�? с�?а�?�?с'}
          </button>
        </form>
      )}

      {detail.status === 'pending' && hasPermission('sellers.verify') && (
        <button
          onClick={onVerify}
          className="w-full px-4 py-2 bg-green-600 text-white rounded-md text-sm font-medium hover:bg-green-700"
        >
          �?�?ове�?и�?�? и ак�?иви�?ова�?�?
        </button>
      )}

      <div>
        <p className="text-sm font-medium text-gray-700 mb-2">�?с�?о�?ия с�?а�?�?сов</p>
        {history.length === 0 ? (
          <p className="text-sm text-gray-400">�?с�?о�?ия п�?с�?а</p>
        ) : (
          <div className="space-y-2">
            {history.map(item => (
              <div key={item.id} className="border-l-2 border-gray-200 pl-3 py-1">
                <div className="flex items-center gap-2 text-sm">
                  {item.oldStatus && (
                    <>
                      <Badge label={STATUS_LABELS[item.oldStatus] ?? item.oldStatus} className={STATUS_BADGE[item.oldStatus] ?? 'bg-gray-100'} />
                      <ChevronRight className="h-3 w-3 text-gray-400" />
                    </>
                  )}
                  <Badge label={STATUS_LABELS[item.newStatus] ?? item.newStatus} className={STATUS_BADGE[item.newStatus] ?? 'bg-gray-100'} />
                </div>
                {item.reason && <p className="text-xs text-gray-500 mt-1">�?�?и�?ина: {item.reason}</p>}
                <p className="text-xs text-gray-400">{new Date(item.createdAt).toLocaleString('ru-RU')}</p>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// --- Tab: �?�?ед�?п�?еждения ---
function WarningsTab({ sellerId, warnings, onRefresh }: {
  sellerId: string;
  warnings: SellerWarning[];
  onRefresh: () => void;
}) {
  const { hasPermission } = useAdminAuth();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [resolveId, setResolveId] = useState<string | null>(null);
  const [resolveNote, setResolveNote] = useState('');
  const [isWorking, setIsWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [wType, setWType] = useState('other');
  const [wTitle, setWTitle] = useState('');
  const [wMessage, setWMessage] = useState('');
  const [wSeverity, setWSeverity] = useState<'low' | 'medium' | 'high'>('medium');

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsWorking(true);
    setError(null);
    try {
      await createSellerWarning(sellerId, { type: wType, title: wTitle, message: wMessage, severity: wSeverity });
      setIsCreateOpen(false);
      setWTitle(''); setWMessage('');
      onRefresh();
    } catch (err: any) {
      setError(err.message || '�?�?ибка');
    } finally {
      setIsWorking(false);
    }
  };

  const handleResolve = async () => {
    if (!resolveId) return;
    setIsWorking(true);
    try {
      await resolveSellerWarning(sellerId, resolveId, resolveNote || undefined);
      setResolveId(null); setResolveNote('');
      onRefresh();
    } catch (err: any) {
      setError(err.message || '�?�?ибка');
    } finally {
      setIsWorking(false);
    }
  };

  const handleCancel = async (warningId: string) => {
    if (!window.confirm('�?�?мени�?�? п�?ед�?п�?еждение?')) return;
    try {
      await cancelSellerWarning(sellerId, warningId);
      onRefresh();
    } catch (err: any) {
      alert(err.message || '�?�?ибка');
    }
  };

  return (
    <div className="space-y-4">
      {error && <div className="p-2 bg-red-50 text-red-700 text-sm rounded">{error}</div>}

      {hasPermission('sellers.warn') && (
        <button onClick={() => setIsCreateOpen(true)}
          className="inline-flex items-center px-3 py-1.5 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700">
          <Plus className="-ml-0.5 mr-1.5 h-4 w-4" /> Созда�?�? п�?ед�?п�?еждение
        </button>
      )}

      {warnings.length === 0 ? (
        <p className="text-sm text-gray-400">�?�?ед�?п�?еждений не�?</p>
      ) : (
        <div className="space-y-2">
          {warnings.map(w => (
            <div key={w.id} className="border rounded-lg p-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-gray-900">{w.title}</span>
                  <Badge label={SEVERITY_LABELS[w.severity] ?? w.severity} className={SEVERITY_BADGE[w.severity] ?? 'bg-gray-100'} />
                  <Badge label={WARNING_STATUS_LABELS[w.status] ?? w.status} className={WARNING_STATUS_BADGE[w.status] ?? 'bg-gray-100'} />
                </div>
                {w.status === 'active' && hasPermission('sellers.warn') && (
                  <div className="flex gap-2 text-xs">
                    <button onClick={() => setResolveId(w.id)} className="text-green-600 hover:text-green-800">Раз�?е�?и�?�?</button>
                    <button onClick={() => handleCancel(w.id)} className="text-gray-500 hover:text-gray-700">�?�?мени�?�?</button>
                  </div>
                )}
              </div>
              <p className="text-xs text-gray-500 mt-1">{WARNING_TYPES[w.type] ?? w.type} · {new Date(w.createdAt).toLocaleDateString('ru-RU')}</p>
              <p className="text-sm text-gray-700 mt-1">{w.message}</p>
            </div>
          ))}
        </div>
      )}

      {isCreateOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-5 w-full max-w-md">
            <h3 className="text-lg font-bold mb-4">Созда�?�? п�?ед�?п�?еждение</h3>
            <form onSubmit={handleCreate} className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-gray-700">Тип</label>
                <select value={wType} onChange={e => setWType(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm">
                  {Object.entries(WARNING_TYPES).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">�?аголовок</label>
                <input required type="text" value={wTitle} onChange={e => setWTitle(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Сооб�?ение</label>
                <textarea required value={wMessage} onChange={e => setWMessage(e.target.value)} rows={3}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Се�?�?�?знос�?�?</label>
                <select value={wSeverity} onChange={e => setWSeverity(e.target.value as any)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm">
                  <option value="low">Низкая</option>
                  <option value="medium">С�?едняя</option>
                  <option value="high">�?�?сокая</option>
                </select>
              </div>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setIsCreateOpen(false)}
                  className="px-3 py-2 border border-gray-300 rounded-md text-sm">�?�?мена</button>
                <button type="submit" disabled={isWorking}
                  className="px-3 py-2 bg-indigo-600 text-white rounded-md text-sm disabled:opacity-50">
                  {isWorking ? 'Создание...' : 'Созда�?�?'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {resolveId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-5 w-full max-w-md">
            <h3 className="text-lg font-bold mb-4">Раз�?е�?и�?�? п�?ед�?п�?еждение</h3>
            <div>
              <label className="block text-sm font-medium text-gray-700">�?�?име�?ание (необяза�?ел�?но)</label>
              <textarea value={resolveNote} onChange={e => setResolveNote(e.target.value)} rows={2}
                className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm" />
            </div>
            <div className="flex justify-end gap-2 mt-3">
              <button onClick={() => setResolveId(null)}
                className="px-3 py-2 border border-gray-300 rounded-md text-sm">�?�?мена</button>
              <button onClick={handleResolve} disabled={isWorking}
                className="px-3 py-2 bg-green-600 text-white rounded-md text-sm disabled:opacity-50">
                {isWorking ? 'Раз�?е�?ение...' : '�?од�?ве�?ди�?�?'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// --- Tab: На�?�?�?ения ---
function ViolationsTab({ sellerId, violations, activePenaltyViolations, onRefresh }: {
  sellerId: string;
  violations: SellerViolation[];
  activePenaltyViolations: number;
  onRefresh: () => void;
}) {
  const { hasPermission } = useAdminAuth();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [resolveId, setResolveId] = useState<string | null>(null);
  const [resolveNote, setResolveNote] = useState('');
  const [isWorking, setIsWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [vType, setVType] = useState('other');
  const [vTitle, setVTitle] = useState('');
  const [vDescription, setVDescription] = useState('');
  const [vSeverity, setVSeverity] = useState<'low' | 'medium' | 'high'>('medium');
  const [vCounts, setVCounts] = useState(true);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsWorking(true);
    setError(null);
    try {
      await createSellerViolation(sellerId, {
        type: vType, title: vTitle, description: vDescription,
        severity: vSeverity, countsForPenalty: vCounts,
      });
      setIsCreateOpen(false);
      setVTitle(''); setVDescription('');
      onRefresh();
    } catch (err: any) {
      setError(err.message || '�?�?ибка');
    } finally {
      setIsWorking(false);
    }
  };

  const handleResolve = async () => {
    if (!resolveId) return;
    setIsWorking(true);
    try {
      await resolveSellerViolation(sellerId, resolveId, resolveNote || undefined);
      setResolveId(null); setResolveNote('');
      onRefresh();
    } catch (err: any) {
      setError(err.message || '�?�?ибка');
    } finally {
      setIsWorking(false);
    }
  };

  const handleCancel = async (violationId: string) => {
    if (!window.confirm('�?�?мени�?�? на�?�?�?ение?')) return;
    try {
      await cancelSellerViolation(sellerId, violationId);
      onRefresh();
    } catch (err: any) {
      alert(err.message || '�?�?ибка');
    }
  };

  return (
    <div className="space-y-4">
      {activePenaltyViolations >= 2 && (
        <div className="flex items-start p-3 bg-amber-50 border border-amber-200 rounded text-sm text-amber-800">
          <AlertTriangle className="h-4 w-4 mr-2 shrink-0 mt-0.5" />
          У п�?одав�?а {activePenaltyViolations} ак�?ивн�?�? �?�?�?а�?н�?�? на�?�?�?ений. �? б�?д�?�?ем э�?о може�? пов�?си�?�? комисси�? до 18%.
        </div>
      )}

      {error && <div className="p-2 bg-red-50 text-red-700 text-sm rounded">{error}</div>}

      {hasPermission('sellers.warn') && (
        <button onClick={() => setIsCreateOpen(true)}
          className="inline-flex items-center px-3 py-1.5 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-red-600 hover:bg-red-700">
          <Plus className="-ml-0.5 mr-1.5 h-4 w-4" /> Созда�?�? на�?�?�?ение
        </button>
      )}

      {violations.length === 0 ? (
        <p className="text-sm text-gray-400">На�?�?�?ений не�?</p>
      ) : (
        <div className="space-y-2">
          {violations.map(v => (
            <div key={v.id} className="border rounded-lg p-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium text-gray-900">{v.title}</span>
                  <Badge label={SEVERITY_LABELS[v.severity] ?? v.severity} className={SEVERITY_BADGE[v.severity] ?? 'bg-gray-100'} />
                  <Badge label={WARNING_STATUS_LABELS[v.status] ?? v.status} className={WARNING_STATUS_BADGE[v.status] ?? 'bg-gray-100'} />
                  {v.countsForPenalty && <Badge label="Ш�?�?а�?ное" className="bg-purple-100 text-purple-800" />}
                </div>
                {v.status === 'active' && hasPermission('sellers.warn') && (
                  <div className="flex gap-2 text-xs">
                    <button onClick={() => setResolveId(v.id)} className="text-green-600 hover:text-green-800">Раз�?е�?и�?�?</button>
                    <button onClick={() => handleCancel(v.id)} className="text-gray-500 hover:text-gray-700">�?�?мени�?�?</button>
                  </div>
                )}
              </div>
              <p className="text-xs text-gray-500 mt-1">{VIOLATION_TYPES[v.type] ?? v.type} · {new Date(v.createdAt).toLocaleDateString('ru-RU')}</p>
              <p className="text-sm text-gray-700 mt-1">{v.description}</p>
            </div>
          ))}
        </div>
      )}

      {isCreateOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-5 w-full max-w-md">
            <h3 className="text-lg font-bold mb-4">Созда�?�? на�?�?�?ение</h3>
            <form onSubmit={handleCreate} className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-gray-700">Тип</label>
                <select value={vType} onChange={e => setVType(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm">
                  {Object.entries(VIOLATION_TYPES).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">�?аголовок</label>
                <input required type="text" value={vTitle} onChange={e => setVTitle(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">�?писание</label>
                <textarea required value={vDescription} onChange={e => setVDescription(e.target.value)} rows={3}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Се�?�?�?знос�?�?</label>
                <select value={vSeverity} onChange={e => setVSeverity(e.target.value as any)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm">
                  <option value="low">Низкая</option>
                  <option value="medium">С�?едняя</option>
                  <option value="high">�?�?сокая</option>
                </select>
              </div>
              <div className="flex items-center gap-2">
                <input type="checkbox" id="vCounts" checked={vCounts} onChange={e => setVCounts(e.target.checked)}
                  className="rounded border-gray-300" />
                <label htmlFor="vCounts" className="text-sm text-gray-700">
                  У�?и�?�?вае�?ся п�?и �?ас�?�?�?е �?�?�?а�?ной комиссии
                </label>
              </div>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setIsCreateOpen(false)}
                  className="px-3 py-2 border border-gray-300 rounded-md text-sm">�?�?мена</button>
                <button type="submit" disabled={isWorking}
                  className="px-3 py-2 bg-red-600 text-white rounded-md text-sm disabled:opacity-50">
                  {isWorking ? 'Создание...' : 'Созда�?�?'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {resolveId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-5 w-full max-w-md">
            <h3 className="text-lg font-bold mb-4">Раз�?е�?и�?�? на�?�?�?ение</h3>
            <div>
              <label className="block text-sm font-medium text-gray-700">�?�?име�?ание (необяза�?ел�?но)</label>
              <textarea value={resolveNote} onChange={e => setResolveNote(e.target.value)} rows={2}
                className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md text-sm" />
            </div>
            <div className="flex justify-end gap-2 mt-3">
              <button onClick={() => setResolveId(null)}
                className="px-3 py-2 border border-gray-300 rounded-md text-sm">�?�?мена</button>
              <button onClick={handleResolve} disabled={isWorking}
                className="px-3 py-2 bg-green-600 text-white rounded-md text-sm disabled:opacity-50">
                {isWorking ? 'Раз�?е�?ение...' : '�?од�?ве�?ди�?�?'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// --- Seller Detail Drawer ---
type DrawerTab = 'overview' | 'profile' | 'status' | 'warnings' | 'violations';

function SellerDetailDrawer({ sellerId, onClose, onRefreshList }: {
  sellerId: string;
  onClose: () => void;
  onRefreshList: () => void;
}) {
  const [tab, setTab] = useState<DrawerTab>('overview');
  const [detail, setDetail] = useState<SellerDetail | null>(null);
  const [history, setHistory] = useState<SellerStatusHistoryItem[]>([]);
  const [warnings, setWarnings] = useState<SellerWarning[]>([]);
  const [violations, setViolations] = useState<SellerViolation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [verifyError, setVerifyError] = useState<string | null>(null);

  const loadDetail = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const [detailData, histData, warnData, vioData] = await Promise.all([
        getAdminSellerDetail(sellerId),
        getSellerStatusHistory(sellerId),
        listSellerWarnings(sellerId),
        listSellerViolations(sellerId),
      ]);
      setDetail(detailData);
      setHistory(histData.items ?? []);
      setWarnings(warnData.items ?? []);
      setViolations(vioData.items ?? []);
    } catch (err: any) {
      setError(err.message || 'Не �?далос�? заг�?�?зи�?�? данн�?е п�?одав�?а');
    } finally {
      setIsLoading(false);
    }
  }, [sellerId]);

  useEffect(() => { loadDetail(); }, [loadDetail]);

  const handleStatusUpdate = async (status: string, reason?: string) => {
    await updateAdminSellerStatus(sellerId, status, reason);
    onRefreshList();
    await loadDetail();
  };

  const handleVerify = async () => {
    setVerifyError(null);
    try {
      await verifyAdminSeller(sellerId);
      onRefreshList();
      await loadDetail();
    } catch (err: any) {
      const body = err.responseBody || {};
      if (body.missingFields) {
        setVerifyError(`�?�?о�?ил�? не заполнен. Не �?ва�?ае�?: ${body.missingFields.join(', ')}`);
      } else {
        setVerifyError(err.message || '�?�?ибка ве�?и�?ика�?ии');
      }
      setTab('status');
    }
  };

  const TABS: { id: DrawerTab; label: string }[] = [
    { id: 'overview', label: '�?бзо�?' },
    { id: 'profile', label: '�?�?о�?ил�?' },
    { id: 'status', label: 'С�?а�?�?с�?' },
    { id: 'warnings', label: '�?�?ед�?п�?еждения' },
    { id: 'violations', label: 'На�?�?�?ения' },
  ];

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="fixed inset-0 bg-black bg-opacity-30" onClick={onClose} />
      <div className="relative bg-white w-full max-w-xl shadow-xl flex flex-col h-full overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b">
          <h2 className="text-lg font-bold text-gray-900">
            {detail ? detail.brandName : '�?аг�?�?зка...'}
          </h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex border-b overflow-x-auto">
          {TABS.map(t => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`px-4 py-2.5 text-sm font-medium whitespace-nowrap ${
                tab === t.id
                  ? 'border-b-2 border-indigo-600 text-indigo-600'
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              {t.label}
              {t.id === 'warnings' && warnings.filter(w => w.status === 'active').length > 0 && (
                <span className="ml-1 inline-flex items-center justify-center h-4 w-4 rounded-full bg-orange-500 text-white text-xs">
                  {warnings.filter(w => w.status === 'active').length}
                </span>
              )}
              {t.id === 'violations' && violations.filter(v => v.status === 'active').length > 0 && (
                <span className="ml-1 inline-flex items-center justify-center h-4 w-4 rounded-full bg-red-500 text-white text-xs">
                  {violations.filter(v => v.status === 'active').length}
                </span>
              )}
            </button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto p-5">
          {isLoading ? (
            <div className="flex justify-center py-10">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600" />
            </div>
          ) : error ? (
            <div className="p-4 bg-red-50 text-red-700 rounded flex items-center">
              <AlertCircle className="h-5 w-5 mr-2" />
              {error}
            </div>
          ) : detail ? (
            <>
              {tab === 'overview' && <OverviewTab detail={detail} onVerify={handleVerify} />}
              {tab === 'profile' && <ProfileTab detail={detail} />}
              {tab === 'status' && (
                <StatusTab
                  detail={detail}
                  history={history}
                  onStatusUpdate={handleStatusUpdate}
                  onVerify={handleVerify}
                  verifyError={verifyError}
                />
              )}
              {tab === 'warnings' && (
                <WarningsTab sellerId={sellerId} warnings={warnings} onRefresh={loadDetail} />
              )}
              {tab === 'violations' && (
                <ViolationsTab
                  sellerId={sellerId}
                  violations={violations}
                  activePenaltyViolations={detail.counts.activePenaltyViolations}
                  onRefresh={loadDetail}
                />
              )}
            </>
          ) : null}
        </div>
      </div>
    </div>
  );
}

// --- Main AdminSellers Page ---
export function AdminSellers() {
  const [sellers, setSellers] = useState<AdminSeller[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedSellerId, setSelectedSellerId] = useState<string | null>(null);

  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [generatedPassword, setGeneratedPassword] = useState<string | null>(null);

  const [brandName, setBrandName] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [ownerName, setOwnerName] = useState('');
  const [ownerEmail, setOwnerEmail] = useState('');
  const [password, setPassword] = useState('');

  const fetchSellers = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminSellers();
      setSellers(data.items ?? []);
    } catch (err: any) {
      setError(err.message || 'Не �?далос�? заг�?�?зи�?�? п�?одав�?ов');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => { fetchSellers(); }, [fetchSellers]);

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsCreating(true);
    setCreateError(null);
    try {
      const localPassword = password;
      await createAdminSeller({ brandName, contactEmail, ownerName, ownerEmail, temporaryPassword: localPassword });
      setGeneratedPassword(localPassword);
      setBrandName(''); setContactEmail(''); setOwnerName(''); setOwnerEmail(''); setPassword('');
      fetchSellers();
    } catch (err: any) {
      setCreateError(err.message || 'Не �?далос�? созда�?�? п�?одав�?а');
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">�?�?одав�?�?</h1>
          <p className="mt-1 text-sm text-gray-500">Уп�?авление дос�?�?пом п�?одав�?ов на пла�?�?о�?ме</p>
        </div>
        <PermissionGuard permission="sellers.create_access">
          <button
            onClick={() => setIsCreateModalOpen(true)}
            className="mt-3 sm:mt-0 inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700"
          >
            <Plus className="-ml-1 mr-2 h-5 w-5" />
            Созда�?�? дос�?�?п п�?одав�?а
          </button>
        </PermissionGuard>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2" />{error}
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-2 text-sm text-gray-500">�?аг�?�?зка п�?одав�?ов...</p>
        </div>
      ) : sellers.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <Store className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">�?�?одав�?ов не�?</h3>
          <p className="mt-1 text-sm text-gray-500">Создай�?е пе�?в�?й дос�?�?п п�?одав�?а.</p>
        </div>
      ) : (
        <div className="shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Название / Слаг</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">С�?а�?�?с</th>
                <th className="relative px-6 py-3"><span className="sr-only">�?ейс�?вия</span></th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {sellers.map((seller) => (
                <tr key={seller.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="text-sm font-medium text-gray-900">{seller.brandName}</div>
                    <div className="text-xs text-gray-500">/{seller.slug}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <Badge
                      label={STATUS_LABELS[seller.status] ?? seller.status}
                      className={STATUS_BADGE[seller.status] ?? 'bg-gray-100 text-gray-700'}
                    />
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button
                      onClick={() => setSelectedSellerId(seller.id)}
                      className="text-indigo-600 hover:text-indigo-900 font-medium"
                    >
                      �?�?к�?�?�?�?
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedSellerId && (
        <SellerDetailDrawer
          sellerId={selectedSellerId}
          onClose={() => setSelectedSellerId(null)}
          onRefreshList={fetchSellers}
        />
      )}

      {isCreateModalOpen && !generatedPassword && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-1">Созда�?�? дос�?�?п п�?одав�?а</h2>
            <p className="text-sm text-gray-500 mb-4">
              Админис�?�?а�?о�? созда�?�? �?ол�?ко дос�?�?п. �?�?одаве�? самос�?оя�?ел�?но заполни�? п�?о�?ил�? магазина после пе�?вого в�?ода.
            </p>
            {createError && (
              <div className="mb-4 p-3 bg-red-50 text-red-700 text-sm rounded flex items-start">
                <AlertCircle className="h-5 w-5 mr-2 shrink-0 mt-0.5" />
                <span>{createError}</span>
              </div>
            )}
            <form onSubmit={handleCreateSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">�?мя владел�?�?а</label>
                <input required type="text" value={ownerName} onChange={e => setOwnerName(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Email владел�?�?а</label>
                <input required type="email" value={ownerEmail} onChange={e => setOwnerEmail(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Название магазина</label>
                <input required type="text" value={brandName} onChange={e => setBrandName(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">�?он�?ак�?ная по�?�?а</label>
                <input type="email" value={contactEmail} onChange={e => setContactEmail(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">�?�?еменн�?й па�?ол�?</label>
                <input required type="text" minLength={8} value={password} onChange={e => setPassword(e.target.value)}
                  placeholder="�?иним�?м 8 символов"
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm text-sm" />
              </div>
              <div className="mt-5 flex justify-end space-x-3">
                <button type="button" onClick={() => setIsCreateModalOpen(false)}
                  className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">
                  �?�?мена
                </button>
                <button type="submit" disabled={isCreating}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700 disabled:opacity-50">
                  {isCreating ? 'Создание...' : 'Созда�?�? дос�?�?п'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {generatedPassword && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <div className="flex items-center text-green-600 mb-4">
              <CheckCircle2 className="h-8 w-8 mr-2" />
              <h2 className="text-xl font-bold">�?�?одаве�? создан</h2>
            </div>
            <p className="text-sm text-gray-600 mb-2">�?�?еменн�?й па�?ол�? создан. Со�?�?ани�?е его �?? он бол�?�?е не б�?де�? показан.</p>
            <p className="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2 mb-4">
              �?е�?едай�?е па�?ол�? п�?одав�?�? над�?жн�?м способом. �?�?и пе�?вом в�?оде п�?одаве�? б�?де�? обязан смени�?�? па�?ол�?.
            </p>
            <div className="bg-gray-100 p-4 rounded text-center mb-6 border border-gray-200">
              <code className="text-lg font-mono font-bold text-gray-900 select-all">{generatedPassword}</code>
            </div>
            <button onClick={() => { setGeneratedPassword(null); setIsCreateModalOpen(false); }}
              className="w-full px-4 py-2 bg-indigo-600 text-white rounded-md text-sm font-medium hover:bg-indigo-700">
              �?а�?ол�? скопи�?ован, зак�?�?�?�?
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
