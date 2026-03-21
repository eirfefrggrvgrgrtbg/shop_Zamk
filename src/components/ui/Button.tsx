import { cn } from "../../lib/utils"

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'pill' | 'outline' | 'link';
  size?: 'sm' | 'md' | 'lg' | 'icon';
}

function Button({ className, variant = 'primary', size = 'md', ...props }: ButtonProps) {
  const baseStyles = "inline-flex items-center justify-center font-medium transition-all active:scale-[0.98] focus-visible:outline-none disabled:opacity-50 disabled:pointer-events-none";
  
  const variants: Record<string, string> = {
    primary: "bg-primary text-white hover:bg-primary-hover shadow-[0_4px_14px_0_rgba(137,207,240,0.39)] hover:shadow-[0_6px_20px_rgba(137,207,240,0.23)] hover:-translate-y-0.5 rounded-full",
    secondary: "bg-surface backdrop-blur-md text-graphite hover:bg-surface-hover border border-border-soft hover:border-primary/50 shadow-sm rounded-full",
    ghost: "bg-transparent text-graphite hover:bg-primary-soft hover:text-primary rounded-full",
    pill: "bg-surface backdrop-blur-md text-graphite hover:bg-primary/20 hover:text-primary border border-border-lighter rounded-full",
    outline: "bg-transparent border-2 border-primary text-primary hover:bg-primary hover:text-white rounded-full",
    link: "bg-transparent text-primary hover:text-primary-hover underline-offset-4 hover:underline rounded-none !p-0 h-auto",
  }

  const sizes: Record<string, string> = {
    sm: "h-9 px-4 text-xs",
    md: "h-11 px-6 text-sm",
    lg: "h-14 px-8 text-base",
    icon: "h-11 w-11",
  }

  return (
    <button
      className={cn(baseStyles, variants[variant], sizes[size], className)}
      {...props}
    />
  )
}

export { Button }
