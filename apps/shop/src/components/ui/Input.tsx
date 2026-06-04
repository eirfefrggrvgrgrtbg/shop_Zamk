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
          "w-full h-12 bg-white dark:bg-white/10 border border-border-soft dark:border-white/20 text-graphite dark:text-white placeholder:text-ash-light dark:placeholder:text-white/50 shadow-sm transition-all focus:outline-none focus:ring-1 focus:ring-graphite/20 dark:focus:ring-white/30 focus:border-graphite/40 dark:focus:border-white/40",
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
