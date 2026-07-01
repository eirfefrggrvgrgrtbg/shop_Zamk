import { useState, useEffect } from 'react';
import { Clock } from 'lucide-react';

interface CountdownProps {
  endsAt: string;
  className?: string;
  onEnd?: () => void;
}

export function Countdown({ endsAt, className = '', onEnd }: CountdownProps) {
  const [timeLeft, setTimeLeft] = useState<{
    days: number;
    hours: number;
    minutes: number;
    seconds: number;
    total: number;
  }>({ days: 0, hours: 0, minutes: 0, seconds: 0, total: 0 });

  useEffect(() => {
    const calculateTimeLeft = () => {
      const difference = new Date(endsAt).getTime() - new Date().getTime();
      
      if (difference <= 0) {
        if (timeLeft.total > 0 && onEnd) {
          onEnd();
        }
        return { days: 0, hours: 0, minutes: 0, seconds: 0, total: 0 };
      }

      return {
        days: Math.floor(difference / (1000 * 60 * 60 * 24)),
        hours: Math.floor((difference / (1000 * 60 * 60)) % 24),
        minutes: Math.floor((difference / 1000 / 60) % 60),
        seconds: Math.floor((difference / 1000) % 60),
        total: difference
      };
    };

    setTimeLeft(calculateTimeLeft());

    const timer = setInterval(() => {
      setTimeLeft(calculateTimeLeft());
    }, 1000);

    return () => clearInterval(timer);
  }, [endsAt, onEnd]);

  if (timeLeft.total <= 0) {
    return (
      <div className={`flex items-center gap-1.5 text-red-500 font-medium ${className}`}>
        <Clock className="w-4 h-4" />
        <span>Завершено</span>
      </div>
    );
  }

  // Visual urgency
  let colorClass = 'text-green-600 dark:text-green-400';
  let isPulsing = false;
  if (timeLeft.total < 60000) { // < 1 minute
    colorClass = 'text-red-500';
    isPulsing = true;
  } else if (timeLeft.total < 3600000) { // < 1 hour
    colorClass = 'text-orange-500';
  }

  return (
    <div className={`flex items-center gap-1.5 font-medium ${colorClass} ${isPulsing ? 'animate-pulse' : ''} ${className}`}>
      <Clock className="w-4 h-4" />
      <span className="tabular-nums">
        {timeLeft.days > 0 && `${timeLeft.days}д `}
        {String(timeLeft.hours).padStart(2, '0')}:
        {String(timeLeft.minutes).padStart(2, '0')}:
        {String(timeLeft.seconds).padStart(2, '0')}
      </span>
    </div>
  );
}
