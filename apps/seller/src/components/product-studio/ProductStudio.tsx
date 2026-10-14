import { useProductStudio } from "../../contexts/ProductStudioContext";
import { ProductStudioHeader } from "./ProductStudioHeader";
import { ProductStudioVisualWorkspace } from "./ProductStudioVisualWorkspace";
import { ProductStudioFormWorkspace } from "./ProductStudioFormWorkspace";

export function ProductStudio() {
  const { viewMode, isSubmittingModeration } = useProductStudio();

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
          <ProductStudioFormWorkspace />
        )}
      </div>
    </div>
  );
}
