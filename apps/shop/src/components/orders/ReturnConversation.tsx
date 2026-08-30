import { useState, useEffect, useRef, useCallback } from 'react';
import { MessageSquare, Send, AlertCircle, Paperclip, X, Loader2 } from 'lucide-react';
import { getCustomerReturnMessages, sendCustomerReturnMessage, uploadCustomerReturnMessageAttachment } from '@zamk/api-client/src/customer';
import { type ReturnMessage, isReturnConversationWritable } from '@zamk/api-client/src/types';
import { useToast } from '../../contexts/ToastContext';

interface ReturnConversationProps {
  returnId: string;
  status: string;
  onMessageSent: () => void;
}

interface StagedAttachment {
  file: File;
  previewUrl: string;
}

export function ReturnConversation({ returnId, status, onMessageSent }: ReturnConversationProps) {
  const [messages, setMessages] = useState<ReturnMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [messageBody, setMessageBody] = useState('');
  const [sending, setSending] = useState(false);
  const [attachments, setAttachments] = useState<StagedAttachment[]>([]);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedImage, setSelectedImage] = useState<string | null>(null);

  let showToast: ((msg: string, type?: 'success' | 'error' | 'info') => void) | null = null;
  try {
    const toastContext = useToast();
    showToast = toastContext.showToast;
  } catch {
    // Graceful fallback for test environments without ToastProvider
  }

  const clearAttachments = useCallback(() => {
    setAttachments(prev => {
      prev.forEach(item => URL.revokeObjectURL(item.previewUrl));
      return [];
    });
  }, []);

  useEffect(() => {
    loadMessages();
  }, [returnId]);

  useEffect(() => {
    return () => {
      attachments.forEach(item => URL.revokeObjectURL(item.previewUrl));
    };
  }, []);

  const loadMessages = async () => {
    try {
      setLoading(true);
      const res = await getCustomerReturnMessages(returnId);
      setMessages(res.messages || []);
    } catch (err) {
      console.error('Failed to load messages:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSend = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const trimmed = messageBody.trim();
    if (!trimmed && attachments.length === 0) return;
    if (sending || uploading) return;

    try {
      setSending(true);

      const attachmentIds: string[] = [];
      if (attachments.length > 0) {
        setUploading(true);
        for (const item of attachments) {
          const res = await uploadCustomerReturnMessageAttachment(returnId, item.file);
          attachmentIds.push(res.id);
        }
        setUploading(false);
      }

      await sendCustomerReturnMessage(returnId, { message: trimmed, attachmentIds });

      setMessageBody('');
      clearAttachments();
      if (showToast) {
        showToast('Сообщение отправлено', 'success');
      }
      await loadMessages();
      onMessageSent();
    } catch (err: any) {
      if (showToast) {
        showToast(err.message || 'Ошибка отправки', 'error');
      }
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
      <div className="fixed inset-0 bg-black/90 z-[100] flex items-center justify-center p-4" onClick={() => setSelectedImage(null)}>
        <button className="absolute top-4 right-4 p-2 text-white/70 hover:text-white" onClick={() => setSelectedImage(null)}>
          <X className="w-6 h-6" />
        </button>
        <img src={selectedImage} alt="Attachment" className="max-w-full max-h-full object-contain" onClick={(e) => e.stopPropagation()} />
      </div>
    );
  };

  if (loading && messages.length === 0) {
    return (
      <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm text-center">
        <div className="animate-spin w-5 h-5 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent mx-auto" />
      </div>
    );
  }

  const canReply = isReturnConversationWritable(status);

  return (
    <>
      {renderLightbox()}
      <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm flex flex-col h-full max-h-[600px]">
        <div className="flex items-center justify-between mb-4 flex-shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-7 h-7 rounded-full bg-graphite/5 dark:bg-white/10 flex items-center justify-center text-graphite/70 dark:text-white/70">
              <MessageSquare className="w-3.5 h-3.5" />
            </div>
            <h3 className="text-base font-semibold text-graphite dark:text-white font-sans">
              Переписка по возврату
            </h3>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto space-y-4 pl-0 md:pl-10 pr-2 scrollbar-thin">
          {status === 'needs_info' && (
            <div className="p-3 rounded-xl bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800/50 flex items-start gap-2.5 text-orange-800 dark:text-orange-300">
              <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
              <div className="text-sm font-medium leading-snug">
                Нужен ваш ответ. Пожалуйста, ответьте на вопросы поддержки, чтобы мы могли продолжить рассмотрение заявки.
              </div>
            </div>
          )}

          {messages.length === 0 ? (
            <div className="py-6 text-center text-sm text-graphite/50 dark:text-white/50">
              Пока сообщений нет.
            </div>
          ) : (
            messages.map((msg) => {
              const isAdmin = msg.senderRole === 'admin';
              const senderLabel = isAdmin ? 'ZAMK' : 'Вы';
              const timeLabel = new Date(msg.createdAt).toLocaleString('ru-RU', {
                day: 'numeric',
                month: 'long',
                hour: '2-digit',
                minute: '2-digit',
              });

              return (
                <div
                  key={msg.id}
                  className={`flex flex-col max-w-[85%] ${
                    isAdmin ? 'items-start' : 'items-end ml-auto'
                  }`}
                >
                  <div className="flex items-center gap-1.5 mb-1 px-1 text-xs">
                    <span className={`font-semibold ${isAdmin ? 'text-graphite dark:text-white' : 'text-graphite/70 dark:text-white/70'}`}>
                      {senderLabel}
                    </span>
                    <span className="text-graphite/40 dark:text-white/40">·</span>
                    <span className="text-graphite/60 dark:text-white/60 text-[11px]">{timeLabel}</span>
                  </div>
                  {msg.body ? (
                    <div
                      className={`text-sm p-3.5 rounded-2xl whitespace-pre-wrap ${
                        isAdmin
                          ? 'bg-graphite/5 dark:bg-white/10 text-graphite dark:text-white rounded-tl-none border border-graphite/10 dark:border-white/10'
                          : 'bg-black text-white dark:bg-white dark:text-black rounded-tr-none'
                      }`}
                    >
                      {msg.body}
                    </div>
                  ) : null}
                  {msg.attachments && msg.attachments.length > 0 && (
                    <div className={"flex flex-wrap gap-2 mt-2 " + (isAdmin ? 'justify-start' : 'justify-end')}>
                      {msg.attachments.map(att => (
                        <div key={att.id} className="relative rounded-lg overflow-hidden border border-graphite/10 dark:border-white/10 cursor-pointer" onClick={() => setSelectedImage(att.url)}>
                          <img src={att.url} alt="Attachment" className="w-32 h-24 object-cover" />
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>

        {canReply && (
          <div className="mt-4 pt-4 border-t border-graphite/5 dark:border-white/5 pl-0 md:pl-10 flex-shrink-0">
            <form onSubmit={handleSend} className="flex flex-col gap-3">
              <div className="bg-white dark:bg-white/5 border border-graphite/20 dark:border-white/20 rounded-xl focus-within:border-black dark:focus-within:border-white transition-colors">
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

              <div className="flex justify-between items-center">
                <div className="flex items-center gap-2">
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
                    className="p-2 text-graphite/50 hover:text-black dark:text-white/50 dark:hover:text-white transition-colors disabled:opacity-50 cursor-pointer"
                    title="Прикрепить фото (макс. 6)"
                  >
                    <Paperclip className="w-5 h-5" />
                  </button>
                </div>
                <button
                  type="submit"
                  disabled={(!messageBody.trim() && attachments.length === 0) || sending || uploading}
                  className="inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-full bg-black text-white dark:bg-white dark:text-black text-xs font-medium hover:opacity-90 disabled:opacity-50 transition-opacity cursor-pointer"
                >
                  {uploading ? 'Загрузка...' : 'Отправить'} {(sending || uploading) ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
                </button>
              </div>
            </form>
          </div>
        )}
      </div>
    </>
  );
}
