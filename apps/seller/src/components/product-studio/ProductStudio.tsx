import { useProductStudio } from "../../contexts/ProductStudioContext";
import { ProductStudioHeader } from "./ProductStudioHeader";
import { ProductStudioVisualWorkspace } from "./ProductStudioVisualWorkspace";
import { ProductStudioFormWorkspace } from "./ProductStudioFormWorkspace";

export function ProductStudio() {
  const { viewMode, isSaveInFlight } = useProductStudio();

  return (
    <div data-testid="product-studio-root" className="w-full">
      <ProductStudioHeader />

      <div
        data-testid="studio-workspace-container"
        aria-disabled={isSaveInFlight}
        className={
          isSaveInFlight
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
