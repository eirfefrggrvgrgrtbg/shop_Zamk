import { useRef, useEffect, Fragment, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import {
  Lock,
  Download,
  FileText,
  Package,
  RotateCcw,
  ShoppingBag,
  ExternalLink,
  SlidersHorizontal,
} from 'lucide-react';
import type {
  SupportConversation,
  SupportMessage,
  SupportInternalNote,
  SupportAttachment,
  SupportContextLink,
} from '../../api/adminSupport';
import { getAdminSupportAttachmentUrl } from '../../api/adminSupport';

interface SupportConversationViewProps {
  conversation: SupportConversation;
  messages: SupportMessage[];
  internalNotes?: SupportInternalNote[];
  isContextPanelOpen: boolean;
  onToggleContextPanel: () => void;
}

type TimelineItem =
  | { type: 'message'; data: SupportMessage; createdAt: string }
  | { type: 'internal_note'; data: SupportInternalNote; createdAt: string };

export function SupportConversationView({
  conversation,
  messages,
  internalNotes = [],
  isContextPanelOpen,
  onToggleContextPanel,
}: SupportConversationViewProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, internalNotes]);

  // Combine and sort chronologically
  const timeline: TimelineItem[] = [
    ...messages.map((m) => ({ type: 'message' as const, data: m, createdAt: m.createdAt })),
    ...internalNotes.map((n) => ({
      type: 'internal_note' as const,
      data: n,
      createdAt: n.createdAt,
    })),
  ].sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());

  const formatDate = (isoString: string) => {
    try {
      return new Date(isoString).toLocaleDateString('ru-RU', {
        day: 'numeric',
        month: 'long',
      });
    } catch {
      return '';
    }
  };

  const formatTime = (isoString: string) => {
    try {
      return new Date(isoString).toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return '';
    }
  };

  const formatBytes = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} Б`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} КБ`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`;
  };

  const isCustomer = conversation.requesterType === 'CUSTOMER';
  const headerTitle = isCustomer
    ? conversation.requesterName || conversation.requesterEmail || 'Покупатель'
    : conversation.requesterStoreName || 'Магазин продавца';

  const renderContextChip = (link: SupportContextLink) => {
    const label = link.label || link.contextId.slice(0, 8);

    if (link.contextType === 'ORDER') {
      return (
        <Link
          key={link.id}
          to={`/orders/${link.contextId}`}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-blue-50 text-blue-700 hover:bg-blue-100 transition-colors border border-blue-200/60"
        >
          <Package className="w-3.5 h-3.5 text-blue-600" />
          <span>Заказ {label}</span>
          <ExternalLink className="w-3 h-3 text-blue-400 ml-0.5" />
        </Link>
      );
    }
    if (link.contextType === 'RETURN') {
      return (
        <Link
          key={link.id}
          to={`/returns?id=${link.contextId}`}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-purple-50 text-purple-700 hover:bg-purple-100 transition-colors border border-purple-200/60"
        >
          <RotateCcw className="w-3.5 h-3.5 text-purple-600" />
          <span>Возврат {label}</span>
          <ExternalLink className="w-3 h-3 text-purple-400 ml-0.5" />
        </Link>
      );
    }
    if (link.contextType === 'PRODUCT') {
      return (
        <Link
          key={link.id}
          to={`/products/${link.contextId}`}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-emerald-50 text-emerald-700 hover:bg-emerald-100 transition-colors border border-emerald-200/60"
        >
          <ShoppingBag className="w-3.5 h-3.5 text-emerald-600" />
          <span>Товар {label}</span>
          <ExternalLink className="w-3 h-3 text-emerald-400 ml-0.5" />
        </Link>
      );
    }
    return null;
  };

  const renderAttachment = (att: SupportAttachment) => {
    const isImage = att.contentType.startsWith('image/');
    const url = getAdminSupportAttachmentUrl(att.id);

    if (isImage) {
      return (
        <a
          key={att.id}
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          data-testid={`attachment-image-${att.id}`}
          className="block group relative rounded-lg overflow-hidden border border-gray-200 max-w-xs hover:opacity-95 transition-opacity"
        >
          <img
            src={url}
            alt={att.originalFilename || 'Вложение'}
            className="w-full h-auto max-h-48 object-cover bg-gray-50"
          />
          <div className="p-1.5 bg-black/60 text-white text-[11px] truncate flex items-center justify-between">
            <span className="truncate">{att.originalFilename || 'Изображение'}</span>
            <span className="ml-2 text-gray-300 flex-shrink-0">{formatBytes(att.sizeBytes)}</span>
          </div>
        </a>
      );
    }

    return (
      <a
        key={att.id}
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        download={att.originalFilename || 'file'}
        data-testid={`attachment-file-${att.id}`}
        className="flex items-center gap-2.5 px-3 py-2 rounded-lg bg-gray-50 hover:bg-gray-100 text-gray-800 border border-gray-200 text-xs font-medium transition-colors max-w-sm"
      >
        <FileText className="w-4 h-4 text-gray-500 flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <p className="truncate text-gray-900 font-medium">{att.originalFilename || 'Документ'}</p>
          <p className="text-[10px] text-gray-400">{formatBytes(att.sizeBytes)}</p>
        </div>
        <Download className="w-3.5 h-3.5 text-gray-400 group-hover:text-gray-600 flex-shrink-0" />
      </a>
    );
  };

  return (
    <div className="flex flex-col h-full bg-slate-50 flex-1 min-w-0">
      {/* Conversation Top Header */}
      <div className="px-6 py-3.5 bg-white border-b border-gray-200 flex items-center justify-between shadow-sm z-10">
        <div className="flex items-center gap-3 min-w-0">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="text-base font-bold text-gray-900 truncate">{headerTitle}</h2>
              <span
                className={`text-[11px] font-semibold px-2 py-0.5 rounded ${
                  isCustomer
                    ? 'bg-gray-100 text-gray-700'
                    : 'bg-amber-100 text-amber-800'
                }`}
              >
                {isCustomer ? 'Покупатель' : 'Продавец'}
              </span>
            </div>
            {isCustomer && conversation.requesterEmail && (
              <p className="text-xs text-gray-500 truncate">{conversation.requesterEmail}</p>
            )}
          </div>
        </div>

        {/* Toggle Context Panel Button */}
        <button
          onClick={onToggleContextPanel}
          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
            isContextPanelOpen
              ? 'bg-gray-100 border-gray-300 text-gray-900'
              : 'bg-white border-gray-200 text-gray-600 hover:bg-gray-50'
          }`}
          title="Панель контекста и управления"
        >
          <SlidersHorizontal className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">Панель деталей</span>
        </button>
      </div>

      {/* Messages Stream */}
      <div className="flex-1 overflow-y-auto p-6 space-y-4">
        {timeline.length === 0 ? (
          <div className="flex items-center justify-center h-48 text-gray-400 text-sm">
            История диалога пуста
          </div>
        ) : (
          timeline.map((item, index) => {
            const prevItem = index > 0 ? timeline[index - 1] : null;

            // Session boundary detection
            let boundaryNotice: ReactNode = null;
            if (
              prevItem &&
              prevItem.data.sessionId &&
              item.data.sessionId &&
              prevItem.data.sessionId !== item.data.sessionId
            ) {
              boundaryNotice = (
                <div className="flex items-center my-6 gap-3 select-none">
                  <div className="h-px bg-gray-200 flex-1" />
                  <span className="text-xs text-gray-400 font-medium px-2 py-0.5 bg-gray-100 rounded">
                    Новый диалог · {formatDate(item.createdAt)}
                  </span>
                  <div className="h-px bg-gray-200 flex-1" />
                </div>
              );
            }

            if (item.type === 'internal_note') {
              const note = item.data;
              return (
                <Fragment key={`note-${note.id}`}>
                  {boundaryNotice}
                  <div
                    data-testid={`internal-note-${note.id}`}
                    className="mx-auto my-3 max-w-2xl bg-amber-50/80 border border-amber-300/80 rounded-xl p-3.5 shadow-sm"
                  >
                    <div className="flex items-center justify-between mb-1.5 pb-1 border-b border-amber-200/60">
                      <div className="flex items-center gap-1.5 text-amber-900 font-semibold text-xs">
                        <Lock className="w-3.5 h-3.5 text-amber-700" />
                        <span>Внутренняя заметка (видна только сотрудникам)</span>
                      </div>
                      <span className="text-[11px] text-amber-700/80">
                        {note.authorName || 'Сотрудник'} · {formatTime(note.createdAt)}
                      </span>
                    </div>
                    <p className="text-sm text-amber-950 whitespace-pre-wrap leading-relaxed font-normal">
                      {note.textContent}
                    </p>
                  </div>
                </Fragment>
              );
            }

            // External message
            const msg = item.data;
            const isStaff = msg.senderType === 'STAFF';
            const isSellerSender = msg.senderType === 'SELLER';

            return (
              <Fragment key={`msg-${msg.id}`}>
                {boundaryNotice}
                <div
                  data-testid={`message-${msg.id}`}
                  className={`flex flex-col ${isStaff ? 'items-end' : 'items-start'}`}
                >
                  {/* Sender & Timestamp Header */}
                  <div className="flex items-center gap-2 mb-1 px-1">
                    <span className="text-xs font-semibold text-gray-600">
                      {isStaff
                        ? 'Сотрудник ZAMK'
                        : isSellerSender
                        ? 'Продавец'
                        : 'Покупатель'}
                    </span>
                    <span className="text-[11px] text-gray-400">{formatTime(msg.createdAt)}</span>
                  </div>

                  {/* Message Bubble */}
                  <div
                    className={`max-w-xl rounded-2xl p-4 shadow-sm text-sm leading-relaxed ${
                      isStaff
                        ? 'bg-blue-600 text-white rounded-tr-none'
                        : 'bg-white text-gray-900 border border-gray-200 rounded-tl-none'
                    }`}
                  >
                    <p className="whitespace-pre-wrap">{msg.textContent}</p>

                    {/* Context Chips */}
                    {msg.contextLinks && msg.contextLinks.length > 0 && (
                      <div className="mt-3 pt-2.5 border-t border-black/10 flex flex-wrap gap-2">
                        {msg.contextLinks.map(renderContextChip)}
                      </div>
                    )}

                    {/* Attachments */}
                    {msg.attachments && msg.attachments.length > 0 && (
                      <div className="mt-3 pt-2.5 border-t border-black/10 flex flex-col gap-2">
                        {msg.attachments.map(renderAttachment)}
                      </div>
                    )}
                  </div>
                </div>
              </Fragment>
            );
          })
        )}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
