import { cn } from "../../lib/utils"

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'pill' | 'outline' | 'link';
  size?: 'sm' | 'md' | 'lg' | 'icon';
}

function Button({ className, variant = 'primary', size = 'md', ...props }: ButtonProps) {
  const baseStyles = "inline-flex items-center justify-center font-medium transition-all duration-300 active:scale-[0.98] focus-visible:outline-none disabled:opacity-50 disabled:pointer-events-none";
  
  const variants: Record<string, string> = {
    primary: "bg-graphite text-white hover:bg-graphite-light shadow-sm hover:shadow hover:-translate-y-px rounded-full",
    secondary: "bg-white text-graphite hover:bg-milk border border-border-soft hover:border-graphite/40 shadow-sm rounded-full",
    ghost: "bg-transparent text-graphite hover:bg-milk rounded-full",
    pill: "bg-white text-graphite hover:bg-milk border border-border-lighter rounded-full shadow-sm",
    outline: "bg-transparent border border-border-soft text-graphite hover:border-graphite hover:bg-white rounded-full",
    link: "bg-transparent text-graphite hover:text-graphite-light underline-offset-4 hover:underline rounded-none !p-0 h-auto",
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
