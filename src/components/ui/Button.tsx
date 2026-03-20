import * as React from "react"
import { cn } from "../../lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "outline" | "ghost" | "link";
  size?: "default" | "sm" | "lg" | "icon";
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", ...props }, ref) => {
    
    const baseStyles = "inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-all focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-dusty-blue disabled:pointer-events-none disabled:opacity-50";
    
    const variants = {
      default: "bg-graphite text-milk shadow hover:bg-graphite/90",
      outline: "border border-border-soft bg-transparent hover:bg-slate-100 hover:text-graphite",
      ghost: "hover:bg-slate-100 hover:text-graphite",
      link: "text-graphite underline-offset-4 hover:underline",
    };
    
    const sizes = {
      default: "h-11 px-8 py-2",
      sm: "h-8 rounded-md px-4 text-xs",
      lg: "h-12 rounded-md px-10",
      icon: "h-9 w-9",
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
