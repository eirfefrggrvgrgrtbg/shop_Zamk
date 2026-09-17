import React from 'react';
import { cn } from '../lib/utils';

export type SellerPageVariant = 'summary' | 'wide' | 'form' | 'default';

export interface SellerPageFrameProps {
  children: React.ReactNode;
  variant?: SellerPageVariant;
  className?: string;
  containerClassName?: string;
}

/**
 * Canonical Seller Page Canvas:
 * All normal Seller pages share the SAME canonical outer content canvas:
 * - w-full
 * - max-w-[1296px]
 * - mx-auto
 * - px-4 sm:px-6 lg:px-8
 * - pt-6 pb-10
 *
 * Page types differ strictly by internal layout/grid composition,
 * NOT by changing the width of the outer page canvas.
 */
export function SellerPageFrame({
  children,
  variant = 'default',
  className,
  containerClassName,
}: SellerPageFrameProps) {
  return (
    <main
      data-testid="seller-page-frame"
      data-variant={variant}
      className={cn(
        "w-full max-w-[1296px] mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-10",
        className
      )}
    >
      <div
        data-testid="seller-page-canvas-container"
        className={cn("w-full space-y-6", containerClassName)}
      >
        {children}
      </div>
    </main>
  );
}

/**
 * Shared 12-column grid foundation for internal page-family composition.
 */
export interface SellerGridProps {
  children: React.ReactNode;
  className?: string;
}

export function SellerGrid({ children, className }: SellerGridProps) {
  return (
    <div
      data-testid="seller-grid"
      className={cn("grid grid-cols-1 lg:grid-cols-12 gap-6 lg:gap-8", className)}
    >
      {children}
    </div>
  );
}

export interface SellerPageHeaderProps {
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}

export function SellerPageHeader({
  eyebrow,
  title,
  description,
  action,
  className,
}: SellerPageHeaderProps) {
  return (
    <header
      data-testid="seller-page-header"
      className={cn("flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between", className)}
    >
      <div className="space-y-1">
        {eyebrow && (
          <p
            data-testid="seller-page-header-eyebrow"
            className="text-xs font-semibold uppercase tracking-wider text-gray-400 select-none"
          >
            {eyebrow}
          </p>
        )}
        <h1
          data-testid="seller-page-header-title"
          className="text-2xl font-bold tracking-tight text-gray-900 sm:text-3xl"
        >
          {title}
        </h1>
        {description && (
          <p
            data-testid="seller-page-header-description"
            className="text-sm text-gray-500 sm:text-base max-w-3xl"
          >
            {description}
          </p>
        )}
      </div>
      {action && (
        <div data-testid="seller-page-header-action" className="flex shrink-0 items-center gap-3 pt-1">
          {action}
        </div>
      )}
    </header>
  );
}
