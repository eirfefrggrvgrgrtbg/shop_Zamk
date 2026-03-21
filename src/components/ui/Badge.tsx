import { cn } from "../../lib/utils"

interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: 'default' | 'new' | 'sale' | 'bestseller';
}

function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  const variants: Record<string, string> = {
    default: "bg-surface text-graphite border border-border-lighter",
    new: "bg-milk text-graphite border border-graphite-light",
    sale: "bg-error/10 text-error border border-error/20",
    bestseller: "bg-graphite text-white border border-graphite",
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
