import { useProductStudio } from "../../contexts/ProductStudioContext";
import { ProductStudioHeader } from "./ProductStudioHeader";
import { ProductStudioVisualWorkspace } from "./ProductStudioVisualWorkspace";
import { ProductStudioFormWorkspace } from "./ProductStudioFormWorkspace";

export function ProductStudio() {
  const { viewMode } = useProductStudio();

  return (
    <div data-testid="product-studio-root" className="w-full space-y-6">
      <ProductStudioHeader />

      <main className="w-full">
        {viewMode === "visual" ? (
          <ProductStudioVisualWorkspace />
        ) : (
          <ProductStudioFormWorkspace />
        )}
      </main>
    </div>
  );
}
