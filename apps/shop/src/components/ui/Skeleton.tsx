import { cn } from "../../lib/utils"

interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'circle' | 'text';
}

function Skeleton({ className, variant = 'default', ...props }: SkeletonProps) {
  const variants: Record<string, string> = {
    default: 'rounded-2xl',
    circle: 'rounded-full',
    text: 'rounded-lg h-4',
  };

  return (
    <div
      className={cn('shimmer-bg bg-white/60', variants[variant], className)}
      {...props}
    />
  )
}

function ProductCardSkeleton() {
  return (
    <div className="capsule rounded-2xl overflow-hidden">
      <div className="aspect-[4/5] m-2 rounded-xl">
        <Skeleton className="w-full h-full rounded-xl" />
      </div>
      <div className="px-4 pb-4 pt-2 space-y-2">
        <div className="flex justify-between">
          <Skeleton variant="text" className="w-20" />
          <Skeleton variant="text" className="w-16" />
        </div>
        <Skeleton variant="text" className="w-3/4" />
      </div>
    </div>
  );
}

export { Skeleton, ProductCardSkeleton }
