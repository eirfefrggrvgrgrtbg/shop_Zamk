import { useEffect, useRef } from 'react';
import {
  MoreHorizontal,
  Edit2,
  Trash2,
  Archive,
  ExternalLink,
  Eye,
  AlertTriangle,
} from 'lucide-react';
import type { SellerProduct } from '../lib/seller-products';
import { cn } from '../lib/utils';

export interface ProductActionMenuProps {
  product: SellerProduct;
  sellerStatus: string;
  isOpen: boolean;
  onToggle: () => void;
  onClose: () => void;
  onOpenEdit?: (product: SellerProduct) => void;
  onOpenDrawer?: (product: SellerProduct) => void;
  onRequestDelete?: (product: SellerProduct) => void;
  onRequestArchive?: (product: SellerProduct) => void;
  variant?: 'row' | 'drawer';
  align?: 'left' | 'right';
  className?: string;
}

export function ProductActionMenu({
  product,
  sellerStatus,
  isOpen,
  onToggle,
  onClose,
  onOpenEdit,
  onOpenDrawer,
  onRequestDelete,
  onRequestArchive,
  variant = 'row',
  align = 'right',
  className,
}: ProductActionMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;

    const handlePointerDown = (event: MouseEvent | TouchEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onClose();
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose]);

  const isSellerActive = sellerStatus !== 'blocked' && sellerStatus !== 'archived';

  // Determine which actions are available according to Section 4
  const canDelete = isSellerActive && product.status === 'draft';
  const canArchive = isSellerActive && [
    'draft',
    'rejected',
    'approved',
    'out_of_stock',
  ].includes(product.status);
  const canStopSelling = isSellerActive && product.status === 'published';

  const hasStorefrontLink = product.status === 'published' && Boolean(product.storefrontUrl && product.storefrontUrl.trim());

  const handleAction = (callback?: (p: SellerProduct) => void) => {
    onClose();
    if (callback) {
      callback(product);
    }
  };

  return (
    <div
      ref={menuRef}
      className={cn('relative inline-block text-left', className)}
      onClick={(e) => e.stopPropagation()}
    >
      <button
        type="button"
        aria-label="Действия"
        aria-expanded={isOpen}
        data-testid={variant === 'drawer' ? 'drawer-actions-menu-btn' : `product-actions-menu-btn-${product.id}`}
        onClick={(e) => {
          e.stopPropagation();
          onToggle();
        }}
        className={cn(
          "inline-flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 hover:text-gray-900 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-white dark:hover:bg-white/10 transition-colors cursor-pointer",
          isOpen && "bg-gray-100 dark:bg-white/10 text-gray-900 dark:text-white"
        )}
      >
        <MoreHorizontal className="h-4 w-4" />
      </button>

      {isOpen && (
        <div
          role="menu"
          aria-label="Меню действий"
          data-testid={variant === 'drawer' ? 'drawer-actions-menu' : `product-actions-menu-${product.id}`}
          className={cn(
            "absolute z-40 mt-1 min-w-[210px] rounded-xl border border-gray-200 dark:border-white/10 bg-white dark:bg-gray-900 p-1.5 shadow-xl transition-all animate-in fade-in zoom-in-95 duration-100",
            align === 'right' ? 'right-0' : 'left-0'
          )}
        >
          {/* Navigation / Edit Actions */}
          {variant === 'row' && (
            <>
              {product.status === 'draft' ? (
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => handleAction(onOpenEdit)}
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
                >
                  <Edit2 className="h-4 w-4 text-gray-500" />
                  Редактировать
                </button>
              ) : product.status === 'rejected' ? (
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => handleAction(onOpenEdit)}
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
                >
                  <AlertTriangle className="h-4 w-4 text-red-500" />
                  Исправить карточку
                </button>
              ) : product.status === 'published' ? (
                <>
                  {hasStorefrontLink && (
                    <a
                      href={product.storefrontUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      role="menuitem"
                      onClick={() => onClose()}
                      className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
                    >
                      <ExternalLink className="h-4 w-4 text-gray-500" />
                      Открыть в магазине
                    </a>
                  )}
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => handleAction(onOpenEdit)}
                    className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
                  >
                    <Edit2 className="h-4 w-4 text-gray-500" />
                    Редактировать карточку
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  role="menuitem"
                  onClick={() => handleAction(onOpenDrawer || onOpenEdit)}
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
                >
                  <Eye className="h-4 w-4 text-gray-500" />
                  Открыть карточку
                </button>
              )}
            </>
          )}

          {/* Lifecycle Mutation: Archive / Stop Selling */}
          {canStopSelling && (
            <button
              type="button"
              role="menuitem"
              onClick={() => handleAction(onRequestArchive)}
              className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
            >
              <Archive className="h-4 w-4 text-gray-500" />
              Снять с продажи
            </button>
          )}

          {canArchive && (
            <button
              type="button"
              role="menuitem"
              onClick={() => handleAction(onRequestArchive)}
              className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors cursor-pointer text-left"
            >
              <Archive className="h-4 w-4 text-gray-500" />
              В архив
            </button>
          )}

          {/* Lifecycle Mutation: Safe Hard Delete (Draft only) */}
          {canDelete && (
            <button
              type="button"
              role="menuitem"
              onClick={() => handleAction(onRequestDelete)}
              className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/30 transition-colors cursor-pointer text-left font-medium"
            >
              <Trash2 className="h-4 w-4 text-red-600 dark:text-red-400" />
              Удалить черновик
            </button>
          )}
        </div>
      )}
    </div>
  );
}
