import { cn } from "../../lib/utils"

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: 'default' | 'new' | 'sale' | 'bestseller';
}

function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  const variants: Record<string, string> = {
    default: "bg-surface text-graphite border border-white/50 backdrop-blur-md",
    new: "bg-primary/20 text-primary border border-primary/30 backdrop-blur-md",
    sale: "bg-error/15 text-error border border-error/30 backdrop-blur-md",
    bestseller: "bg-graphite/10 text-graphite border border-graphite/20 backdrop-blur-md",
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
