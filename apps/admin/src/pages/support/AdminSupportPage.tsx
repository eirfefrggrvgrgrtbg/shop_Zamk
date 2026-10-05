import { useState, useEffect, useCallback } from 'react';
import {
  MessageSquare,
  AlertCircle,
  RefreshCw,
} from 'lucide-react';
import {
  getAdminSupportConversations,
  getAdminSupportConversation,
  sendAdminSupportReply,
  createAdminSupportInternalNote,
  markAdminSupportRead,
  completeAdminSupportSession,
  updateAdminSupportSession,
  type SupportConversation,
  type SupportConversationDetail,
  type SupportRequesterType,
  type SupportSessionPriority,
} from '../../api/adminSupport';
import { SupportInbox } from '../../components/support/SupportInbox';
import { SupportConversationView } from '../../components/support/SupportConversationView';
import { SupportComposer } from '../../components/support/SupportComposer';
import { SupportContextPanel } from '../../components/support/SupportContextPanel';

export function AdminSupportPage() {
  const [conversations, setConversations] = useState<SupportConversation[]>([]);
  const [selectedConvId, setSelectedConvId] = useState<string | null>(null);
  const [currentDetail, setCurrentDetail] = useState<SupportConversationDetail | null>(null);
  const [filter, setFilter] = useState<'ALL' | SupportRequesterType>('ALL');
  const [search, setSearch] = useState<string>('');

  const [loadingInbox, setLoadingInbox] = useState<boolean>(true);
  const [loadingDetail, setLoadingDetail] = useState<boolean>(false);
  const [sending, setSending] = useState<boolean>(false);
  const [inboxError, setInboxError] = useState<string | null>(null);
  const [isContextPanelOpen, setIsContextPanelOpen] = useState<boolean>(true);

  // Load Inbox Conversations
  const loadConversations = useCallback(async () => {
    setLoadingInbox(true);
    setInboxError(null);
    try {
      const data = await getAdminSupportConversations({
        filter: filter === 'ALL' ? undefined : filter,
        search: search.trim() ? search.trim() : undefined,
      });
      setConversations(data || []);

      // If current selection is still in list, keep it; else select first or keep null
      if (selectedConvId && data && !data.some((c) => c.id === selectedConvId)) {
        // Keep selected if still valid
      }
    } catch (e: any) {
      setInboxError(e.message || 'Ошибка загрузки диалогов службы поддержки');
    } finally {
      setLoadingInbox(false);
    }
  }, [filter, search, selectedConvId]);

  useEffect(() => {
    loadConversations();
  }, [filter, search]);

  // Load Selected Conversation Detail
  const handleSelectConversation = async (conv: SupportConversation) => {
    setSelectedConvId(conv.id);
    setLoadingDetail(true);
    try {
      const detail = await getAdminSupportConversation(conv.id);
      setCurrentDetail(detail);

      // Auto mark read if conversation had unread count
      if (conv.unreadCount > 0) {
        markAdminSupportRead(conv.id).catch(() => {});
        setConversations((prev) =>
          prev.map((c) => (c.id === conv.id ? { ...c, unreadCount: 0 } : c))
        );
      }
    } catch (e: any) {
      setInboxError(e.message || 'Ошибка загрузки сообщений диалога');
    } finally {
      setLoadingDetail(false);
    }
  };

  // Reply submission
  const handleSendReply = async (textContent: string) => {
    if (!selectedConvId) return;
    setSending(true);
    try {
      const newMsg = await sendAdminSupportReply(selectedConvId, { textContent });
      setCurrentDetail((prev) =>
        prev
          ? {
              ...prev,
              messages: [...prev.messages, newMsg],
            }
          : prev
      );
      // Update snippet in inbox list
      setConversations((prev) =>
        prev.map((c) =>
          c.id === selectedConvId
            ? {
                ...c,
                latestMessageText: newMsg.textContent,
                latestMessageAt: newMsg.createdAt,
              }
            : c
        )
      );
    } finally {
      setSending(false);
    }
  };

  // Internal Note submission
  const handleSendInternalNote = async (textContent: string) => {
    if (!selectedConvId) return;
    setSending(true);
    try {
      const newNote = await createAdminSupportInternalNote(selectedConvId, { textContent });
      setCurrentDetail((prev) =>
        prev
          ? {
              ...prev,
              internalNotes: [...(prev.internalNotes || []), newNote],
            }
          : prev
      );
    } finally {
      setSending(false);
    }
  };

  // Session metadata update
  const handleUpdateSession = async (data: {
    priority?: SupportSessionPriority;
    categoryId?: string;
    clearCategory?: boolean;
    assignedTo?: string;
    clearAssignee?: boolean;
  }) => {
    if (!selectedConvId) return;
    await updateAdminSupportSession(selectedConvId, data);

    // Refresh conversation detail
    const updatedDetail = await getAdminSupportConversation(selectedConvId);
    setCurrentDetail(updatedDetail);

    // Update in inbox list
    setConversations((prev) =>
      prev.map((c) =>
        c.id === selectedConvId
          ? {
              ...c,
              activeSession: updatedDetail.conversation.activeSession,
            }
          : c
      )
    );
  };

  // Complete dialogue
  const handleCompleteSession = async () => {
    if (!selectedConvId) return;
    await completeAdminSupportSession(selectedConvId);

    // Refresh detail
    const updatedDetail = await getAdminSupportConversation(selectedConvId);
    setCurrentDetail(updatedDetail);

    // Update in inbox
    setConversations((prev) =>
      prev.map((c) =>
        c.id === selectedConvId
          ? {
              ...c,
              activeSession: updatedDetail.conversation.activeSession,
            }
          : c
      )
    );
  };

  const isCompleted =
    !currentDetail?.conversation.activeSession ||
    currentDetail.conversation.activeSession.status === 'COMPLETED';

  return (
    <div className="flex h-[calc(100vh-64px)] w-full overflow-hidden bg-white">
      {/* Left Pane: Inbox */}
      <SupportInbox
        conversations={conversations}
        selectedId={selectedConvId}
        onSelect={handleSelectConversation}
        filter={filter}
        onFilterChange={setFilter}
        search={search}
        onSearchChange={setSearch}
        loading={loadingInbox}
      />

      {/* Center & Right Panes */}
      <div className="flex flex-1 min-w-0 h-full overflow-hidden">
        {inboxError && (
          <div
            data-testid="support-inbox-error"
            className="absolute top-4 right-4 z-50 bg-red-600 text-white px-4 py-2 rounded-lg text-xs font-medium shadow-lg flex items-center gap-2"
          >
            <AlertCircle className="w-4 h-4" />
            <span>{inboxError}</span>
            <button
              onClick={() => setInboxError(null)}
              className="ml-2 underline text-[11px] opacity-80 hover:opacity-100"
            >
              Закрыть
            </button>
          </div>
        )}

        {loadingDetail ? (
          <div className="flex-1 flex flex-col items-center justify-center bg-slate-50 gap-3 text-gray-500">
            <RefreshCw className="w-6 h-6 animate-spin text-gray-400" />
            <p className="text-sm font-medium">Загрузка сообщений диалога...</p>
          </div>
        ) : currentDetail ? (
          <>
            {/* Center: Conversation & Composer */}
            <div className="flex flex-col flex-1 min-w-0 h-full">
              <SupportConversationView
                conversation={currentDetail.conversation}
                messages={currentDetail.messages}
                internalNotes={currentDetail.internalNotes}
                isContextPanelOpen={isContextPanelOpen}
                onToggleContextPanel={() => setIsContextPanelOpen((prev) => !prev)}
              />
              <SupportComposer
                onSendReply={handleSendReply}
                onSendInternalNote={handleSendInternalNote}
                isCompleted={isCompleted}
                sending={sending}
              />
            </div>

            {/* Right: Context & Operator Controls */}
            <SupportContextPanel
              conversation={currentDetail.conversation}
              onUpdateSession={handleUpdateSession}
              onCompleteSession={handleCompleteSession}
              isOpen={isContextPanelOpen}
              onClose={() => setIsContextPanelOpen(false)}
            />
          </>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center bg-slate-50 p-6 text-center text-gray-400 select-none">
            <div className="w-14 h-14 rounded-2xl bg-gray-100 flex items-center justify-center mb-3 text-gray-400">
              <MessageSquare className="w-7 h-7" />
            </div>
            <h3 className="text-base font-semibold text-gray-700">Выберите диалог</h3>
            <p className="text-xs text-gray-400 max-w-sm mt-1">
              Выберите обращение покупателя или продавца из списка слева, чтобы просмотреть переписку и ответить.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
