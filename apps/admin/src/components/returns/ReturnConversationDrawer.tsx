import { useState, useEffect, useRef, useCallback } from 'react';
import { X, Send, MessageSquare, Paperclip, Loader2 } from 'lucide-react';
import { ReturnMessage, isReturnConversationWritable } from '@zamk/api-client/src/types';
import { getAdminReturnMessages, sendAdminReturnMessage, uploadAdminReturnMessageAttachment } from '@zamk/api-client/src/admin';

interface ReturnConversationDrawerProps {
  returnId: string;
  isOpen: boolean;
  onClose: () => void;
  status: string;
  onStatusChange: () => void;
}

interface StagedAttachment {
  file: File;
  previewUrl: string;
}

export function ReturnConversationDrawer({ returnId, isOpen, onClose, status, onStatusChange }: ReturnConversationDrawerProps) {
  const [messages, setMessages] = useState<ReturnMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [messageBody, setMessageBody] = useState('');
  const [sending, setSending] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [needsResponse, setNeedsResponse] = useState(false);
  const [attachments, setAttachments] = useState<StagedAttachment[]>([]);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedImage, setSelectedImage] = useState<string | null>(null);

  const clearAttachments = useCallback(() => {
    setAttachments(prev => {
      prev.forEach(item => URL.revokeObjectURL(item.previewUrl));
      return [];
    });
  }, []);

  useEffect(() => {
    if (isOpen) {
      loadMessages();
    }
  }, [isOpen, returnId]);

  useEffect(() => {
    return () => {
      attachments.forEach(item => URL.revokeObjectURL(item.previewUrl));
    };
  }, []);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const loadMessages = async () => {
    try {
      setLoading(true);
      const res = await getAdminReturnMessages(returnId);
      setMessages(res.messages || []);
    } catch (err) {
      console.error('Failed to load messages:', err);
    } finally {
      setLoading(false);
    }
  };

  const canReply = isReturnConversationWritable(status);

  const handleSend = async () => {
    const trimmed = messageBody.trim();
    if (!trimmed && attachments.length === 0) return;
    if (sending || uploading) return;

    try {
      setSending(true);

      const attachmentIds: string[] = [];
      if (attachments.length > 0) {
        setUploading(true);
        for (const item of attachments) {
          const res = await uploadAdminReturnMessageAttachment(returnId, item.file);
          attachmentIds.push(res.id);
        }
        setUploading(false);
      }

      await sendAdminReturnMessage(returnId, {
        message: trimmed,
        needsResponse: status === 'requested' ? needsResponse : false,
        attachmentIds,
      });

      setMessageBody('');
      clearAttachments();
      setNeedsResponse(false);
      await loadMessages();
      if (status === 'requested' && needsResponse) {
        onStatusChange();
      }
    } catch (err: any) {
      alert(err.message || 'Ошибка отправки');
    } finally {
      setSending(false);
      setUploading(false);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      const newFiles = Array.from(e.target.files);
      const remainingSlots = 6 - attachments.length;
      const filesToAdd = newFiles.slice(0, remainingSlots);

      const newStaged: StagedAttachment[] = filesToAdd.map(file => ({
        file,
        previewUrl: URL.createObjectURL(file),
      }));

      setAttachments(prev => [...prev, ...newStaged]);
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const removeAttachment = (index: number) => {
    setAttachments(prev => {
      const target = prev[index];
      if (target) {
        URL.revokeObjectURL(target.previewUrl);
      }
      return prev.filter((_, i) => i !== index);
    });
  };

  const renderLightbox = () => {
    if (!selectedImage) return null;
    return (
      <div className="fixed inset-0 bg-black/90 z-[60] flex items-center justify-center p-4" onClick={() => setSelectedImage(null)}>
        <button className="absolute top-4 right-4 p-2 text-white/70 hover:text-white" onClick={() => setSelectedImage(null)}>
          <X className="w-6 h-6" />
        </button>
        <img src={selectedImage} alt="Attachment" className="max-w-full max-h-full object-contain" onClick={(e) => e.stopPropagation()} />
      </div>
    );
  };

  if (!isOpen) return null;

  return (
    <>
      {renderLightbox()}
      <div className="fixed inset-0 bg-black/20 dark:bg-black/40 z-40 transition-opacity" onClick={onClose} />
      <div className="fixed top-0 right-0 h-full w-full max-w-md bg-white dark:bg-graphite shadow-2xl z-50 flex flex-col transform transition-transform">
        <div className="flex items-center justify-between p-4 border-b border-graphite/10 dark:border-white/10">
          <div className="flex items-center gap-2">
            <MessageSquare className="w-5 h-5 text-graphite dark:text-white" />
            <h2 className="text-lg font-semibold text-graphite dark:text-white">Переписка с покупателем</h2>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-graphite/5 dark:hover:bg-white/10 rounded-full transition-colors">
            <X className="w-5 h-5 text-graphite/70 dark:text-white/70" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-4 bg-gray-50 dark:bg-graphite/50" ref={scrollRef}>
          {loading && messages.length === 0 ? (
            <div className="flex justify-center p-8">
              <div className="animate-spin w-6 h-6 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent" />
            </div>
          ) : messages.length === 0 ? (
            <div className="text-center text-sm text-graphite/50 dark:text-white/50 py-8">
              Пока сообщений нет.
            </div>
          ) : (
            messages.map((msg) => {
              const isAdmin = msg.senderRole === 'admin';
              const senderLabel = isAdmin ? 'Вы' : 'Покупатель';
              const timeLabel = new Date(msg.createdAt).toLocaleString('ru-RU', {
                day: 'numeric',
                month: 'long',
                hour: '2-digit',
                minute: '2-digit',
              });

              return (
                <div key={msg.id} className={"flex flex-col max-w-[85%] " + (isAdmin ? 'items-end ml-auto' : 'items-start')}>
                  <div className="flex items-center gap-1.5 mb-1 px-1 text-xs">
                    <span className={"font-semibold " + (isAdmin ? 'text-graphite dark:text-white' : 'text-graphite/70 dark:text-white/70')}>
                      {senderLabel}
                    </span>
                    <span className="text-graphite/40 dark:text-white/40">·</span>
                    <span className="text-graphite/60 dark:text-white/60 text-[11px]">{timeLabel}</span>
                  </div>
                  {msg.body ? (
                    <div className={"text-sm p-3.5 rounded-2xl whitespace-pre-wrap " + (
                      isAdmin
                        ? 'bg-black text-white dark:bg-white dark:text-black rounded-tr-none'
                        : 'bg-white dark:bg-graphite text-graphite dark:text-white border border-graphite/10 dark:border-white/10 rounded-tl-none'
                    )}>
                      {msg.body}
                    </div>
                  ) : null}
                  {msg.attachments && msg.attachments.length > 0 && (
                    <div className={"flex flex-wrap gap-2 mt-2 " + (isAdmin ? 'justify-end' : 'justify-start')}>
                      {msg.attachments.map(att => (
                        <div key={att.id} className="relative rounded-lg overflow-hidden border border-graphite/10 dark:border-white/10 cursor-pointer" onClick={() => setSelectedImage(att.url)}>
                          <img src={att.url} alt="Attachment" className="w-32 h-24 object-cover" />
                        </div>
                      ))}
                    </div>
                  )}
                  {msg.messageType === 'info_request' && isAdmin && (
                    <div className="text-[10px] text-orange-600 dark:text-orange-400 mt-1 mr-1 font-medium">Требуется ответ</div>
                  )}
                </div>
              );
            })
          )}
        </div>

        {canReply ? (
          <div className="p-4 border-t border-graphite/10 dark:border-white/10 bg-white dark:bg-graphite">
            <div className="bg-gray-50 dark:bg-white/5 border border-graphite/20 dark:border-white/20 rounded-xl mb-3 focus-within:border-black dark:focus-within:border-white">
              <textarea
                value={messageBody}
                onChange={(e) => setMessageBody(e.target.value)}
                placeholder="Введите сообщение..."
                className="w-full bg-transparent p-3 text-sm text-graphite dark:text-white focus:outline-none resize-none min-h-[80px]"
                disabled={sending || uploading}
              />
              {attachments.length > 0 && (
                <div className="flex gap-2 p-3 pt-0 flex-wrap border-t border-graphite/10 dark:border-white/10 mt-2">
                  {attachments.map((item, i) => (
                    <div key={i} className="relative w-16 h-16 rounded border border-graphite/10 dark:border-white/10 overflow-hidden bg-gray-100 flex items-center justify-center group">
                      <img src={item.previewUrl} alt="Preview" className="w-full h-full object-cover" />
                      <button type="button" onClick={() => removeAttachment(i)} className="absolute top-1 right-1 p-1 bg-black/60 rounded-full text-white opacity-0 group-hover:opacity-100 transition-opacity">
                        <X className="w-3 h-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="flex gap-2 justify-between items-center">
              <div className="flex items-center gap-4">
                <input
                  type="file"
                  ref={fileInputRef}
                  className="hidden"
                  multiple
                  accept="image/jpeg,image/png,image/webp"
                  onChange={handleFileSelect}
                  disabled={sending || uploading}
                />
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={sending || uploading || attachments.length >= 6}
                  className="p-2 text-graphite/60 dark:text-white/60 hover:text-graphite dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 rounded-full transition-colors disabled:opacity-50 cursor-pointer"
                  title="Прикрепить фото (макс. 6)"
                >
                  <Paperclip className="w-5 h-5" />
                </button>
                {status === 'requested' && (
                  <label className="flex items-center gap-2 cursor-pointer select-none">
                    <input
                      type="checkbox"
                      checked={needsResponse}
                      onChange={(e) => setNeedsResponse(e.target.checked)}
                      disabled={sending || uploading}
                      className="rounded border-graphite/30 text-black focus:ring-black accent-black"
                    />
                    <span className="text-sm font-medium text-graphite dark:text-white">Нужен ответ покупателя</span>
                  </label>
                )}
              </div>
              <button
                type="button"
                onClick={handleSend}
                disabled={(!messageBody.trim() && attachments.length === 0) || sending || uploading}
                className="inline-flex items-center gap-2 px-5 py-2 rounded-full bg-black text-white dark:bg-white dark:text-black text-xs font-medium hover:opacity-90 disabled:opacity-50 transition-opacity cursor-pointer"
              >
                {uploading ? 'Загрузка...' : 'Отправить'} {(sending || uploading) ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
              </button>
            </div>
          </div>
        ) : (
          <div className="p-4 bg-gray-50 dark:bg-white/5 text-center text-sm text-graphite/60 dark:text-white/60">
            Переписка закрыта (возврат завершен)
          </div>
        )}
      </div>
    </>
  );
}
