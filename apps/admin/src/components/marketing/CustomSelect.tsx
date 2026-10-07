import { useState, useEffect, useRef, useId } from 'react';

export interface CustomSelectOption {
  value: string;
  label: string;
  disabled?: boolean;
  disabledReason?: string;
}

export interface CustomSelectProps {
  value: string;
  options: CustomSelectOption[];
  onChange: (val: string) => void;
  placeholder?: string;
  className?: string;
  'data-testid'?: string;
}

export function CustomSelect({
  value,
  options,
  onChange,
  placeholder = 'Выберите...',
  className = '',
  'data-testid': testId,
}: CustomSelectProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState<number>(-1);

  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listboxRef = useRef<HTMLDivElement>(null);

  const id = useId();
  const triggerId = `${id}-trigger`;
  const listboxId = `${id}-listbox`;

  const getFirstEnabledIndex = (opts: CustomSelectOption[]) => {
    return opts.findIndex(o => !o.disabled);
  };

  const getLastEnabledIndex = (opts: CustomSelectOption[]) => {
    for (let i = opts.length - 1; i >= 0; i--) {
      if (!opts[i].disabled) return i;
    }
    return -1;
  };

  const getNextEnabledIndex = (currentIndex: number, opts: CustomSelectOption[]) => {
    for (let i = currentIndex + 1; i < opts.length; i++) {
      if (!opts[i].disabled) return i;
    }
    return currentIndex;
  };

  const getPrevEnabledIndex = (currentIndex: number, opts: CustomSelectOption[]) => {
    for (let i = currentIndex - 1; i >= 0; i--) {
      if (!opts[i].disabled) return i;
    }
    return currentIndex;
  };

  const openListbox = (initialIdx?: number) => {
    setOpen(true);
    if (initialIdx !== undefined && initialIdx >= 0 && !options[initialIdx]?.disabled) {
      setActiveIndex(initialIdx);
    } else {
      const selectedIdx = options.findIndex(o => o.value === value && !o.disabled);
      if (selectedIdx >= 0) {
        setActiveIndex(selectedIdx);
      } else {
        setActiveIndex(getFirstEnabledIndex(options));
      }
    }
  };

  const closeListbox = (restoreFocus = true) => {
    setOpen(false);
    setActiveIndex(-1);
    if (restoreFocus) {
      triggerRef.current?.focus();
    }
  };

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
        setActiveIndex(-1);
      }
    }
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [open]);

  useEffect(() => {
    if (open && activeIndex >= 0 && listboxRef.current) {
      const activeEl = listboxRef.current.querySelector(`#${id}-opt-${activeIndex}`) as HTMLElement;
      if (activeEl) {
        activeEl.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [open, activeIndex, id]);

  const selectedOption = options.find(o => o.value === value);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!open) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        openListbox();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        openListbox(getLastEnabledIndex(options));
      } else if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        openListbox();
      }
      return;
    }

    if (e.key === 'Escape') {
      e.preventDefault();
      closeListbox(true);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex(curr => getNextEnabledIndex(curr, options));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex(curr => getPrevEnabledIndex(curr, options));
    } else if (e.key === 'Home') {
      e.preventDefault();
      setActiveIndex(getFirstEnabledIndex(options));
    } else if (e.key === 'End') {
      e.preventDefault();
      setActiveIndex(getLastEnabledIndex(options));
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (activeIndex >= 0 && activeIndex < options.length) {
        const opt = options[activeIndex];
        if (!opt.disabled) {
          onChange(opt.value);
          closeListbox(true);
        }
      }
    } else if (e.key === 'Tab') {
      closeListbox(false);
    }
  };

  return (
    <div className={`relative ${className}`} ref={containerRef} onKeyDown={handleKeyDown}>
      <button
        id={triggerId}
        ref={triggerRef}
        type="button"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        aria-activedescendant={open && activeIndex >= 0 ? `${id}-opt-${activeIndex}` : undefined}
        onClick={() => {
          if (open) {
            closeListbox(false);
          } else {
            openListbox();
          }
        }}
        data-testid={testId || `select-${placeholder}`}
        className="flex items-center justify-between w-full text-left bg-transparent border-b border-gray-900 pb-1.5 focus:outline-none group cursor-pointer"
      >
        <span className="text-[11px] font-bold uppercase tracking-widest text-gray-900 truncate pr-4 group-hover:text-gray-600 transition-colors">
          {selectedOption ? selectedOption.label : placeholder}
        </span>
        <svg
          className={`w-3 h-3 text-gray-900 transition-transform duration-200 ${open ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="square" strokeLinejoin="miter" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div
          id={listboxId}
          ref={listboxRef}
          role="listbox"
          aria-labelledby={triggerId}
          tabIndex={-1}
          className="absolute z-50 top-full left-0 mt-2 w-max min-w-[200px] max-h-96 overflow-y-auto bg-white border border-gray-900 shadow-xl py-2 focus:outline-none"
        >
          {options.map((opt, idx) => {
            const isSelected = value === opt.value;
            const isFocused = activeIndex === idx;

            return (
              <div
                key={opt.value}
                id={`${id}-opt-${idx}`}
                role="option"
                aria-selected={isSelected}
                aria-disabled={opt.disabled ? 'true' : undefined}
                onClick={() => {
                  if (!opt.disabled) {
                    onChange(opt.value);
                    closeListbox(true);
                  }
                }}
                onMouseEnter={() => {
                  if (!opt.disabled) {
                    setActiveIndex(idx);
                  }
                }}
                className={`w-full text-left px-4 py-2 text-[11px] uppercase tracking-widest font-medium select-none ${
                  opt.disabled
                    ? 'text-gray-300 cursor-not-allowed'
                    : 'text-gray-900 hover:bg-gray-50 cursor-pointer'
                } ${isSelected ? 'bg-gray-100 font-semibold' : ''} ${
                  isFocused && !isSelected && !opt.disabled ? 'bg-gray-50' : ''
                }`}
                title={opt.disabledReason}
              >
                {opt.label}
                {opt.disabled && (
                  <div className="text-[9px] text-gray-400 normal-case tracking-normal mt-0.5">
                    {opt.disabledReason}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
