import { cn } from "../../lib/utils"

interface BadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'new' | 'sale' | 'bestseller';
  className?: string;
}

export function Badge({ children, variant = 'default', className }: BadgeProps) {
  const variants: Record<string, string> = {
    default: 'bg-surface text-ash border-border-lighter',
    new: 'bg-primary/10 text-primary-hover border-primary/20',
    sale: 'bg-error/10 text-error border-error/20',
    bestseller: 'bg-warning/10 text-warning border-warning/20',
  };

  return (
    <span className={cn(
      "inline-flex items-center px-2.5 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider border",
      variants[variant],
      className
    )}>
      {children}
    </span>
  );
}
