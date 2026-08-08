import { useState } from "react"
import { cn } from "../../lib/utils"
import { Search, Eye, EyeOff } from "lucide-react"

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  isSearch?: boolean;
}

function Input({ className, isSearch, type, ...props }: InputProps) {
  const [showPassword, setShowPassword] = useState(false);
  const isPasswordType = type === 'password';
  const inputType = isPasswordType && showPassword ? 'text' : type;

  return (
    <div className="relative w-full">
      {isSearch && (
        <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-ash" />
      )}
      <input
        type={inputType}
        className={cn(
          "w-full h-12 bg-white dark:bg-white/10 border border-border-soft dark:border-white/20 text-graphite dark:text-white placeholder:text-ash-light dark:placeholder:text-white/50 shadow-sm transition-all focus:outline-none focus:ring-1 focus:ring-graphite/20 dark:focus:ring-white/30 focus:border-graphite/40 dark:focus:border-white/40",
          "rounded-[18px]",
          isSearch ? "pl-11 pr-5" : "px-5",
          isPasswordType ? "pr-12" : "",
          className
        )}
        {...props}
      />
      {isPasswordType && (
        <button
          type="button"
          onClick={() => setShowPassword(!showPassword)}
          className="absolute right-4 top-1/2 -translate-y-1/2 text-ash hover:text-graphite dark:hover:text-white focus:outline-none"
          aria-label={showPassword ? "Скрыть пароль" : "Показать пароль"}
        >
          {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
        </button>
      )}
    </div>
  )
}

export { Input }
