import { cn } from "../../lib/utils"
import { Search } from "lucide-react"

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  isSearch?: boolean;
}

function Input({ className, isSearch, ...props }: InputProps) {
  return (
    <div className="relative w-full">
      {isSearch && (
        <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-ash" />
      )}
      <input
        className={cn(
          "w-full h-12 bg-white border border-border-soft text-graphite placeholder:text-ash-light shadow-sm transition-all focus:outline-none focus:ring-1 focus:ring-graphite/20 focus:border-graphite/40",
          "rounded-[18px]",
          isSearch ? "pl-11 pr-5" : "px-5",
          className
        )}
        {...props}
      />
    </div>
  )
}

export { Input }
