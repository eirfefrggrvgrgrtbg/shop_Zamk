import * as React from "react"
import { cn } from "../../lib/utils"
import { Search } from "lucide-react"

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  icon?: React.ReactNode;
  isSearch?: boolean;
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, icon, isSearch, ...props }, ref) => {
    return (
      <div className="relative">
        {(icon || isSearch) && (
          <div className="absolute left-3.5 top-1/2 -translate-y-1/2 text-ash-light pointer-events-none">
            {icon || <Search className="w-4 h-4" />}
          </div>
        )}
        <input
          className={cn(
            "w-full h-11 rounded-2xl border border-border-soft bg-white px-4 py-2.5 text-sm text-graphite placeholder:text-ash-light",
            "focus:outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary/50",
            "transition-all duration-200",
            (icon || isSearch) && "pl-10",
            className
          )}
          ref={ref}
          {...props}
        />
      </div>
    )
  }
)
Input.displayName = "Input"

export { Input }
