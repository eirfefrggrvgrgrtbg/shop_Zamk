import { useEffect } from "react";
import { useProductStudio, type ProductStudioVariant } from "../../contexts/ProductStudioContext";
import { ProductStudioHeader } from "./ProductStudioHeader";
import { ProductStudioVisualWorkspace } from "./ProductStudioVisualWorkspace";
import { ProductStudioFormWorkspace } from "./ProductStudioFormWorkspace";

export function ProductStudio({
  focusVariantId,
  focusVariantStored,
}: { focusVariantId?: string; focusVariantStored?: ProductStudioVariant } = {}) {
  const { viewMode, isSubmittingModeration, setViewMode, setActiveSection } = useProductStudio();

  // A link from the product list to one variant lands on the variants section of the form.
  useEffect(() => {
    if (!focusVariantId) return;
    setViewMode("form");
    setActiveSection("variants");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusVariantId]);

  // In autosave (both create and edit), do NOT aggressively freeze the form (seller can continue typing).
  // Freeze only during explicit moderation submit.
  const isLocked = isSubmittingModeration;

  return (
    <div data-testid="product-studio-root" className="w-full">
      <ProductStudioHeader />

      <div
        data-testid="studio-workspace-container"
        aria-disabled={isLocked}
        className={
          isLocked
            ? "w-full pointer-events-none opacity-60 select-none cursor-wait"
            : "w-full"
        }
      >
        {viewMode === "visual" ? (
          <ProductStudioVisualWorkspace />
        ) : (
          <ProductStudioFormWorkspace focusVariantId={focusVariantId} focusVariantStored={focusVariantStored} />
        )}
      </div>
    </div>
  );
}
