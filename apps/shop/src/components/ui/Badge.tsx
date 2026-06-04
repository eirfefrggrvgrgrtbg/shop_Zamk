import { cn } from "../../lib/utils"

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: 'default' | 'new' | 'sale' | 'bestseller';
}

function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  const variants: Record<string, string> = {
    default: "bg-surface text-graphite border border-border-lighter",
    new: "bg-graphite text-white dark:text-black border-0 shadow-sm",
    sale: "bg-error text-white border-0 shadow-sm",
    bestseller: "bg-accent text-white border-0 shadow-sm",
  }

  return (
    <span
      className={cn(
        "inline-flex items-center px-3 py-1 rounded-full text-[10px] font-bold tracking-wider uppercase transition-colors shrink-0",
        variants[variant],
        className
      )}
      {...props}
    />
  )
}

export { Badge }
