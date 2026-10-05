import { useState, type FormEvent, type KeyboardEvent } from 'react';
import { Send, Lock, MessageSquare } from 'lucide-react';

interface SupportComposerProps {
  onSendReply: (text: string) => Promise<void>;
  onSendInternalNote: (text: string) => Promise<void>;
  isCompleted: boolean;
  sending: boolean;
}

export function SupportComposer({
  onSendReply,
  onSendInternalNote,
  isCompleted,
  sending,
}: SupportComposerProps) {
  const [mode, setMode] = useState<'reply' | 'note'>('reply');
  const [text, setText] = useState('');

  const isNote = mode === 'note';
  const trimmed = text.trim();
  const canSend = trimmed.length > 0 && !sending && !isCompleted;

  const handleSubmit = async (e?: FormEvent) => {
    if (e) e.preventDefault();
    if (!canSend) return;

    const payload = trimmed;
    setText('');
    try {
      if (isNote) {
        await onSendInternalNote(payload);
      } else {
        await onSendReply(payload);
      }
    } catch {
      // Restore on failure
      setText(payload);
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  };

  if (isCompleted) {
    return (
      <div className="p-4 bg-gray-100 border-t border-gray-200 text-center select-none">
        <p className="text-xs font-semibold text-gray-700">Диалог завершён</p>
        <p className="text-[11px] text-gray-500 mt-0.5">
          Новое обращение пользователя автоматически начнёт следующий диалог.
        </p>
      </div>
    );
  }

  return (
    <div
      className={`border-t transition-colors ${
        isNote ? 'bg-amber-50/70 border-amber-200' : 'bg-white border-gray-200'
      } p-4`}
    >
      {/* Mode Switcher Tabs */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-1 bg-gray-100 p-0.5 rounded-lg text-xs font-medium">
          <button
            type="button"
            data-testid="composer-mode-reply"
            onClick={() => setMode('reply')}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-colors ${
              !isNote
                ? 'bg-white text-gray-900 shadow-sm font-semibold'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            <MessageSquare className="w-3.5 h-3.5 text-blue-600" />
            <span>Ответить</span>
          </button>
          <button
            type="button"
            data-testid="composer-mode-note"
            onClick={() => setMode('note')}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-colors ${
              isNote
                ? 'bg-amber-500 text-white shadow-sm font-semibold'
                : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            <Lock className="w-3.5 h-3.5" />
            <span>Внутренняя заметка</span>
          </button>
        </div>

        {isNote && (
          <span className="text-[11px] font-medium text-amber-800 flex items-center gap-1">
            <Lock className="w-3 h-3 text-amber-700" />
            Видно только операторам ZAMK
          </span>
        )}
      </div>

      {/* Input Textarea & Submit */}
      <form onSubmit={handleSubmit} className="flex flex-col gap-2">
        <textarea
          data-testid="composer-input"
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={
            isNote
              ? 'Добавить внутреннюю заметку по обращению...'
              : 'Написать ответ клиенту...'
          }
          rows={3}
          className={`w-full text-sm p-3 rounded-lg border focus:outline-none transition-colors resize-none ${
            isNote
              ? 'bg-white border-amber-300 focus:border-amber-500 focus:ring-1 focus:ring-amber-500 text-amber-950 placeholder-amber-400'
              : 'bg-gray-50 border-gray-200 focus:bg-white focus:border-blue-500 focus:ring-1 focus:ring-blue-500 text-gray-900 placeholder-gray-400'
          }`}
        />

        <div className="flex items-center justify-between">
          <span className="text-[11px] text-gray-400 hidden sm:inline">
            ⌘ + Enter для отправки
          </span>

          <button
            type="submit"
            data-testid="composer-submit"
            disabled={!canSend}
            className={`ml-auto flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold shadow-sm transition-all focus:outline-none ${
              !canSend
                ? 'bg-gray-200 text-gray-400 cursor-not-allowed shadow-none'
                : isNote
                ? 'bg-amber-600 hover:bg-amber-700 text-white active:scale-[0.98]'
                : 'bg-blue-600 hover:bg-blue-700 text-white active:scale-[0.98]'
            }`}
          >
            {isNote ? <Lock className="w-3.5 h-3.5" /> : <Send className="w-3.5 h-3.5" />}
            <span>{isNote ? 'Добавить заметку' : 'Отправить'}</span>
          </button>
        </div>
      </form>
    </div>
  );
}
