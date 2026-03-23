import { cn } from "../../lib/utils"

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'pill' | 'outline' | 'link';
  size?: 'sm' | 'md' | 'lg' | 'icon';
}

function Button({ className, variant = 'primary', size = 'md', ...props }: ButtonProps) {
  const baseStyles = "inline-flex items-center justify-center font-medium transition-all duration-300 active:scale-[0.98] focus-visible:outline-none disabled:opacity-50 disabled:pointer-events-none";

  const variants: Record<string, string> = {
    primary: "bg-graphite dark:bg-white text-white dark:text-black hover:bg-graphite-light dark:hover:bg-gray-100 shadow-sm hover:shadow hover:-translate-y-px rounded-lg",
    secondary: "bg-white dark:bg-transparent text-graphite dark:text-white hover:bg-milk dark:hover:bg-white/5 border border-border-soft dark:border-white/20 hover:border-graphite/40 dark:hover:border-white/40 shadow-sm rounded-lg",
    ghost: "bg-transparent text-graphite dark:text-white hover:bg-milk dark:hover:bg-white/5 rounded-lg",
    pill: "bg-white dark:bg-transparent text-graphite dark:text-white hover:bg-milk dark:hover:bg-white/5 border border-border-lighter dark:border-white/20 rounded-full shadow-sm",
    outline: "bg-transparent border border-border-soft dark:border-white/20 text-graphite dark:text-white hover:border-graphite dark:hover:border-white hover:bg-white dark:hover:bg-white/5 rounded-lg",
    link: "bg-transparent text-graphite dark:text-white hover:text-graphite-light dark:hover:text-white/70 underline-offset-4 hover:underline rounded-none !p-0 h-auto",
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
