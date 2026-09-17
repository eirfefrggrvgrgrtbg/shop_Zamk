import React from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import { cn } from '../lib/utils';

export type SellerSurfaceVariant = 'primary' | 'secondary' | 'muted';

export interface SellerSurfaceProps extends React.HTMLAttributes<HTMLElement> {
  as?: 'div' | 'section' | 'article' | 'aside';
  variant?: SellerSurfaceVariant;
  children?: React.ReactNode;
  className?: string;
}

/**
 * Canonical Seller Surface Primitive:
 * - primary: clean white background, subtle border, rounded-xl (12-16px), no heavy shadow
 * - secondary / muted: light neutral background, subtle border
 */
export function SellerSurface({
  as: Component = 'div',
  variant = 'primary',
  children,
  className,
  ...props
}: SellerSurfaceProps) {
  return (
    <Component
      data-testid="seller-surface"
      className={cn(
        "rounded-xl border",
        variant === 'primary' && "bg-white dark:bg-white/[0.03] border-gray-200 dark:border-white/10",
        (variant === 'secondary' || variant === 'muted') && "bg-gray-50/60 dark:bg-white/[0.02] border-gray-200/80 dark:border-white/10",
        className
      )}
      {...props}
    >
      {children}
    </Component>
  );
}

export type SellerKpiAccent = 'neutral' | 'positive' | 'warning' | 'danger' | 'info';

export interface SellerKpiCardProps {
  label: React.ReactNode;
  value: React.ReactNode;
  supportText?: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  accent?: SellerKpiAccent;
  to?: string;
  onClick?: () => void;
  className?: string;
  children?: React.ReactNode;
  'data-testid'?: string;
}

/**
 * Canonical Seller KPI Tile Contract:
 * - label: muted, consistent weight
 * - value: dominant, high hierarchy
 * - support text: secondary
 * - icon: low-emphasis, quiet container
 * - accent: strictly semantic (positive/warning/danger/info), never purely decorative
 * - rounded-xl, restrained border, no heavy shadow
 */
export function SellerKpiCard({
  label,
  value,
  supportText,
  icon: Icon,
  accent = 'neutral',
  to,
  onClick,
  className,
  children,
  'data-testid': dataTestId = 'seller-kpi-card',
}: SellerKpiCardProps) {
  const isInteractive = Boolean(to || onClick);

  const valueAccentClasses: Record<SellerKpiAccent, string> = {
    neutral: 'text-gray-900 dark:text-white',
    positive: 'text-emerald-600 dark:text-emerald-400',
    warning: 'text-amber-600 dark:text-amber-400',
    danger: 'text-red-600 dark:text-red-400',
    info: 'text-blue-600 dark:text-blue-400',
  };

  const iconAccentClasses: Record<SellerKpiAccent, string> = {
    neutral: 'text-gray-400 dark:text-gray-500 bg-gray-50 dark:bg-white/5',
    positive: 'text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/30',
    warning: 'text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/30',
    danger: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/30',
    info: 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-950/30',
  };

  const borderInteractiveClasses = isInteractive
    ? accent === 'danger'
      ? 'border-red-200 hover:border-red-300 dark:border-red-900/40 dark:hover:border-red-800/60'
      : 'border-gray-200 hover:border-gray-300 dark:border-white/10 dark:hover:border-white/20'
    : accent === 'danger'
    ? 'border-red-200 dark:border-red-900/30'
    : 'border-gray-200 dark:border-white/10';

  const content = (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p
            data-testid="seller-kpi-label"
            className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate"
          >
            {label}
          </p>
          <div
            data-testid="seller-kpi-value"
            className={cn(
              "mt-2 text-2xl sm:text-3xl font-bold tracking-tight",
              valueAccentClasses[accent]
            )}
          >
            {value}
          </div>
        </div>
        {Icon && (
          <span
            data-testid="seller-kpi-icon"
            className={cn(
              "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors",
              iconAccentClasses[accent]
            )}
          >
            <Icon className="h-4 w-4" />
          </span>
        )}
      </div>

      {(supportText || isInteractive) && (
        <div className="mt-3 flex items-center justify-between text-xs sm:text-sm text-gray-500 dark:text-gray-400">
          <span data-testid="seller-kpi-support" className="truncate">
            {supportText}
          </span>
          {isInteractive && (
            <ChevronRight className="h-4 w-4 text-gray-400 shrink-0 opacity-0 transition-all group-hover:opacity-100 group-hover:translate-x-0.5" />
          )}
        </div>
      )}

      {children}
    </>
  );

  const sharedClasses = cn(
    "group rounded-xl border bg-white dark:bg-white/[0.03] p-5 transition-all text-left flex flex-col justify-between",
    borderInteractiveClasses,
    className
  );

  if (to) {
    return (
      <Link to={to} data-testid={dataTestId} className={sharedClasses}>
        {content}
      </Link>
    );
  }

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        data-testid={dataTestId}
        className={sharedClasses}
      >
        {content}
      </button>
    );
  }

  return (
    <div data-testid={dataTestId} className={sharedClasses}>
      {content}
    </div>
  );
}

export interface SellerTableShellProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  className?: string;
}

/**
 * Canonical Seller Table Container Primitive:
 * - white operational surface
 * - restrained neutral border
 * - rounded-xl (12-16px) with overflow-hidden
 * - no heavy decorative shadow
 */
export function SellerTableShell({
  children,
  className,
  ...props
}: SellerTableShellProps) {
  return (
    <div
      data-testid="seller-table-shell"
      className={cn(
        "rounded-xl border border-gray-200 dark:border-white/10 bg-white dark:bg-white/[0.03] overflow-hidden",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export interface SellerFilterBarProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  className?: string;
}

/**
 * Canonical Seller Filter Bar Primitive:
 * - compact padding
 * - visually flush/integrated with table shell
 * - no floating glow card
 */
export function SellerFilterBar({
  children,
  className,
  ...props
}: SellerFilterBarProps) {
  return (
    <div
      data-testid="seller-filter-bar"
      className={cn(
        "p-3 sm:p-4 border-b border-gray-200 dark:border-white/10 bg-gray-50/50 dark:bg-white/[0.02] flex flex-col sm:flex-row justify-between items-center gap-3 sm:gap-4",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export interface SellerDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  title?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  'data-testid'?: string;
}

/**
 * Canonical Seller Drawer / Sheet Primitive:
 * - Floating surface overlaying the page (does not push or compress content underneath)
 * - Backdrop blur / tint with click-to-close
 * - Explicit close button & Escape key handling
 * - Clean semantic elevation shadow (shadow-xl)
 * - Responsive desktop width ~440-480px, nearly full-width on mobile
 * - Accessible z-index (z-50) above page content and sticky nav (z-30)
 */
export function SellerDrawer({
  isOpen,
  onClose,
  title,
  children,
  className,
  'data-testid': dataTestId = 'seller-drawer',
}: SellerDrawerProps) {
  React.useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-hidden" data-testid={dataTestId}>
      {/* Backdrop */}
      <div
        data-testid="seller-drawer-backdrop"
        className="fixed inset-0 bg-black/40 backdrop-blur-xs transition-opacity animate-in fade-in duration-200"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Drawer Container */}
      <div className="fixed inset-y-0 right-0 max-w-full flex pl-10">
        <div
          role="dialog"
          aria-modal="true"
          aria-label={typeof title === 'string' ? title : 'Детали'}
          className={cn(
            "w-screen max-w-md sm:max-w-lg bg-white dark:bg-gray-900 shadow-2xl border-l border-gray-200 dark:border-white/10 flex flex-col transition-transform animate-in slide-in-from-right duration-200",
            className
          )}
        >
          {/* Header */}
          <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-white/10 bg-white dark:bg-gray-900 shrink-0">
            <div className="min-w-0 flex-1">
              {typeof title === 'string' ? (
                <h3 className="text-base font-semibold text-gray-900 dark:text-white truncate">
                  {title}
                </h3>
              ) : (
                title
              )}
            </div>
            <button
              type="button"
              onClick={onClose}
              data-testid="seller-drawer-close"
              aria-label="Закрыть"
              className="p-1.5 -mr-1.5 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-white/10 dark:hover:text-white transition-colors cursor-pointer"
            >
              <svg
                aria-hidden="true"
                className="h-5 w-5"
                fill="none"
                height="24"
                stroke="currentColor"
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
                viewBox="0 0 24 24"
                width="24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path d="M18 6 6 18" />
                <path d="m6 6 12 12" />
              </svg>
            </button>
          </div>

          {/* Body */}
          <div className="flex-1 overflow-y-auto p-5 sm:p-6">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
