import { useState, useMemo, useRef, useEffect } from "react";
import { ProductPresentationCore, type ProductPresentationMediaItem } from "@zamk/shared";
import {
  getSellerColors,
  getSellerCategorySchema,
  getSellerSizeValues,
  type SellerColor,
  type SellerSizeValue,
  type SellerSizeSystem,
  type SellerCategorySchema,
} from "@zamk/api-client";
import { Info, Link2, AlertTriangle } from "lucide-react";
import { useProductStudio } from "../../contexts/ProductStudioContext";
import { cn } from "../../lib/utils";
import { ProductStudioPhotoColorModal, getColorDisplayName } from "./ProductStudioPhotoColorModal";
import { ProductStudioCompositionModal } from "./ProductStudioCompositionModal";
import { ProductStudioCareModal } from "./ProductStudioCareModal";
import { ProductStudioCharacteristicsModal } from "./ProductStudioCharacteristicsModal";
import { ProductStudioSizeChartModal } from "./ProductStudioSizeChartModal";
import {
  mapStudioDraftToPresentation,
  computePresentationSizes,
  computePresentationColors,
  findMatchingDraftVariant,
  mapToPresentationSelectedVariant,
  findFirstMediaIndexForColor,
  getCanonicalSizeLabel,
} from "./productStudioPresentationAdapter";
import {
  reconcileProductStudioVariantMatrix,
  resolveProductStudioDimensionType,
} from "./productStudioMatrixHelper";
import {
  validateImageFile,
  getMediaProgressText,
  createLocalProductStudioImage,
  deriveProductStudioMediaMode,
  transitionMediaToColorway,
  transitionMediaToGeneral,
  resolveLegacyMixedMedia,
  reconcileMediaOnColorRemoval,
  isImageUnassigned,
  getProductStudioImageDisplayUrl,
  type ProductStudioMediaMode,
  MAX_PRODUCT_IMAGES,
  ALLOWED_IMAGE_MIME_TYPES,
  resolveInitialMediaColorId,
  reorderProductStudioImages,
  normalizeProductStudioCovers,
  getMediaReadinessWarning,
} from "./productStudioMediaHelper";
import {
  getSizeChartCompleteness,
  getOfferedSizes,
  getCompositionCompleteness,
  getCanonicalRequiredProductAttributes,
} from "./productStudioReadinessHelper";

export function ProductStudioVisualWorkspace() {
  const {
    draft,
    updateDraft,
    readiness,
    createMediaUrl,
    setCategoryModalOpen,
    markTouched,
    isFieldAttention,
    activeSizeSystemId,
    setActiveSizeSystemId,
    selectedPreviewColorId: selectedColorId,
    selectedPreviewSizeValueId: selectedSizeId,
    setSelectedPreviewColorId: setSelectedColorId,
    setSelectedPreviewSizeValueId: setSelectedSizeId,
    selectedMediaColorId,
    setSelectedMediaColorId,
    categorySchema: contextCategorySchema,
  } = useProductStudio();

  const workspaceRef = useRef<HTMLDivElement>(null);

  const [activeImage, setActiveImage] = useState<number>(0);

  // Popover state
  const [isColorPopoverOpen, setIsColorPopoverOpen] = useState(false);
  const [pendingSelectedColorIds, setPendingSelectedColorIds] = useState<Set<string>>(new Set());

  const [isSizePopoverOpen, setIsSizePopoverOpen] = useState(false);
  const [pendingSelectedSizeIds, setPendingSelectedSizeIds] = useState<Set<string>>(new Set());

  // In-place editing state
  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [titleValue, setTitleValue] = useState("");

  const [isEditingPrice, setIsEditingPrice] = useState(false);
  const [priceValue, setPriceValue] = useState("");

  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [descValue, setDescValue] = useState("");

  // Modals state
  const [isPhotoColorModalOpen, setIsPhotoColorModalOpen] = useState(false);
  const [isConfirmToColorwayOpen, setIsConfirmToColorwayOpen] = useState(false);
  const [isConfirmToGeneralOpen, setIsConfirmToGeneralOpen] = useState(false);
  const [isCompositionModalOpen, setIsCompositionModalOpen] = useState(false);
  const [isCareModalOpen, setIsCareModalOpen] = useState(false);
  const [isCharacteristicsModalOpen, setIsCharacteristicsModalOpen] = useState(false);
  const [isSizeChartModalOpen, setIsSizeChartModalOpen] = useState(false);
  const [showMediaInfo, setShowMediaInfo] = useState(false);

  // Reference dictionaries
  const [localCategorySchema, setLocalCategorySchema] = useState<SellerCategorySchema | null>(null);
  const categorySchema = contextCategorySchema || localCategorySchema;
  const [colorsList, setColorsList] = useState<SellerColor[]>([]);
  const [allowedSizeSystems, setAllowedSizeSystems] = useState<SellerSizeSystem[]>([]);
  const selectedSizeSystemId = activeSizeSystemId;
  const setSelectedSizeSystemId = setActiveSizeSystemId;
  const [sizeValuesList, setSizeValuesList] = useState<SellerSizeValue[]>([]);
  const [loadingSizes, setLoadingSizes] = useState(false);
  const [mediaError, setMediaError] = useState<string | null>(null);
  const [isValidatingPhoto, setIsValidatingPhoto] = useState(false);

  // Load colors once
  useEffect(() => {
    getSellerColors()
      .then((data) => setColorsList(data || []))
      .catch((err) => console.error("Failed to load seller colors:", err));
  }, []);

  // Load category schema and allowed size systems when categoryId changes
  useEffect(() => {
    if (!draft.categoryId) {
      setSizeValuesList([]);
      setAllowedSizeSystems([]);
      setSelectedSizeSystemId(null);
      setLocalCategorySchema(null);
      return;
    }

    let isMounted = true;
    setLoadingSizes(true);

    getSellerCategorySchema(draft.categoryId)
      .then((schema) => {
        if (!isMounted) return;
        setLocalCategorySchema(schema);
        const allowed = schema.allowedSizeSystems || [];
        setAllowedSizeSystems(allowed);
        if (allowed.length > 0) {
          if (!activeSizeSystemId) {
            const defaultSys = allowed.find((s: any) => s.isDefault) || allowed[0];
            setSelectedSizeSystemId(defaultSys.id);
          }
        } else {
          setSelectedSizeSystemId(null);
          setSizeValuesList([]);
        }
      })
      .catch((err) => {
        console.error("Failed to load category schema:", err);
      })
      .finally(() => {
        if (isMounted) setLoadingSizes(false);
      });

    return () => {
      isMounted = false;
    };
  }, [draft.categoryId, activeSizeSystemId, setSelectedSizeSystemId]);

  // Load size values when selectedSizeSystemId changes
  useEffect(() => {
    if (!selectedSizeSystemId) {
      setSizeValuesList([]);
      return;
    }

    let isMounted = true;
    setLoadingSizes(true);

    getSellerSizeValues(selectedSizeSystemId)
      .then((values) => {
        if (isMounted) {
          setSizeValuesList(values || []);
        }
      })
      .catch((err) => {
        console.error("Failed to load size values:", err);
      })
      .finally(() => {
        if (isMounted) setLoadingSizes(false);
      });

    return () => {
      isMounted = false;
    };
  }, [selectedSizeSystemId]);

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

  const canonicalUniqueSizes = useMemo(() => {
    return uniqueSizes.map((s) => {
      const canonical = getCanonicalSizeLabel(s.id, sizeValuesList, s.label);
      return {
        id: s.id,
        label: canonical !== 'Размер недоступен' ? canonical : s.label,
      };
    });
  }, [uniqueSizes, sizeValuesList]);

  // Derive presentation colors and sizes with bidirectional filtering
  const visibleColors = useMemo(
    () => computePresentationColors(draft, colors, selectedSizeId),
    [draft, colors, selectedSizeId]
  );

  const sizes = useMemo(
    () => computePresentationSizes(draft, dimensionType, canonicalUniqueSizes, selectedColorId),
    [draft, dimensionType, canonicalUniqueSizes, selectedColorId]
  );

  // Derive selected objects and matched variant
  const selectedColor = useMemo(
    () => (selectedColorId ? colors.find((c) => c.id === selectedColorId) || null : null),
    [colors, selectedColorId]
  );

  const selectedSize = useMemo(() => {
    if (!selectedSizeId) return null;
    const found = sizes.find((s) => s.id === selectedSizeId);
    if (found) return found;
    const fallback = canonicalUniqueSizes.find((s) => s.id === selectedSizeId);
    if (fallback) {
      return {
        id: fallback.id,
        label: fallback.label,
        state: "AVAILABLE" as const,
        disabled: false,
      };
    }
    return null;
  }, [sizes, canonicalUniqueSizes, selectedSizeId]);

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
  const effectiveDimensionType = useMemo(
    () => resolveProductStudioDimensionType(draft.dimensionType, categorySchema?.dimensionType),
    [draft.dimensionType, categorySchema?.dimensionType]
  );

  const isExplicitOnlySize =
    effectiveDimensionType === "SIZE_ONLY" ||
    draft.dimensionType === "ONLY_SIZE" ||
    draft.dimensionType === "SIZE_ONLY";
  const isExplicitOnlyColor =
    effectiveDimensionType === "COLOR_ONLY" ||
    draft.dimensionType === "ONLY_COLOR" ||
    draft.dimensionType === "COLOR_ONLY";
  const isExplicitSingleVariant =
    effectiveDimensionType === "SINGLE_VARIANT" ||
    draft.dimensionType === "SINGLE_VARIANT";

  const requiresColor = !isExplicitOnlySize && !isExplicitSingleVariant;
  const requiresSize = !isExplicitOnlyColor && !isExplicitSingleVariant;

  const mediaMode: ProductStudioMediaMode = draft.mediaMode || deriveProductStudioMediaMode(draft.images);
  const unassignedImagesCount = useMemo(
    () => (draft.images || []).filter((img) => isImageUnassigned(img, mediaMode)).length,
    [draft.images, mediaMode]
  );

  const activeProductColors = useMemo<Array<{ id: string; name?: string; nameRu?: string; hex?: string; hexValue?: string }>>(() => {
    if (colors && colors.length > 0) {
      return colors;
    }
    return (draft.colors || []).filter((c: any) => Boolean(c.id));
  }, [colors, draft.colors]);

  const colorPhotoCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const img of draft.images || []) {
      if (img.colorId && !img.isUnassigned) {
        counts[img.colorId] = (counts[img.colorId] || 0) + 1;
      }
    }
    return counts;
  }, [draft.images]);

  const hasOrphanedImages = useMemo(() => {
    return (draft.images || []).some(
      (img) => Boolean(img.isUnassigned) || (Boolean(img.colorId) && !activeProductColors.some((c) => c.id === img.colorId))
    );
  }, [draft.images, activeProductColors]);

  // Selected media color resolution logic (Rules 1-5 + UNASSIGNED)
  const effectiveSelectedMediaColorId = useMemo(() => {
    return resolveInitialMediaColorId({
      selectedMediaColorId,
      selectedPreviewColorId: selectedColorId,
      images: draft.images || [],
      colors: activeProductColors,
      mediaMode,
      unassignedCount: unassignedImagesCount,
    });
  }, [mediaMode, activeProductColors, selectedMediaColorId, unassignedImagesCount, selectedColorId, draft.images]);

  // Derived filtered images for gallery display
  const activeFilteredImages = useMemo(() => {
    const allImages = draft.images || [];
    if (mediaMode === 'GENERAL') {
      return allImages;
    }
    if (mediaMode === 'COLORWAY') {
      if (effectiveSelectedMediaColorId === 'UNASSIGNED') {
        return allImages.filter((img) => isImageUnassigned(img, mediaMode));
      }
      if (effectiveSelectedMediaColorId) {
        return allImages.filter(
          (img) => img.colorId === effectiveSelectedMediaColorId && !img.isUnassigned
        );
      }
      return [];
    }
    return allImages;
  }, [draft.images, mediaMode, effectiveSelectedMediaColorId]);

  const effectiveVisibleImages: ProductPresentationMediaItem[] = useMemo(() => {
    return activeFilteredImages.map((img) => ({
      url: getProductStudioImageDisplayUrl(img),
      colorId: img.colorId || undefined,
    }));
  }, [activeFilteredImages]);

  const selectedColorObj = useMemo(() => {
    if (!effectiveSelectedMediaColorId || effectiveSelectedMediaColorId === 'UNASSIGNED') return null;
    return activeProductColors.find((c: any) => c.id === effectiveSelectedMediaColorId) || null;
  }, [activeProductColors, effectiveSelectedMediaColorId]);

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

  // CTA text in Product Studio is always static preview text
  const ctaText = "Добавить в корзину";

  // Handlers for local preview interactions with toggle-off (deselect) support
  const handleColorChange = (colorId: string) => {
    if (selectedColorId === colorId) {
      setSelectedColorId(null);
      return;
    }

    setSelectedColorId(colorId);

    if (mediaMode === 'COLORWAY') {
      setSelectedMediaColorId(colorId);
      setActiveImage(0);
    } else {
      const targetIdx = findFirstMediaIndexForColor(visibleImages, colorId);
      setActiveImage(targetIdx);
    }
  };

  const handleSelectMediaColorTab = (colorId: string | 'UNASSIGNED') => {
    setSelectedMediaColorId(colorId);
    if (colorId !== 'UNASSIGNED') {
      setSelectedColorId(colorId);
    }
    setActiveImage(0);
  };

  const handleSizeChange = (sizeId: string) => {
    if (selectedSizeId === sizeId) {
      setSelectedSizeId(null);
      return;
    }

    setSelectedSizeId(sizeId);
  };

  const handleActiveImageChange = (index: number) => {
    setActiveImage(index);
  };

  // Photo upload handler: local preview semantics only with validation
  const handlePhotoSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setMediaError(null);

    const existingImages = draft.images || [];
    if (existingImages.length >= MAX_PRODUCT_IMAGES) {
      setMediaError(`Удалите лишние фотографии: можно сохранить не более ${MAX_PRODUCT_IMAGES}`);
      e.target.value = '';
      return;
    }

    setIsValidatingPhoto(true);

    try {
      const validation = await validateImageFile(file);
      if (!validation.valid) {
        setMediaError(validation.error || 'Недопустимый файл');
        return;
      }

      const objectUrl = createMediaUrl(file);

      let newImageColorId: string | null = null;
      let newImageIsUnassigned = false;

      if (mediaMode === 'COLORWAY') {
        if (effectiveSelectedMediaColorId && effectiveSelectedMediaColorId !== 'UNASSIGNED') {
          newImageColorId = effectiveSelectedMediaColorId;
          newImageIsUnassigned = false;
        } else if (effectiveSelectedMediaColorId === 'UNASSIGNED') {
          newImageColorId = null;
          newImageIsUnassigned = true;
        } else if (activeProductColors.length > 0) {
          newImageColorId = activeProductColors[0].id;
          newImageIsUnassigned = false;
        } else {
          newImageColorId = null;
          newImageIsUnassigned = true;
        }
      } else {
        newImageColorId = null;
        newImageIsUnassigned = false;
      }

      const newImage = createLocalProductStudioImage({
        file,
        previewUrl: objectUrl,
        isMain: false,
        sortOrder: existingImages.length,
        colorId: newImageColorId,
        isUnassigned: newImageIsUnassigned,
        width: validation.width,
        height: validation.height,
      });

      const nextImages = normalizeProductStudioCovers(
        [...existingImages, newImage],
        mediaMode,
        activeProductColors,
        draft.variants
      );

      updateDraft({
        images: nextImages,
      });
      markTouched('media');
    } finally {
      setIsValidatingPhoto(false);
      e.target.value = '';
    }
  };

  // Color popover open & atomic commit
  const handleOpenColorPopover = () => {
    setIsSizePopoverOpen(false);
    setPendingSelectedColorIds(new Set((draft.colors || []).map((c: any) => c.id)));
    setIsColorPopoverOpen(true);
  };

  const handleApplyColors = () => {
    const nextColors = colorsList
      .filter((c: any) => pendingSelectedColorIds.has(c.id))
      .map((c: any) => ({
        id: c.id,
        name: c.nameRu,
        hex: c.hex || c.hexValue || "",
      }));

    const currentSizes = Array.from(
      new Map(
        (draft.variants || [])
          .filter((v: any) => v.sizeValueId && v.isActive !== false)
          .map((v: any) => {
            const fallback = (draft.variants || []).find((dv) => dv.sizeValueId === v.sizeValueId);
            const label = getCanonicalSizeLabel(v.sizeValueId, sizeValuesList, fallback?.size);
            return [v.sizeValueId, { id: v.sizeValueId, label }];
          })
      ).values()
    );

    if (sizeValuesList && sizeValuesList.length > 0) {
      currentSizes.sort((a, b) => {
        const idxA = sizeValuesList.findIndex((s) => s.id === a.id);
        const idxB = sizeValuesList.findIndex((s) => s.id === b.id);
        if (idxA !== -1 && idxB !== -1) {
          const orderA = sizeValuesList[idxA].sortOrder ?? idxA;
          const orderB = sizeValuesList[idxB].sortOrder ?? idxB;
          return orderA - orderB;
        }
        if (idxA !== -1) return -1;
        if (idxB !== -1) return 1;
        return 0;
      });
    }

    const updatedVariants = reconcileProductStudioVariantMatrix(
      effectiveDimensionType || draft.dimensionType || 'COLOR_AND_SIZE',
      nextColors,
      currentSizes,
      draft.variants || [],
      (draft.colors || []) as any,
      currentSizes
    );

    const nextColorIds = new Set(nextColors.map((c: any) => c.id));
    const reconciledImages = reconcileMediaOnColorRemoval(draft.images, nextColorIds, mediaMode);

    updateDraft({
      colors: nextColors,
      variants: updatedVariants,
      images: reconciledImages,
    });

    if (pendingSelectedColorIds.size === 0) {
      markTouched('color');
    }

    setIsColorPopoverOpen(false);
  };

  // Size popover open & atomic commit
  const handleOpenSizePopover = () => {
    setIsColorPopoverOpen(false);
    setPendingSelectedSizeIds(
      new Set(
        (draft.variants || [])
          .filter((v: any) => v.isActive !== false && v.sizeValueId)
          .map((v: any) => v.sizeValueId)
      )
    );
    setIsSizePopoverOpen(true);
  };

  const handleApplySizes = () => {
    const nextSizes = Array.from(pendingSelectedSizeIds).map((sId) => {
      const fallback = (draft.variants || []).find((v) => v.sizeValueId === sId);
      const label = getCanonicalSizeLabel(sId, sizeValuesList, fallback?.size);
      return { id: sId, label };
    });

    if (sizeValuesList && sizeValuesList.length > 0) {
      nextSizes.sort((a, b) => {
        const idxA = sizeValuesList.findIndex((s) => s.id === a.id);
        const idxB = sizeValuesList.findIndex((s) => s.id === b.id);
        if (idxA !== -1 && idxB !== -1) {
          const orderA = sizeValuesList[idxA].sortOrder ?? idxA;
          const orderB = sizeValuesList[idxB].sortOrder ?? idxB;
          return orderA - orderB;
        }
        if (idxA !== -1) return -1;
        if (idxB !== -1) return 1;
        return 0;
      });
    }

    const currentColors = (draft.colors || []).map((c: any) => ({
      id: c.id,
      name: c.nameRu || c.name,
      hex: c.hex || c.hexValue || "",
    }));

    const currentActiveSizes = Array.from(
      new Map(
        (draft.variants || [])
          .filter((v: any) => v.sizeValueId && v.isActive !== false)
          .map((v: any) => {
            const fallback = (draft.variants || []).find((dv) => dv.sizeValueId === v.sizeValueId);
            const label = getCanonicalSizeLabel(v.sizeValueId, sizeValuesList, fallback?.size);
            return [v.sizeValueId, { id: v.sizeValueId, label }];
          })
      ).values()
    );

    const updatedVariants = reconcileProductStudioVariantMatrix(
      effectiveDimensionType || draft.dimensionType || 'COLOR_AND_SIZE',
      currentColors,
      nextSizes,
      draft.variants || [],
      currentColors,
      currentActiveSizes
    );

    updateDraft({
      variants: updatedVariants,
    });

    if (selectedSizeId && !pendingSelectedSizeIds.has(selectedSizeId)) {
      const remaining = Array.from(pendingSelectedSizeIds);
      setSelectedSizeId(remaining[0] || null);
    } else if (!selectedSizeId && pendingSelectedSizeIds.size > 0) {
      setSelectedSizeId(Array.from(pendingSelectedSizeIds)[0]);
    }

    if (pendingSelectedSizeIds.size === 0) {
      markTouched('size');
    }

    setIsSizePopoverOpen(false);
  };

  // Price commit helper
  const handlePriceCommit = (val: string) => {
    const clean = val.replace(/\s+/g, "").replace(",", ".");
    const rub = parseFloat(clean);
    if (!isNaN(rub) && rub >= 0) {
      updateDraft({ priceCents: Math.round(rub * 100) });
    }
  };

  const formatRub = (rub: number) =>
    new Intl.NumberFormat("ru-RU", {
      style: "currency",
      currency: "RUB",
      maximumFractionDigits: 0,
    }).format(rub);

  // Escape key handler
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsColorPopoverOpen(false);
        setIsSizePopoverOpen(false);
        setShowMediaInfo(false);
        setIsEditingTitle(false);
        setIsEditingPrice(false);
        setIsEditingDescription(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Global click capture to dismiss popovers when clicking outside
  const handleWorkspaceClickCapture = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement;

    if (
      target.closest(".popover-content") ||
      target.closest(".popover-trigger") ||
      target.closest('[data-slot="title"]') ||
      target.closest('[data-slot="price"]') ||
      target.closest('[data-slot="description"]')
    ) {
      return;
    }

    setIsColorPopoverOpen(false);
    setIsSizePopoverOpen(false);
    setShowMediaInfo(false);
  };

  // Immediate blocker flags
  const isColorMissing = readiness?.blockingFields?.includes("color") ?? false;
  const isSizeMissing = readiness?.blockingFields?.includes("size") ?? false;
  const isCompositionMissing =
    readiness?.blockingFields?.includes("composition") ?? !getCompositionCompleteness(draft).isComplete;
  const isCharacteristicsMissing = readiness?.blockingFields?.includes("characteristics") ?? false;
  const offeredSizes = useMemo(() => getOfferedSizes(draft), [draft]);
  const sizeChartCompleteness = useMemo(() => {
    return getSizeChartCompleteness(draft, categorySchema);
  }, [draft, categorySchema]);

  // Interaction attention flags
  const isTitleAttention = isFieldAttention("title");
  const isPriceAttention = isFieldAttention("price");
  const isMediaAttention = isFieldAttention("media");
  const mediaWarning = useMemo(() => getMediaReadinessWarning(draft), [draft]);
  const isSelectedColorEmpty = useMemo(() => {
    if (mediaMode !== 'COLORWAY' || !selectedColorObj) return false;
    return (draft.images || []).filter(
      (img) => !img.isUnassigned && img.colorId === selectedColorObj.id
    ).length === 0;
  }, [mediaMode, selectedColorObj, draft.images]);
  const isColorAttention = isFieldAttention("color");
  const isSizeAttention = isFieldAttention("size");
  const isDescriptionAttention = isFieldAttention("description");
  const isCompositionAttention = isFieldAttention("composition");
  const isCharacteristicsAttention = isFieldAttention("characteristics");
  const isSizeChartAttention = isFieldAttention("sizeChart");

  const requiredProductAttrs = useMemo(() => {
    return getCanonicalRequiredProductAttributes(categorySchema);
  }, [categorySchema]);

  const filledRequiredCharacteristicsCount = useMemo(() => {
    if (requiredProductAttrs.length === 0) return 0;
    return requiredProductAttrs.filter((reqAttr) => {
      const found = draft.attributes?.find(
        (a) =>
          a.attributeDefinitionId === reqAttr.id ||
          (reqAttr.nameRu && a.name === reqAttr.nameRu)
      );
      if (!found) return false;
      if (found.dictionaryValueId) return true;
      if (typeof found.value === 'string') return found.value.trim().length > 0;
      if (typeof found.value === 'number') return !isNaN(found.value);
      if (typeof found.value === 'boolean') return true;
      return false;
    }).length;
  }, [requiredProductAttrs, draft.attributes]);

  const titleSlot = isEditingTitle ? (
    <input
      type="text"
      data-slot="title"
      data-testid="visual-inline-input"
      autoFocus
      value={titleValue}
      placeholder="Например: Куртка-бомбер оверсайз"
      onChange={(e) => setTitleValue(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          updateDraft({ title: titleValue.trim() });
          markTouched('title');
          setIsEditingTitle(false);
        } else if (e.key === "Escape") {
          e.preventDefault();
          setTitleValue(draft.title || "");
          setIsEditingTitle(false);
        }
      }}
      onBlur={() => {
        updateDraft({ title: titleValue.trim() });
        markTouched('title');
        setIsEditingTitle(false);
      }}
      className="w-full text-[26px] sm:text-[28px] min-[1200px]:text-[32px] leading-[32px] sm:leading-[34px] min-[1200px]:leading-[38px] font-serif text-graphite dark:text-white font-normal mt-1 bg-transparent border border-indigo-500 rounded px-1.5 py-0.5 outline-none ring-1 ring-indigo-500"
    />
  ) : (
    <div data-slot="title" className="group/title">
      <h1
        role="heading"
        aria-level={1}
        onClick={() => {
          setTitleValue(draft.title || "");
          setIsEditingTitle(true);
        }}
        className={cn(
          "text-[26px] sm:text-[28px] min-[1200px]:text-[32px] leading-[32px] sm:leading-[34px] min-[1200px]:leading-[38px] font-serif font-normal mt-1 cursor-text rounded-sm transition-colors",
          isTitleAttention
            ? "text-amber-700 dark:text-amber-400 border-b-2 border-dashed border-amber-400/80 hover:outline-dashed hover:outline-1 hover:outline-amber-400"
            : "text-graphite dark:text-white hover:outline-dashed hover:outline-1 hover:outline-ash/50"
        )}
        title="Нажмите, чтобы изменить название"
      >
        {draft.title?.trim() || "Название товара *"}
      </h1>
      {isTitleAttention && (
        <span data-testid="title-required-helper" className="block text-xs font-sans text-amber-700 dark:text-amber-400 font-medium mt-0.5">
          Укажите название
        </span>
      )}
    </div>
  );

  const priceSlot = isEditingPrice ? (
    <div className="mt-3.5 flex items-baseline gap-2" data-slot="price">
      <input
        type="text"
        data-slot="price"
        data-testid="visual-inline-input"
        autoFocus
        value={priceValue}
        placeholder="0"
        onChange={(e) => setPriceValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            handlePriceCommit(priceValue);
            markTouched('price');
            setIsEditingPrice(false);
          } else if (e.key === "Escape") {
            e.preventDefault();
            const currentRub = draft.priceCents !== undefined && draft.priceCents > 0 ? String(draft.priceCents / 100) : "";
            setPriceValue(currentRub);
            setIsEditingPrice(false);
          }
        }}
        onBlur={() => {
          handlePriceCommit(priceValue);
          markTouched('price');
          setIsEditingPrice(false);
        }}
        className="w-44 text-[26px] min-[1200px]:text-[28px] font-semibold text-graphite dark:text-white bg-transparent border border-indigo-500 rounded px-1.5 py-0.5 outline-none ring-1 ring-indigo-500"
      />
      <span className="text-base text-ash font-medium">₽</span>
    </div>
  ) : (
    <div
      data-slot="price"
      onClick={() => {
        const currentRub = draft.priceCents !== undefined && draft.priceCents > 0 ? String(draft.priceCents / 100) : "";
        setPriceValue(currentRub);
        setIsEditingPrice(true);
      }}
      className="mt-3.5 cursor-text group/price"
      title="Нажмите, чтобы изменить цену"
    >
      <div className="flex items-baseline gap-3">
        {product.discountPrice ? (
          <>
            <span
              data-testid="visual-display-price"
              className="text-[26px] min-[1200px]:text-[28px] font-semibold text-red-600 dark:text-red-400 group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-ash/50 rounded-sm px-1"
            >
              {formatRub(product.discountPrice)}
            </span>
            <span className="text-base text-ash line-through">
              {formatRub(displayPrice)}
            </span>
          </>
        ) : draft.priceCents !== undefined && draft.priceCents > 0 ? (
          <span
            data-testid="visual-display-price"
            className="text-[26px] min-[1200px]:text-[28px] font-semibold tracking-tight text-graphite dark:text-white font-sans group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-ash/50 rounded-sm px-1"
          >
            {formatRub(draft.priceCents / 100)}
          </span>
        ) : (
          <span
            data-testid="visual-display-price"
            className={cn(
              "text-[26px] min-[1200px]:text-[28px] font-semibold tracking-tight font-sans rounded-sm px-1 transition-colors",
              isPriceAttention
                ? "text-amber-700 dark:text-amber-400 border-b-2 border-dashed border-amber-400/80 group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-amber-400"
                : "text-ash/60 dark:text-white/40 group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-ash/50"
            )}
          >
            {displayPrice > 0 ? formatRub(displayPrice) : "Цена, ₽ *"}
          </span>
        )}
      </div>
      {isPriceAttention && (
        <span data-testid="price-required-helper" className="block text-xs font-sans text-amber-700 dark:text-amber-400 font-medium mt-0.5">
          Укажите цену
        </span>
      )}
    </div>
  );

  const descriptionSlot = isEditingDescription ? (
    <div data-slot="description" className="w-full">
      <textarea
        data-slot="description"
        data-testid="visual-inline-textarea"
        autoFocus
        value={descValue}
        placeholder="Расскажите о посадке, особенностях модели и важных деталях товара"
        onChange={(e) => setDescValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            setDescValue(draft.description || "");
            setIsEditingDescription(false);
          }
        }}
        onBlur={() => {
          updateDraft({ description: descValue });
          markTouched('description');
          setIsEditingDescription(false);
        }}
        className="w-full min-h-[140px] text-sm text-graphite-light dark:text-white/80 leading-relaxed bg-transparent border border-indigo-500 rounded p-2.5 outline-none ring-1 ring-indigo-500 resize-y"
      />
    </div>
  ) : (
    <div
      data-slot="description"
      data-testid="description-container"
      onClick={() => {
        setDescValue(draft.description || "");
        setIsEditingDescription(true);
      }}
      className={cn(
        "text-sm leading-relaxed whitespace-pre-line space-y-2 cursor-text rounded-xl p-3 transition-colors",
        isDescriptionAttention
          ? "bg-amber-50/40 dark:bg-amber-950/20 border border-dashed border-amber-300 dark:border-amber-800/70 hover:border-amber-400"
          : "text-graphite-light dark:text-white/80 hover:outline-dashed hover:outline-1 hover:outline-ash/50"
      )}
      title="Нажмите, чтобы изменить описание"
    >
      {draft.description?.trim() ? (
        <p className="text-graphite-light dark:text-white/80">{draft.description}</p>
      ) : (
        <div>
          <p className={cn(
            "text-sm font-medium",
            isDescriptionAttention ? "text-amber-700 dark:text-amber-400" : "text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white"
          )}>
            Добавить описание *
          </p>
          {isDescriptionAttention ? (
            <span
              data-testid="description-required-helper"
              className="block text-xs text-amber-700/80 dark:text-amber-400/80 mt-1"
            >
              Добавьте описание товара
            </span>
          ) : (
            <span
              data-testid="description-required-helper"
              className="block text-xs text-ash mt-1"
            >
              Добавьте описание товара
            </span>
          )}
        </div>
      )}
    </div>
  );

  const handleDeleteVisualImage = (index: number) => {
    const target = activeFilteredImages[index];
    if (!target) return;
    const existing = [...(draft.images || [])];
    const draftIdx = existing.findIndex(
      (img) => img === target || (img.uiKey && img.uiKey === target.uiKey)
    );
    if (draftIdx === -1) return;

    existing.splice(draftIdx, 1);
    const reindexed = existing.map((img, i) => ({
      ...img,
      sortOrder: i,
    }));
    const normalized = normalizeProductStudioCovers(
      reindexed,
      mediaMode,
      activeProductColors,
      draft.variants
    );
    if (effectiveSelectedMediaColorId) {
      setSelectedMediaColorId(effectiveSelectedMediaColorId);
    }
    updateDraft({ images: normalized });
    markTouched('media');
    if (activeImage >= activeFilteredImages.length - 1) {
      setActiveImage(Math.max(0, activeFilteredImages.length - 2));
    }
  };

  const handleThumbnailReorder = (fromIndex: number, toIndex: number) => {
    if (fromIndex === toIndex) return;
    const currentImages = draft.images || [];
    const nextImages = reorderProductStudioImages(
      currentImages,
      fromIndex,
      toIndex,
      mediaMode === 'COLORWAY' ? effectiveSelectedMediaColorId : null,
      mediaMode,
      activeProductColors,
      draft.variants
    );
    updateDraft({ images: nextImages });
    markTouched('media');
    setActiveImage(toIndex);
  };

  const renderMediaThumbnailOverlay = (_image: ProductPresentationMediaItem, index: number) => {
    const currentImg = activeFilteredImages[index];
    const isUnassigned = currentImg?.isUnassigned;
    const isCover = currentImg?.isMain;

    return (
      <>
        {isCover && (
          <span
            data-testid={`thumbnail-main-badge-${index}`}
            className="absolute top-1 left-1 px-1.5 py-0.5 rounded text-[9px] font-semibold bg-gray-900 text-white dark:bg-white dark:text-gray-900 leading-none z-10 shadow-xs pointer-events-none"
          >
            Обложка
          </span>
        )}

        {mediaMode === 'COLORWAY' && (isUnassigned || !currentImg?.colorId) && (
          <span
            data-testid={`thumbnail-unassigned-dot-${index}`}
            title="Нераспределённая фотография"
            className="absolute bottom-1 right-1 w-2.5 h-2.5 rounded-full bg-amber-500 border border-white dark:border-black shadow-xs pointer-events-none z-10"
          />
        )}

        {mediaMode === 'COLORWAY' && currentImg?.colorId && (() => {
          const assignedColor = (draft.colors || []).find((c: any) => c.id === currentImg.colorId);
          if (!assignedColor) return null;
          return (
            <span
              data-testid={`thumbnail-color-dot-${index}`}
              title={`Цвет: ${getColorDisplayName(assignedColor)}`}
              className="absolute bottom-1 right-1 w-2.5 h-2.5 rounded-full border border-white dark:border-black shadow-xs pointer-events-none z-10"
              style={{ backgroundColor: assignedColor.hex || "#000000" }}
            />
          );
        })()}

        <div className="absolute inset-0 bg-black/25 opacity-0 group-hover/thumb:opacity-100 transition-opacity flex items-center justify-center gap-1 rounded-lg pointer-events-none z-20">
          <button
            type="button"
            draggable={false}
            onDragStart={(e) => {
              e.stopPropagation();
            }}
            data-testid={`thumbnail-delete-btn-${index}`}
            title="Удалить фото"
            aria-label="Удалить фото"
            onClick={(e) => {
              e.stopPropagation();
              handleDeleteVisualImage(index);
            }}
            className="pointer-events-auto w-5 h-5 rounded-full bg-red-600/90 hover:bg-red-600 text-white flex items-center justify-center text-[10px] font-bold shadow-xs cursor-pointer"
          >
            ✕
          </button>
        </div>
      </>
    );
  };

  const safeActiveImage = activeImage < effectiveVisibleImages.length ? activeImage : 0;

  return (
    <div
      ref={workspaceRef}
      id="studio-workspace-visual"
      role="tabpanel"
      aria-labelledby="studio-tab-visual"
      data-testid="studio-visual-workspace"
      className="w-full relative"
      onClickCapture={handleWorkspaceClickCapture}
    >
      <div className="max-w-[1360px] mx-auto px-4 sm:px-6 lg:px-8 py-6 relative">
        {mediaError && effectiveVisibleImages.length > 0 && (
          <div
            data-testid="media-upload-error-banner"
            className="mb-4 text-xs text-red-600 bg-red-50 dark:bg-red-950/40 p-3 rounded-lg border border-red-200 dark:border-red-900/50 flex items-center justify-between"
          >
            <span>{mediaError}</span>
            <button
              type="button"
              onClick={() => setMediaError(null)}
              className="text-red-500 hover:text-red-700 font-bold ml-2"
            >
              ✕
            </button>
          </div>
        )}

        {(draft.images || []).length > MAX_PRODUCT_IMAGES && (
          <div
            data-testid="oversized-media-banner"
            className="mb-4 p-4 rounded-xl border border-amber-300 dark:border-amber-700/60 bg-amber-50 dark:bg-amber-950/30 flex items-center justify-between gap-3 text-amber-900 dark:text-amber-200"
          >
            <div className="flex items-center gap-3">
              <AlertTriangle className="w-5 h-5 text-amber-600 dark:text-amber-400 shrink-0" />
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-semibold">
                  {`Удалите лишние фотографии: можно сохранить не более ${MAX_PRODUCT_IMAGES}`}
                </span>
                <span className="text-xs text-amber-800 dark:text-amber-300">
                  {`В товаре сохранено ${(draft.images || []).length} фото. Максимально допустимо: ${MAX_PRODUCT_IMAGES}.`}
                </span>
              </div>
            </div>
            <span
              className="px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-200/60 dark:bg-amber-900/60 text-amber-900 dark:text-amber-200 shrink-0"
              data-testid="media-progress-badge"
            >
              {getMediaProgressText((draft.images || []).length)}
            </span>
          </div>
        )}

        {mediaMode === 'LEGACY_MIXED' && (
          <div
            data-testid="legacy-mixed-banner"
            className="mb-4 p-4 rounded-xl border border-amber-300 dark:border-amber-700/60 bg-amber-50 dark:bg-amber-950/30 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 text-amber-900 dark:text-amber-200"
          >
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-5 h-5 text-amber-600 dark:text-amber-400 shrink-0" />
                <span className="text-sm font-medium">
                  Фотографии товара нужно привести к одному режиму перед сохранением.
                </span>
              </div>
              {hasOrphanedImages ? (
                <p className="text-xs text-amber-800 dark:text-amber-300 ml-7">
                  Некоторые фотографии были привязаны к цветам, которых больше нет в товаре.
                </p>
              ) : (
                <p className="text-xs text-amber-800 dark:text-amber-300 ml-7">
                  В товаре есть и общие фотографии, и фотографии с привязкой к цветам.
                </p>
              )}
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                type="button"
                data-testid="resolve-to-general-btn"
                onClick={() => {
                  const resolved = resolveLegacyMixedMedia(draft.images || [], 'GENERAL', activeProductColors, draft.variants);
                  updateDraft({ images: resolved, mediaMode: 'GENERAL' });
                  markTouched('media');
                }}
                className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-white dark:bg-neutral-800 border border-amber-300 dark:border-amber-700 hover:bg-amber-100 dark:hover:bg-neutral-700 transition-colors cursor-pointer"
              >
                Объединить в общую галерею
              </button>
              <button
                type="button"
                data-testid="resolve-to-colorway-btn"
                disabled={activeProductColors.length === 0}
                onClick={() => {
                  const resolved = resolveLegacyMixedMedia(draft.images || [], 'COLORWAY', activeProductColors, draft.variants);
                  updateDraft({ images: resolved, mediaMode: 'COLORWAY' });
                  markTouched('media');
                }}
                className={cn(
                  "px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors",
                  activeProductColors.length === 0
                    ? "bg-neutral-300 dark:bg-neutral-700 text-neutral-500 cursor-not-allowed opacity-60"
                    : "bg-amber-600 hover:bg-amber-700 text-white cursor-pointer"
                )}
              >
                Разложить по цветам
              </button>
            </div>
          </div>
        )}

        {mediaMode === 'COLORWAY' && unassignedImagesCount > 0 && (
          <div
            data-testid="unassigned-photos-banner"
            className="mb-4 p-3 rounded-xl border border-amber-200 dark:border-amber-800/40 bg-amber-50/70 dark:bg-amber-950/20 flex items-center justify-between gap-2 text-amber-800 dark:text-amber-300 text-xs"
          >
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-amber-500 shrink-0" />
              <span>
                Нераспределённые фотографии: {unassignedImagesCount}. Их нужно привязать к цветам перед сохранением.
              </span>
            </div>
            <button
              type="button"
              data-testid="unassigned-banner-bind-btn"
              onClick={() => setIsPhotoColorModalOpen(true)}
              disabled={!draft.colors || draft.colors.length === 0}
              className="px-2.5 py-1 rounded-md border border-amber-300 dark:border-amber-700/60 bg-white dark:bg-amber-900/40 text-amber-900 dark:text-amber-200 font-semibold text-xs hover:bg-amber-100 dark:hover:bg-amber-900/60 transition-colors shadow-2xs shrink-0 cursor-pointer"
            >
              Распределить
            </button>
          </div>
        )}

        <ProductPresentationCore
          product={product}
          visibleImages={effectiveVisibleImages}
          activeImage={safeActiveImage}
          onActiveImageChange={handleActiveImageChange}
          displayPrice={displayPrice}
          emptyPricePlaceholder="Цена, ₽ *"
          colors={visibleColors}
          sizes={sizes}
          colorLabelSuffix={requiresColor ? " *" : ""}
          colorLabelSuffixClassName={isColorAttention ? "text-amber-600 dark:text-amber-400" : "text-ash dark:text-gray-400"}
          sizeLabelSuffix={requiresSize ? " *" : ""}
          sizeLabelSuffixClassName={isSizeAttention ? "text-amber-600 dark:text-amber-400" : "text-ash dark:text-gray-400"}
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
          sizeSelectionNotice={null}
          refreshErrorNotice={null}
          sizeError=""
          titleSlot={titleSlot}
          priceSlot={priceSlot}
          descriptionSlot={descriptionSlot}
          renderThumbnailOverlay={renderMediaThumbnailOverlay}
          onThumbnailReorder={handleThumbnailReorder}
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
          emptyMediaSlot={
            <label
              data-testid="presentation-empty-media-slot"
              className={cn(
                "w-full aspect-[4/5] max-h-[640px] rounded-xl flex flex-col items-center justify-center p-8 text-center cursor-pointer transition-colors group relative",
                isMediaAttention
                  ? "bg-amber-50/20 dark:bg-amber-950/10 border-2 border-dashed border-amber-300 dark:border-amber-700/50 hover:border-amber-400"
                  : "bg-paper-light dark:bg-paper-dark border-2 border-dashed border-ash/30 hover:border-graphite dark:hover:border-white/50"
              )}
            >
              <input
                type="file"
                accept={ALLOWED_IMAGE_MIME_TYPES.join(',')}
                className="hidden"
                aria-label="Добавить фото"
                data-testid="empty-stage-photo-input"
                disabled={(draft.images || []).length >= MAX_PRODUCT_IMAGES}
                onChange={handlePhotoSelect}
              />
              <div className={cn(
                "w-16 h-16 rounded-full flex items-center justify-center text-3xl font-light group-hover:scale-110 transition-transform mb-4",
                isMediaAttention
                  ? "bg-amber-100/60 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400"
                  : "bg-ash/10 dark:bg-white/10 text-graphite dark:text-white"
              )}>
                +
              </div>
              <span className={cn(
                "text-base font-semibold mb-2",
                isMediaAttention
                  ? "text-amber-700 dark:text-amber-400"
                  : "text-graphite dark:text-white"
              )}>
                {selectedColorObj
                  ? `Для цвета «${getColorDisplayName(selectedColorObj)}» пока нет фотографий`
                  : selectedMediaColorId === 'UNASSIGNED'
                  ? 'Нет нераспределённых фотографий'
                  : 'Добавить фото *'}
              </span>

              {selectedColorObj && (
                <span
                  data-testid="empty-stage-add-btn"
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-gray-900 text-white dark:bg-white dark:text-gray-900 text-xs font-semibold shadow-xs mb-3 group-hover:bg-gray-800 transition-colors"
                >
                  + Добавить фото
                </span>
              )}

              <span className="text-xs font-medium text-graphite dark:text-white mb-1" data-testid="media-progress-badge">
                {getMediaProgressText((draft.images || []).length)}
              </span>
              {mediaWarning && (isMediaAttention || isSelectedColorEmpty) && (
                <span data-testid="media-required-helper" className="text-xs font-semibold text-amber-700 dark:text-amber-400 mb-1">
                  {mediaWarning}
                </span>
              )}
              <span className="text-xs text-ash dark:text-gray-400 mb-4">
                Первая — обложка
              </span>
              <div className="flex items-center gap-1.5 text-[11px] text-ash/80 dark:text-gray-400">
                <span>Вертикальные · от 800×1000 · JPG/PNG/WebP · до 10 МБ</span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setShowMediaInfo((prev) => !prev);
                  }}
                  title="Рекомендация по формату"
                  className="p-0.5 rounded-full hover:bg-black/10 dark:hover:bg-white/10 text-ash hover:text-graphite transition-colors"
                >
                  <Info className="w-3.5 h-3.5" />
                </button>
              </div>
              {showMediaInfo && (
                <div
                  className="mt-2 text-[10px] text-ash/90 dark:text-gray-300 max-w-xs bg-black/5 dark:bg-white/5 p-2 rounded-lg"
                  onClick={(e) => e.stopPropagation()}
                >
                  Лучше использовать формат 4:5 — фото лучше заполняет карточку товара
                </div>
              )}
              {isValidatingPhoto && (
                <p className="mt-3 text-xs text-indigo-600 dark:text-indigo-400 font-medium">Проверка файла...</p>
              )}
              {mediaError && (
                <p className="mt-3 text-xs text-red-500 font-medium" data-testid="media-upload-error">
                  {mediaError}
                </p>
              )}
            </label>
          }
          galleryExtraSlot={
            ((draft.images && draft.images.length > 0) || (draft.colors && draft.colors.length > 0)) ? (
              <div className="flex flex-col gap-2.5 w-full" data-testid="media-gallery-controls">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div
                    data-testid="media-mode-switcher"
                    className="inline-flex items-center rounded-lg bg-black/5 dark:bg-white/10 p-0.5 text-xs font-medium"
                  >
                    <button
                      type="button"
                      data-testid="media-mode-general-btn"
                      onClick={() => {
                        if (mediaMode === 'GENERAL') return;
                        if ((draft.images || []).length > 0) {
                          setIsConfirmToGeneralOpen(true);
                        } else {
                          updateDraft({ mediaMode: 'GENERAL' });
                          markTouched('media');
                        }
                      }}
                      className={cn(
                        "px-2.5 py-1 rounded-md transition-colors cursor-pointer",
                        mediaMode === 'GENERAL'
                          ? "bg-white dark:bg-neutral-800 text-graphite dark:text-white shadow-xs font-semibold"
                          : "text-ash hover:text-graphite dark:hover:text-white"
                      )}
                    >
                      Общая галерея
                    </button>
                    <button
                      type="button"
                      data-testid="media-mode-colorway-btn"
                      disabled={activeProductColors.length === 0}
                      title={
                        activeProductColors.length === 0
                          ? "Добавьте цвет товара, чтобы использовать фотографии по цветам."
                          : "Фотографии по цветам"
                      }
                      onClick={() => {
                        if (activeProductColors.length === 0) return;
                        if (mediaMode === 'COLORWAY') return;
                        if ((draft.images || []).length > 0) {
                          setIsConfirmToColorwayOpen(true);
                        } else {
                          updateDraft({ mediaMode: 'COLORWAY' });
                          markTouched('media');
                        }
                      }}
                      className={cn(
                        "px-2.5 py-1 rounded-md transition-colors",
                        activeProductColors.length === 0
                          ? "text-ash/40 opacity-50 cursor-not-allowed"
                          : mediaMode === 'COLORWAY'
                          ? "bg-white dark:bg-neutral-800 text-graphite dark:text-white shadow-xs font-semibold cursor-pointer"
                          : "text-ash hover:text-graphite dark:hover:text-white cursor-pointer"
                      )}
                    >
                      Фотографии по цветам
                    </button>
                  </div>

                  {(mediaMode === 'COLORWAY'
                    ? unassignedImagesCount > 0
                    : ((draft.images && draft.images.length > 0) || activeProductColors.length === 0)
                  ) && (
                    <button
                      type="button"
                      data-testid="bind-photos-to-colors-btn"
                      disabled={activeProductColors.length === 0}
                      title={activeProductColors.length === 0 ? "Сначала добавьте цвета" : "Привязать фото к цветам"}
                      onClick={() => setIsPhotoColorModalOpen(true)}
                      className={cn(
                        "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-medium transition-all select-none",
                        activeProductColors.length === 0
                          ? "border-black/5 dark:border-white/5 bg-black/[0.02] dark:bg-white/[0.02] text-ash/40 dark:text-white/20 cursor-not-allowed"
                          : "border-amber-300 dark:border-amber-700/60 bg-amber-50 dark:bg-amber-950/30 text-amber-900 dark:text-amber-200 hover:bg-amber-100 dark:hover:bg-amber-900/50 hover:border-amber-400 shadow-2xs cursor-pointer font-semibold"
                      )}
                    >
                      <Link2
                        className={cn(
                          "w-3.5 h-3.5 shrink-0",
                          activeProductColors.length === 0
                            ? "text-ash/30 dark:text-white/20"
                            : "text-amber-600 dark:text-amber-400"
                        )}
                      />
                      <span>
                        {mediaMode === 'COLORWAY' && unassignedImagesCount > 0
                          ? `Распределить фотографии · ${unassignedImagesCount}`
                          : "Привязать фото к цветам"}
                      </span>
                      {activeProductColors.length === 0 && (
                        <span className="text-[11px] text-ash/60 dark:text-white/30 font-normal">(Сначала добавьте цвета)</span>
                      )}
                    </button>
                  )}
                </div>

                {mediaMode === 'COLORWAY' && activeProductColors.length > 0 && (
                  <div
                    data-testid="colorway-tabs-bar"
                    className="flex items-center gap-1.5 overflow-x-auto py-1 no-scrollbar flex-wrap"
                  >
                    {activeProductColors.map((color) => {
                      const isSelected = effectiveSelectedMediaColorId === color.id;
                      const count = colorPhotoCounts[color.id] || 0;
                      return (
                        <button
                          key={color.id}
                          type="button"
                          data-testid={`colorway-tab-${color.id}`}
                          aria-selected={isSelected ? "true" : "false"}
                          onClick={() => handleSelectMediaColorTab(color.id)}
                          className={cn(
                            "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer border select-none shrink-0",
                            isSelected
                              ? "bg-gray-900 text-white dark:bg-white dark:text-gray-900 border-transparent shadow-xs font-semibold"
                              : "bg-white dark:bg-neutral-800 text-graphite dark:text-white border-gray-200 dark:border-white/10 hover:border-gray-300 dark:hover:border-white/20"
                          )}
                        >
                          <span
                            className="w-2.5 h-2.5 rounded-full border border-black/10 dark:border-white/20 shrink-0"
                            style={{ backgroundColor: color.hex || '#000000' }}
                          />
                          <span>{getColorDisplayName(color)}</span>
                          <span className={cn(
                            "text-[11px] opacity-75 font-normal ml-0.5",
                            isSelected ? "text-white/80 dark:text-gray-700" : "text-ash dark:text-gray-400"
                          )}>
                            · {count}
                          </span>
                        </button>
                      );
                    })}

                    {unassignedImagesCount > 0 && (
                      <button
                        type="button"
                        data-testid="colorway-tab-unassigned"
                        aria-selected={effectiveSelectedMediaColorId === 'UNASSIGNED' ? "true" : "false"}
                        onClick={() => handleSelectMediaColorTab('UNASSIGNED')}
                        className={cn(
                          "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer border select-none shrink-0",
                          effectiveSelectedMediaColorId === 'UNASSIGNED'
                            ? "bg-amber-600 text-white border-transparent shadow-xs font-semibold"
                            : "bg-amber-50 dark:bg-amber-950/30 text-amber-900 dark:text-amber-200 border-amber-300 dark:border-amber-700/60 hover:bg-amber-100"
                        )}
                      >
                        <span className="text-amber-500 font-bold">⚠</span>
                        <span>Нераспределённые</span>
                        <span className="text-[11px] opacity-80 font-normal ml-0.5">
                          · {unassignedImagesCount}
                        </span>
                      </button>
                    )}
                  </div>
                )}
              </div>
            ) : null
          }
          mediaAddSlot={
            effectiveVisibleImages.length < MAX_PRODUCT_IMAGES && (draft.images || []).length < MAX_PRODUCT_IMAGES ? (
              <label
                className="w-14 h-[70px] sm:w-16 sm:h-20 min-[1200px]:w-[72px] min-[1200px]:h-[90px] flex-shrink-0 rounded-lg border-2 border-dashed border-ash/30 flex flex-col items-center justify-center cursor-pointer hover:border-graphite transition-colors hover:bg-gray-50/50 text-center p-1 group"
                title={getMediaProgressText((draft.images || []).length)}
                data-testid="media-add-thumb-slot"
              >
                <input
                  type="file"
                  accept={ALLOWED_IMAGE_MIME_TYPES.join(',')}
                  className="hidden"
                  aria-label="Добавить фото"
                  data-testid="thumbnail-add-photo-input"
                  onChange={handlePhotoSelect}
                />
                <span className="text-xl text-ash group-hover:text-graphite leading-none">+</span>
                <span className="text-[10px] text-ash group-hover:text-graphite font-medium mt-1 leading-tight">
                  + фото
                </span>
              </label>
            ) : null
          }
          colorAddSlot={
            <div className="relative popover-trigger inline-flex items-center">
              <button
                type="button"
                data-testid="color-manage-btn"
                aria-label={colors.length > 0 ? "Управление цветами" : "Добавить цвет"}
                title={colors.length > 0 ? "Управление цветами" : isColorMissing ? "Выберите цвет" : "Добавить цвет"}
                onClick={(e) => {
                  e.stopPropagation();
                  if (isColorPopoverOpen) {
                    setIsColorPopoverOpen(false);
                  } else {
                    handleOpenColorPopover();
                  }
                }}
                className={cn(
                  "w-11 h-11 rounded-full border border-dashed flex items-center justify-center transition-colors cursor-pointer",
                  isColorAttention
                    ? "border-amber-400 text-amber-700 bg-amber-50/30 dark:bg-amber-950/20 hover:border-amber-500 hover:text-amber-800"
                    : "border-ash/50 text-ash hover:text-graphite hover:border-graphite"
                )}
              >
                <span className="text-xl font-light leading-none">+</span>
              </button>
              {isColorAttention && (
                <span data-testid="color-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium ml-2 shrink-0">
                  Выберите цвет
                </span>
              )}
              {isColorPopoverOpen && (
                <div
                  data-testid="color-popover"
                  className="absolute top-12 left-0 z-50 bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-xl shadow-xl p-4 w-72 popover-content"
                  onClick={(e) => e.stopPropagation()}
                >
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="font-medium text-sm text-graphite dark:text-white">
                      Выберите цвет
                    </h3>
                    <button
                      type="button"
                      onClick={() => setIsColorPopoverOpen(false)}
                      className="text-xs text-ash hover:text-graphite dark:hover:text-white"
                    >
                      Отмена
                    </button>
                  </div>
                  <div className="max-h-60 overflow-y-auto space-y-1 mb-3">
                    {colorsList.map((color) => {
                      const hex = color.hex || color.hexValue || "#cccccc";
                      const isChecked = pendingSelectedColorIds.has(color.id);
                      return (
                        <label
                          key={color.id}
                          className="w-full flex items-center gap-3 px-2.5 py-1.5 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer text-left text-sm"
                        >
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => {
                              const next = new Set(pendingSelectedColorIds);
                              if (next.has(color.id)) next.delete(color.id);
                              else next.add(color.id);
                              setPendingSelectedColorIds(next);
                            }}
                            className="rounded border-gray-300 dark:border-white/20 text-indigo-600 focus:ring-indigo-500"
                          />
                          <span
                            className="w-5 h-5 rounded-full border border-gray-300 dark:border-white/20 shrink-0 shadow-sm"
                            style={{ backgroundColor: hex }}
                          />
                          <span className="font-medium text-graphite dark:text-white truncate">
                            {color.nameRu}
                          </span>
                        </label>
                      );
                    })}
                  </div>
                  <button
                    type="button"
                    data-testid="color-popover-apply-btn"
                    onClick={handleApplyColors}
                    className="w-full h-9 bg-graphite text-white dark:bg-white dark:text-black rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity cursor-pointer flex items-center justify-center"
                  >
                    Готово
                  </button>
                </div>
              )}
            </div>
          }
          sizeAddSlot={
            <div className="relative popover-trigger inline-flex items-center">
              <button
                type="button"
                data-testid="size-manage-btn"
                aria-label={sizes.length > 0 ? "Управление размерами" : "Добавить размер"}
                title={sizes.length > 0 ? "Управление размерами" : isSizeMissing ? "Выберите размер" : "Добавить размер"}
                onClick={(e) => {
                  e.stopPropagation();
                  if (isSizePopoverOpen) {
                    setIsSizePopoverOpen(false);
                  } else {
                    handleOpenSizePopover();
                  }
                }}
                className={cn(
                  "min-w-[48px] h-11 px-3.5 rounded-md border border-dashed flex items-center justify-center transition-colors cursor-pointer",
                  isSizeAttention
                    ? "border-amber-400 text-amber-700 bg-amber-50/30 dark:bg-amber-950/20 hover:border-amber-500 hover:text-amber-800"
                    : "border-ash/50 text-ash hover:text-graphite hover:border-graphite"
                )}
              >
                <span className="text-xl font-light leading-none">+</span>
              </button>
              {isSizeAttention && (
                <span data-testid="size-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium ml-2 shrink-0">
                  Выберите размер
                </span>
              )}
              {isSizePopoverOpen && (
                <div
                  data-testid="size-popover"
                  className="absolute top-12 left-0 z-50 bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-xl shadow-xl p-4 w-80 popover-content"
                  onClick={(e) => e.stopPropagation()}
                >
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="font-medium text-sm text-graphite dark:text-white">
                      Выберите размер
                    </h3>
                    <button
                      type="button"
                      onClick={() => setIsSizePopoverOpen(false)}
                      className="text-xs text-ash hover:text-graphite dark:hover:text-white"
                    >
                      Отмена
                    </button>
                  </div>

                  {!draft.categoryId ? (
                    <div className="space-y-3">
                      <p className="text-xs text-ash">Сначала выберите категорию товара</p>
                      <button
                        type="button"
                        onClick={() => {
                          setIsSizePopoverOpen(false);
                          markTouched('category');
                          setCategoryModalOpen(true);
                        }}
                        className="w-full h-10 border border-indigo-200 dark:border-indigo-900 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 rounded-md text-xs font-medium hover:bg-indigo-100 dark:hover:bg-indigo-900/50 transition-colors"
                      >
                        Выбрать категорию
                      </button>
                    </div>
                  ) : loadingSizes ? (
                    <p className="text-xs text-ash py-2">Загрузка размеров...</p>
                  ) : allowedSizeSystems.length === 0 ? (
                    <div className="py-2 space-y-2">
                      <p className="text-xs text-ash">
                        Для этой категории пока не настроены размеры
                      </p>
                      <button
                        type="button"
                        onClick={() => {
                          setIsSizePopoverOpen(false);
                          setCategoryModalOpen(true);
                        }}
                        className="w-full h-9 px-3 border border-indigo-200 dark:border-indigo-900 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 rounded-md text-xs font-medium hover:bg-indigo-100 dark:hover:bg-indigo-900/50 transition-colors"
                      >
                        Выбрать другую категорию
                      </button>
                    </div>
                  ) : (
                    <div>
                      {/* Size system switcher tabs if multiple */}
                      {allowedSizeSystems.length > 1 && (
                        <div className="flex items-center gap-1.5 mb-3 border-b border-gray-100 dark:border-white/10 pb-2 overflow-x-auto">
                          {allowedSizeSystems.map((sys) => (
                            <button
                              key={sys.id}
                              type="button"
                              onClick={() => setSelectedSizeSystemId(sys.id)}
                              className={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${
                                selectedSizeSystemId === sys.id
                                  ? "bg-black text-white dark:bg-white dark:text-black font-semibold"
                                  : "text-ash hover:text-graphite dark:hover:text-white bg-gray-100 dark:bg-white/5"
                              }`}
                            >
                              {sys.name}
                            </button>
                          ))}
                        </div>
                      )}

                      {sizeValuesList.length === 0 ? (
                        <p className="text-xs text-ash py-2">
                          Нет доступных значений размера в выбранной системе.
                        </p>
                      ) : (
                        <div className="grid grid-cols-3 gap-1.5 max-h-60 overflow-y-auto mb-3">
                          {sizeValuesList.map((sv) => {
                            const isChecked = pendingSelectedSizeIds.has(sv.id);
                            const label = getCanonicalSizeLabel(sv.id, sizeValuesList, sv.value);
                            return (
                              <button
                                key={sv.id}
                                type="button"
                                data-testid={`visual-size-option-${sv.id}`}
                                onClick={() => {
                                  const next = new Set(pendingSelectedSizeIds);
                                  if (next.has(sv.id)) next.delete(sv.id);
                                  else next.add(sv.id);
                                  setPendingSelectedSizeIds(next);
                                }}
                                className={`h-10 border rounded-md flex items-center justify-center text-xs font-medium transition-colors cursor-pointer ${
                                  isChecked
                                    ? "border-indigo-600 bg-indigo-50 dark:bg-indigo-950/40 text-indigo-700 dark:text-indigo-300 font-semibold ring-1 ring-indigo-600"
                                    : "border-border-soft dark:border-white/10 text-graphite dark:text-white hover:border-graphite dark:hover:border-white"
                                }`}
                              >
                                {label}
                              </button>
                            );
                          })}
                        </div>
                      )}

                      <button
                        type="button"
                        data-testid="size-popover-apply-btn"
                        onClick={handleApplySizes}
                        className="w-full h-9 bg-graphite text-white dark:bg-white dark:text-black rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity cursor-pointer flex items-center justify-center"
                      >
                        Готово
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          }
          sizeChartSlot={
            sizeChartCompleteness.isNeeded && (
              isSizeChartAttention ? (
                <button
                  type="button"
                  data-testid="size-chart-required-btn"
                  onClick={() => setIsSizeChartModalOpen(true)}
                  className="text-xs font-semibold text-amber-700 dark:text-amber-400 hover:text-amber-800 underline decoration-dashed underline-offset-2 transition-colors cursor-pointer"
                >
                  {sizeChartCompleteness.filledRequiredCellCount === 0
                    ? "Таблица размеров * · Не заполнена"
                    : `Таблица размеров * · заполнено ${sizeChartCompleteness.filledRequiredCellCount} из ${sizeChartCompleteness.requiredCellCount}`}
                </button>
              ) : !sizeChartCompleteness.isComplete ? (
                <button
                  type="button"
                  data-testid="size-chart-required-btn"
                  onClick={() => setIsSizeChartModalOpen(true)}
                  className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                >
                  Таблица размеров *
                </button>
              ) : (
                <button
                  type="button"
                  data-testid="size-chart-btn"
                  onClick={() => setIsSizeChartModalOpen(true)}
                  className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                >
                  {sizeChartCompleteness.requiredCellCount > 0
                    ? `Таблица размеров · заполнено ${sizeChartCompleteness.requiredCellCount} из ${sizeChartCompleteness.requiredCellCount}`
                    : "Таблица размеров"}
                </button>
              )
            )
          }
          characteristicsAddSlot={
            <div className="mt-4 pt-3 border-t border-border-lighter dark:border-white/5 flex items-center justify-between flex-wrap gap-2">
              <div>
                {!draft.categoryId ? (
                  <span className="text-xs text-ash">
                    Сначала выберите категорию товара
                  </span>
                ) : requiredProductAttrs.length > 0 ? (
                  <span
                    data-testid="characteristics-progress-badge"
                    className={cn(
                      "text-xs font-medium",
                      isCharacteristicsAttention
                        ? "text-amber-700 dark:text-amber-400 font-semibold"
                        : filledRequiredCharacteristicsCount === requiredProductAttrs.length
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-ash"
                    )}
                  >
                    {filledRequiredCharacteristicsCount === requiredProductAttrs.length
                      ? `Характеристики · заполнено ${requiredProductAttrs.length} из ${requiredProductAttrs.length}`
                      : isCharacteristicsAttention
                      ? `Характеристики * · заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length}`
                      : `Характеристики * · заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length}`}
                  </span>
                ) : (
                  <span className="text-xs text-ash">Характеристики</span>
                )}
                {isCharacteristicsMissing && (
                  <p
                    data-testid="characteristics-required-helper"
                    className={cn(
                      "text-[11px]",
                      isCharacteristicsAttention
                        ? "text-amber-700/80 dark:text-amber-400/80 font-medium"
                        : "text-ash"
                    )}
                  >
                    Заполните обязательные характеристики категории
                  </p>
                )}
              </div>
              {!draft.categoryId ? (
                <button
                  type="button"
                  data-testid="choose-category-characteristics-btn"
                  onClick={() => {
                    markTouched('category');
                    setCategoryModalOpen(true);
                  }}
                  className="px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors cursor-pointer bg-paper-light dark:bg-white/5 text-graphite dark:text-white border-border-soft dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/10"
                >
                  Выбрать категорию
                </button>
              ) : (
                <button
                  type="button"
                  data-testid="manage-characteristics-btn"
                  onClick={() => {
                    setIsCharacteristicsModalOpen(true);
                  }}
                  className={cn(
                    "px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors cursor-pointer",
                    isCharacteristicsAttention
                      ? "bg-amber-50 dark:bg-amber-950/20 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-800 hover:bg-amber-100"
                      : "bg-paper-light dark:bg-white/5 text-graphite dark:text-white border-border-soft dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/10"
                  )}
                >
                  {isCharacteristicsAttention
                    ? "Заполнить характеристики"
                    : requiredProductAttrs.length > 0
                    ? "Изменить характеристики"
                    : "Настроить характеристики"}
                </button>
              )}
            </div>
          }
          compositionSlot={
            <div className="text-sm text-graphite-light dark:text-white/80 leading-relaxed space-y-3" data-testid="visual-composition-section">
              {/* Composition */}
              {isCompositionMissing ? (
                <div
                  onClick={() => setIsCompositionModalOpen(true)}
                  data-testid="composition-required-container"
                  className={cn(
                    "py-2 px-3 rounded-lg border-l-2 cursor-pointer transition-colors space-y-0.5",
                    isCompositionAttention
                      ? "border-amber-500 bg-amber-50/30 dark:bg-amber-950/15 hover:bg-amber-50/50 dark:hover:bg-amber-950/25"
                      : "border-border-soft bg-paper-light/40 dark:bg-white/[0.02] hover:bg-paper-light dark:hover:bg-white/[0.04]"
                  )}
                >
                  <div className="flex items-center justify-between">
                    <span className={cn(
                      "text-xs font-semibold",
                      isCompositionAttention
                        ? "text-amber-700 dark:text-amber-400"
                        : "text-graphite dark:text-white"
                    )}>
                      Состав *
                    </span>
                    <button
                      type="button"
                      data-testid="add-composition-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        setIsCompositionModalOpen(true);
                      }}
                      className={cn(
                        "text-xs font-semibold underline decoration-dashed cursor-pointer",
                        isCompositionAttention
                          ? "text-amber-700 dark:text-amber-400 hover:text-amber-800"
                          : "text-graphite dark:text-white hover:text-indigo-600"
                      )}
                    >
                      Добавить состав
                    </button>
                  </div>
                  <p
                    data-testid="composition-required-helper"
                    className={cn(
                      "text-xs",
                      isCompositionAttention
                        ? "text-amber-700/80 dark:text-amber-400/80"
                        : "text-ash"
                    )}
                  >
                    Укажите состав товара
                  </p>
                </div>
              ) : (
                <div className="space-y-1">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <span className="text-xs font-medium text-ash block mb-0.5">Состав</span>
                      <p className="text-sm text-graphite dark:text-white font-medium">
                        {draft.materialComposition && draft.materialComposition.length > 0
                          ? draft.materialComposition.map((mc: any) => `${mc.materialName || mc.material} ${mc.percentage}%`).join(', ')
                          : draft.material}
                      </p>
                    </div>
                    <button
                      type="button"
                      data-testid="edit-composition-btn"
                      onClick={() => setIsCompositionModalOpen(true)}
                      className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer shrink-0 mt-0.5"
                    >
                      Изменить
                    </button>
                  </div>
                </div>
              )}

              {/* Care instructions */}
              <div className="pt-3 border-t border-border-lighter dark:border-white/5 space-y-2" data-testid="visual-care-section">
                {draft.careInstructions?.trim() ? (
                  <div className="space-y-1">
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <span className="text-xs font-medium text-ash block mb-0.5">Уход</span>
                        <p className="text-xs text-graphite dark:text-white">
                          {draft.careInstructions}
                        </p>
                      </div>
                      <button
                        type="button"
                        data-testid="edit-care-btn"
                        onClick={() => setIsCareModalOpen(true)}
                        className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer shrink-0 mt-0.5"
                      >
                        Изменить
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-1.5 p-2.5 rounded-lg bg-paper-light/60 dark:bg-white/[0.02] border border-border-soft/60 dark:border-white/5">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-graphite dark:text-white">
                        Уход
                      </span>
                      <span className="text-[11px] text-ash">
                        Необязательно
                      </span>
                    </div>
                    <p className="text-xs text-ash">
                      Рекомендации по стирке, сушке и глажке
                    </p>
                    <button
                      type="button"
                      data-testid="add-care-btn"
                      onClick={() => setIsCareModalOpen(true)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-graphite dark:text-white bg-white dark:bg-white/10 border border-border-soft dark:border-white/10 rounded-lg hover:bg-black/5 dark:hover:bg-white/20 transition-colors cursor-pointer"
                    >
                      Добавить уход
                    </button>
                  </div>
                )}
              </div>
            </div>
          }
        />
      </div>

      <ProductStudioPhotoColorModal
        isOpen={isPhotoColorModalOpen}
        onClose={() => setIsPhotoColorModalOpen(false)}
        images={draft.images || []}
        colors={activeProductColors}
        mediaMode={mediaMode}
        onSave={(updatedImages) => {
          const nextMode = mediaMode === 'GENERAL' ? 'COLORWAY' : mediaMode;
          updateDraft({ images: updatedImages, mediaMode: nextMode });
          markTouched('media');
        }}
      />

      <ProductStudioCompositionModal
        isOpen={isCompositionModalOpen}
        onClose={() => {
          setIsCompositionModalOpen(false);
          markTouched('composition');
        }}
        materialComposition={draft.materialComposition}
        onSave={({ materialComposition, material }) => {
          updateDraft({ materialComposition, material });
          markTouched('composition');
        }}
      />

      <ProductStudioCareModal
        isOpen={isCareModalOpen}
        onClose={() => setIsCareModalOpen(false)}
        careInstructions={draft.careInstructions}
        onSave={(careInstructions) => updateDraft({ careInstructions })}
      />

      <ProductStudioCharacteristicsModal
        isOpen={isCharacteristicsModalOpen}
        onClose={() => {
          setIsCharacteristicsModalOpen(false);
          markTouched('characteristics');
        }}
        schema={categorySchema}
        categoryName={draft.categoryName || categorySchema?.name}
        attributes={draft.attributes}
        onSave={(attributes) => {
          updateDraft({ attributes });
          markTouched('characteristics');
        }}
      />

      <ProductStudioSizeChartModal
        isOpen={isSizeChartModalOpen}
        onClose={() => {
          setIsSizeChartModalOpen(false);
          markTouched('sizeChart');
        }}
        schema={categorySchema}
        categoryName={draft.categoryName || categorySchema?.name}
        draftSizes={offeredSizes.map((s) => ({ id: s.sizeValueId, label: s.sizeValueName }))}
        sizeChart={draft.sizeChart}
        onSaveSizeChart={(sizeChart) => {
          updateDraft({ sizeChart });
          markTouched('sizeChart');
        }}
      />

      {isConfirmToColorwayOpen && (
        <div
          role="dialog"
          aria-modal="true"
          data-testid="confirm-to-colorway-modal"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        >
          <div className="bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-4">
            <h3 className="text-lg font-semibold text-graphite dark:text-white">
              Переключить на фотографии по цветам?
            </h3>
            <p className="text-sm text-ash dark:text-gray-300">
              Фотографии нужно будет распределить по цветам.
            </p>
            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                data-testid="confirm-to-colorway-cancel"
                onClick={() => setIsConfirmToColorwayOpen(false)}
                className="px-4 py-2 text-sm font-medium rounded-lg text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer"
              >
                Отмена
              </button>
              <button
                type="button"
                data-testid="confirm-to-colorway-submit"
                onClick={() => {
                  const transitioned = transitionMediaToColorway(draft.images || [], activeProductColors, draft.variants);
                  updateDraft({ images: transitioned, mediaMode: 'COLORWAY' });
                  if (transitioned.some((img) => img.isUnassigned)) {
                    setSelectedMediaColorId('UNASSIGNED');
                  }
                  markTouched('media');
                  setIsConfirmToColorwayOpen(false);
                }}
                className="px-4 py-2 text-sm font-semibold rounded-lg bg-graphite text-white dark:bg-white dark:text-black hover:opacity-90 transition-opacity cursor-pointer"
              >
                Переключить
              </button>
            </div>
          </div>
        </div>
      )}

      {isConfirmToGeneralOpen && (
        <div
          role="dialog"
          aria-modal="true"
          data-testid="confirm-to-general-modal"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        >
          <div className="bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-4">
            <h3 className="text-lg font-semibold text-graphite dark:text-white">
              Переключить на общую галерею?
            </h3>
            <p className="text-sm text-ash dark:text-gray-300">
              Фотографии разных цветов будут объединены в одну общую галерею.
            </p>
            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                data-testid="confirm-to-general-cancel"
                onClick={() => setIsConfirmToGeneralOpen(false)}
                className="px-4 py-2 text-sm font-medium rounded-lg text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer"
              >
                Отмена
              </button>
              <button
                type="button"
                data-testid="confirm-to-general-submit"
                onClick={() => {
                  const transitioned = transitionMediaToGeneral(draft.images || [], activeProductColors, draft.variants);
                  updateDraft({ images: transitioned, mediaMode: 'GENERAL' });
                  markTouched('media');
                  setIsConfirmToGeneralOpen(false);
                }}
                className="px-4 py-2 text-sm font-semibold rounded-lg bg-graphite text-white dark:bg-white dark:text-black hover:opacity-90 transition-opacity cursor-pointer"
              >
                Переключить
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
