import type { ButtonHTMLAttributes } from 'react';
import { cn } from './presentationUtils';

export interface PresentationButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary';
  size?: 'md' | 'icon';
}

export function PresentationButton({
  className,
  variant = 'primary',
  size = 'md',
  ...props
}: PresentationButtonProps) {
  const baseStyles =
    'inline-flex items-center justify-center font-medium transition-all duration-300 active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-graphite/40 dark:focus-visible:ring-white/40 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-[#111214] disabled:opacity-50 disabled:pointer-events-none cursor-pointer';

  const variants: Record<string, string> = {
    primary:
      'bg-graphite dark:bg-white text-white dark:text-black hover:bg-graphite-light dark:hover:bg-gray-100 shadow-sm hover:shadow hover:-translate-y-px rounded-lg',
    secondary:
      'bg-white dark:bg-transparent text-graphite dark:text-white hover:bg-milk dark:hover:bg-white/5 border border-border-soft dark:border-white/20 hover:border-graphite/40 dark:hover:border-white/40 shadow-sm rounded-lg',
  };

  const sizes: Record<string, string> = {
    md: 'h-11 px-6 text-sm',
    icon: 'h-11 w-11',
  };

  return (
    <button
      className={cn(baseStyles, variants[variant], sizes[size], className)}
      {...props}
    />
  );
}
