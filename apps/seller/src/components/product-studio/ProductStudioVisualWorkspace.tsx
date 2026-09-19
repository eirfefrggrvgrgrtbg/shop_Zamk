import { useState, useMemo } from "react";
import { ProductPresentationCore } from "@zamk/shared";
import { useProductStudio } from "../../contexts/ProductStudioContext";
import {
  mapStudioDraftToPresentation,
  computePresentationSizes,
  findMatchingDraftVariant,
  mapToPresentationSelectedVariant,
  findFirstMediaIndexForColor,
} from "./productStudioPresentationAdapter";

export function ProductStudioVisualWorkspace() {
  const { draft } = useProductStudio();

  // Local preview selection state (purely Visual-local, not persisted, no router)
  const [selectedColorId, setSelectedColorId] = useState<string | null>(null);
  const [selectedSizeId, setSelectedSizeId] = useState<string | null>(null);
  const [activeImage, setActiveImage] = useState<number>(0);

  // Map draft to normalized presentation models
  const {
    product,
    visibleImages,
    colors,
    dimensionType,
    uniqueSizes,
    basePrice,
    hasVariants,
  } = useMemo(() => mapStudioDraftToPresentation(draft), [draft]);

  // Derive presentation sizes given current preview color selection
  const sizes = useMemo(
    () => computePresentationSizes(draft, dimensionType, uniqueSizes, selectedColorId),
    [draft, dimensionType, uniqueSizes, selectedColorId]
  );

  // Derive selected objects and matched variant
  const selectedColor = useMemo(
    () => (selectedColorId ? colors.find((c) => c.id === selectedColorId) || null : null),
    [colors, selectedColorId]
  );

  const selectedSize = useMemo(
    () => (selectedSizeId ? sizes.find((s) => s.id === selectedSizeId) || null : null),
    [sizes, selectedSizeId]
  );

  const matchingVariant = useMemo(
    () => findMatchingDraftVariant(draft, dimensionType, selectedColorId, selectedSizeId),
    [draft, dimensionType, selectedColorId, selectedSizeId]
  );

  const selectedVariant = useMemo(
    () => mapToPresentationSelectedVariant(matchingVariant),
    [matchingVariant]
  );

  // Display price: variant price if available, otherwise base draft price
  const displayPrice =
    selectedVariant?.priceCents !== undefined
      ? selectedVariant.priceCents / 100
      : basePrice;

  // Derive resolution & required flags
  const requiresColor = dimensionType === "COLOR_AND_SIZE" || dimensionType === "COLOR_ONLY";
  const requiresSize = dimensionType === "COLOR_AND_SIZE" || dimensionType === "SIZE_ONLY";

  const isResolved = useMemo(() => {
    if (!hasVariants) return false;
    switch (dimensionType) {
      case "COLOR_AND_SIZE":
        return Boolean(selectedColorId && selectedSizeId);
      case "COLOR_ONLY":
        return Boolean(selectedColorId);
      case "SIZE_ONLY":
        return Boolean(selectedSizeId);
      case "SINGLE_VARIANT":
        return true;
    }
  }, [dimensionType, selectedColorId, selectedSizeId, hasVariants]);

  // Derive CTA text
  const ctaText = useMemo(() => {
    if (!hasVariants) return "Добавить в корзину";
    if (dimensionType === "COLOR_AND_SIZE") {
      if (!selectedColorId) return "Выберите цвет";
      if (!selectedSizeId) return "Выберите размер";
      return "Добавить в корзину";
    }
    if (dimensionType === "COLOR_ONLY") {
      if (!selectedColorId) return "Выберите цвет";
      return "Добавить в корзину";
    }
    if (dimensionType === "SIZE_ONLY") {
      if (!selectedSizeId) return "Выберите размер";
      return "Добавить в корзину";
    }
    return "Добавить в корзину";
  }, [dimensionType, selectedColorId, selectedSizeId, hasVariants]);

  const sizeSelectionNotice =
    dimensionType === "COLOR_AND_SIZE" && !selectedColorId
      ? "Сначала выберите цвет"
      : null;

  // Handlers for local preview interactions
  const handleColorChange = (colorId: string) => {
    setSelectedColorId(colorId);

    // If a size was already selected, check if it is offered for the new color
    if (selectedSizeId) {
      const isOffered = (draft.variants || []).some(
        (v) => v.colorId === colorId && v.sizeValueId === selectedSizeId
      );
      if (!isOffered) {
        setSelectedSizeId(null);
      }
    }

    // Focus matching media in gallery
    const targetIdx = findFirstMediaIndexForColor(visibleImages, colorId);
    setActiveImage(targetIdx);
  };

  const handleSizeChange = (sizeId: string) => {
    setSelectedSizeId(sizeId);
  };

  const handleActiveImageChange = (index: number) => {
    setActiveImage(index);
  };

  const safeActiveImage = activeImage < visibleImages.length ? activeImage : 0;

  return (
    <div
      id="studio-workspace-visual"
      role="tabpanel"
      aria-labelledby="studio-tab-visual"
      data-testid="studio-visual-workspace"
      className="w-full"
    >
      <div className="max-w-[1360px] mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <ProductPresentationCore
          product={product}
          visibleImages={visibleImages}
          activeImage={safeActiveImage}
          onActiveImageChange={handleActiveImageChange}
          displayPrice={displayPrice}
          colors={colors}
          sizes={sizes}
          selectedColorId={selectedColorId}
          selectedSizeId={selectedSizeId}
          selectedColor={selectedColor}
          selectedSize={selectedSize}
          selectedVariant={selectedVariant}
          isResolved={isResolved}
          canAddToCart={isResolved}
          requiresColor={requiresColor}
          requiresSize={requiresSize}
          isAddingToCart={false}
          isProductUnavailable={false}
          ctaText={ctaText}
          sizeSelectionNotice={sizeSelectionNotice}
          refreshErrorNotice={null}
          sizeError=""
          onColorChange={handleColorChange}
          onSizeChange={handleSizeChange}
          onAddToCart={() => {
            // Preview only: NO cart side effect, NO network request, NO toast
          }}
          isFavorite={false}
          onToggleFavorite={() => {
            // Preview only: NO favorite side effect
          }}
          onScrollToReviews={() => {
            // Preview only: NO review navigation
          }}
          onBrandClick={() => {
            // Preview only: NO navigation
          }}
          onDeliveryClick={() => {
            // Preview only: NO navigation
          }}
          onReturnsClick={() => {
            // Preview only: NO navigation
          }}
          onSellerClick={() => {
            // Preview only: NO navigation
          }}
        />
      </div>
    </div>
  );
}
