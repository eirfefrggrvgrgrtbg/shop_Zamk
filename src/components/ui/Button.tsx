import * as React from "react"
import { cn } from "../../lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost" | "pill" | "outline" | "link";
  size?: "default" | "sm" | "lg" | "icon";
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "primary", size = "default", ...props }, ref) => {

    const baseStyles = "inline-flex items-center justify-center whitespace-nowrap font-medium transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 select-none";

    const variants: Record<string, string> = {
      primary: "bg-primary text-white rounded-2xl shadow-sm hover:bg-primary-hover hover:shadow-md active:scale-[0.97]",
      secondary: "bg-white text-graphite border border-border-soft rounded-2xl hover:border-primary/40 hover:bg-ice active:scale-[0.97]",
      ghost: "text-graphite rounded-xl hover:bg-surface active:bg-surface-warm",
      pill: "bg-surface text-ash rounded-full border border-border-lighter hover:bg-primary-soft hover:text-graphite hover:border-primary/30 active:scale-[0.97]",
      outline: "border border-border-soft bg-transparent rounded-2xl text-graphite hover:bg-surface hover:border-primary/40",
      link: "text-primary underline-offset-4 hover:underline p-0 h-auto",
    };

    const sizes: Record<string, string> = {
      default: "h-11 px-6 py-2.5 text-sm",
      sm: "h-8 px-4 text-xs",
      lg: "h-12 px-8 text-base",
      icon: "h-10 w-10 rounded-xl",
    };

    return (
      <button
        className={cn(baseStyles, variants[variant], sizes[size], className)}
        ref={ref}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

export { Button }
