import { useState, useRef, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { HelpCircle } from 'lucide-react';

interface HelpTooltipProps {
  content: string;
}

export function HelpTooltip({ content }: HelpTooltipProps) {
  const [isVisible, setIsVisible] = useState(false);
  const [coords, setCoords] = useState({ top: 0, left: 0 });
  const iconRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isVisible && iconRef.current) {
      const rect = iconRef.current.getBoundingClientRect();
      setCoords({
        top: rect.top + window.scrollY,
        left: rect.left + window.scrollX + rect.width / 2,
      });
    }
  }, [isVisible]);

  return (
    <div 
      ref={iconRef}
      className="inline-flex items-center ml-1 group relative cursor-help"
      onMouseEnter={() => setIsVisible(true)}
      onMouseLeave={() => setIsVisible(false)}
    >
      <HelpCircle className="h-4 w-4 text-slate-400 hover:text-indigo-500 transition-colors" />
      
      {isVisible && createPortal(
        <div 
          className="absolute z-[9999] p-2 bg-slate-800 text-white text-xs rounded-md shadow-lg text-center leading-relaxed"
          style={{
            top: coords.top - 8,
            left: coords.left,
            transform: 'translate(-50%, -100%)',
            width: 'max-content',
            maxWidth: '256px'
          }}
        >
          {content}
          {/* Arrow */}
          <div className="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-slate-800"></div>
        </div>,
        document.body
      )}
    </div>
  );
}
