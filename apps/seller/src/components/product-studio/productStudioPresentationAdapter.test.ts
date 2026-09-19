import { describe, it, expect } from "vitest";
import {
  mapStudioDraftToPresentation,
  computePresentationSizes,
  findMatchingDraftVariant,
  mapToPresentationSelectedVariant,
  findFirstMediaIndexForColor,
  STUDIO_PREVIEW_PLACEHOLDER_IMAGE,
} from "./productStudioPresentationAdapter";
import type { ProductStudioDraft } from "../../contexts/ProductStudioContext";

describe("productStudioPresentationAdapter", () => {
  it("maps title, brand, description, and empty title fallback", () => {
    const emptyDraft: ProductStudioDraft = {
      title: "",
      description: "",
      categoryId: "cat-1",
    };
    const emptyRes = mapStudioDraftToPresentation(emptyDraft);
    expect(emptyRes.product.name).toBe("Название товара");
    expect(emptyRes.product.brand).toBeUndefined();

    const filledDraft: ProductStudioDraft = {
      title: "  Летнее платье  ",
      brandName: "Acne Studios",
      brandId: "brand-123",
      categoryName: "Платья",
      description: "Легкое платье из шелка",
      material: "100% шелк",
      materialComposition: [{ materialName: "Шелк", percentage: 100 }],
      careInstructions: "Ручная стирка",
      categoryId: "cat-1",
    };
    const filledRes = mapStudioDraftToPresentation(filledDraft);
    expect(filledRes.product.name).toBe("  Летнее платье  ");
    expect(filledRes.product.brand).toBe("Acne Studios");
    expect(filledRes.product.brandId).toBe("brand-123");
    expect(filledRes.product.category).toBe("Платья");
    expect(filledRes.product.description).toBe("Легкое платье из шелка");
    expect(filledRes.product.materials).toBe("100% шелк");
    expect(filledRes.product.materialComposition).toEqual([
      { materialName: "Шелк", material: "Шелк", percentage: 100 },
    ]);
    expect(filledRes.product.careInstructions).toBe("Ручная стирка");
  });

  it("maps price cents to rubles exactly once without mixing currencies and follows semantic exact mapping", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      priceCents: 450000, // 4500 руб (new selling price)
      oldPriceCents: 600000, // 6000 руб (old base price)
    };
    const res = mapStudioDraftToPresentation(draft);
    expect(res.basePrice).toBe(6000);
    expect(res.product.price).toBe(6000);
    expect(res.product.discountPrice).toBe(4500);

    const noPriceDraft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
    };
    const noPriceRes = mapStudioDraftToPresentation(noPriceDraft);
    expect(noPriceRes.basePrice).toBe(0);
    expect(noPriceRes.product.price).toBe(0);
    expect(noPriceRes.product.discountPrice).toBeUndefined();
  });

  it("sorts media respecting isMain and sortOrder, and handles empty media state", () => {
    const emptyDraft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      images: [],
    };
    const emptyRes = mapStudioDraftToPresentation(emptyDraft);
    expect(emptyRes.visibleImages).toEqual([{ url: STUDIO_PREVIEW_PLACEHOLDER_IMAGE }]);

    const draftWithImages: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      images: [
        { url: "img-3.jpg", sortOrder: 3, colorId: "c-black" },
        { url: "img-1.jpg", sortOrder: 1, isMain: true, colorId: "c-white" },
        { url: "img-2.jpg", sortOrder: 2, colorId: "c-black" },
      ],
    };
    const res = mapStudioDraftToPresentation(draftWithImages);
    expect(res.visibleImages).toHaveLength(3);
    expect(res.visibleImages[0].url).toBe("img-1.jpg");
    expect(res.visibleImages[0].colorId).toBe("c-white");
    expect(res.visibleImages[1].url).toBe("img-2.jpg");
    expect(res.visibleImages[2].url).toBe("img-3.jpg");
  });

  it("identifies dimension type and extracts canonical colors and sizes", () => {
    // COLOR_AND_SIZE
    const colorAndSizeDraft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c1", colorName: "Черный", colorHex: "#000", sizeValueId: "s1", size: "S" },
        { id: "v2", colorId: "c1", colorName: "Черный", colorHex: "#000", sizeValueId: "s2", size: "M" },
        { id: "v3", colorId: "c2", colorName: "Белый", colorHex: "#fff", sizeValueId: "s1", size: "S" },
      ],
    };
    const casRes = mapStudioDraftToPresentation(colorAndSizeDraft);
    expect(casRes.dimensionType).toBe("COLOR_AND_SIZE");
    expect(casRes.colors).toEqual([
      { id: "c1", name: "Черный", hex: "#000", hasInStock: true },
      { id: "c2", name: "Белый", hex: "#fff", hasInStock: true },
    ]);
    expect(casRes.uniqueSizes).toEqual([
      { id: "s1", label: "S" },
      { id: "s2", label: "M" },
    ]);

    // COLOR_ONLY
    const colorOnlyDraft: ProductStudioDraft = {
      title: "Помада",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c1", colorName: "Красный", colorHex: "#f00" },
        { id: "v2", colorId: "c2", colorName: "Розовый", colorHex: "#ffc0cb" },
      ],
    };
    expect(mapStudioDraftToPresentation(colorOnlyDraft).dimensionType).toBe("COLOR_ONLY");

    // SIZE_ONLY
    const sizeOnlyDraft: ProductStudioDraft = {
      title: "Кольцо",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", sizeValueId: "sz1", size: "16" },
        { id: "v2", sizeValueId: "sz2", size: "17" },
      ],
    };
    expect(mapStudioDraftToPresentation(sizeOnlyDraft).dimensionType).toBe("SIZE_ONLY");

    // SINGLE_VARIANT (1 variant)
    const singleDraft: ProductStudioDraft = {
      title: "Книга",
      description: "",
      categoryId: "cat-1",
      variants: [{ id: "v1" }],
    };
    expect(mapStudioDraftToPresentation(singleDraft).dimensionType).toBe("SINGLE_VARIANT");
    expect(mapStudioDraftToPresentation(singleDraft).hasVariants).toBe(true);

    // EMPTY DRAFT (0 variants)
    const emptyVariantsDraft: ProductStudioDraft = {
      title: "Пусто",
      description: "",
      categoryId: "cat-1",
      variants: [],
    };
    expect(mapStudioDraftToPresentation(emptyVariantsDraft).dimensionType).toBe("SINGLE_VARIANT");
    expect(mapStudioDraftToPresentation(emptyVariantsDraft).hasVariants).toBe(false);
  });

  it("computes size matrix correctly: disabled before color, NOT_OFFERED for missing, NO fake SOLD_OUT", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c1", sizeValueId: "s1", size: "S" },
        { id: "v2", colorId: "c1", sizeValueId: "s2", size: "M" },
        { id: "v3", colorId: "c2", sizeValueId: "s1", size: "S" }, // c2 has only S, no M
      ],
    };
    const uniqueSizes = [
      { id: "s1", label: "S" },
      { id: "s2", label: "M" },
    ];

    // Before color selection: all sizes visible but disabled
    const beforeColor = computePresentationSizes(draft, "COLOR_AND_SIZE", uniqueSizes, null);
    expect(beforeColor).toEqual([
      { id: "s1", label: "S", state: "AVAILABLE", disabled: true },
      { id: "s2", label: "M", state: "AVAILABLE", disabled: true },
    ]);

    // After selecting c1: both S and M are AVAILABLE and enabled
    const c1Sizes = computePresentationSizes(draft, "COLOR_AND_SIZE", uniqueSizes, "c1");
    expect(c1Sizes).toEqual([
      { id: "s1", label: "S", state: "AVAILABLE", disabled: false },
      { id: "s2", label: "M", state: "AVAILABLE", disabled: false },
    ]);

    // After selecting c2: S is AVAILABLE, M is NOT_OFFERED
    const c2Sizes = computePresentationSizes(draft, "COLOR_AND_SIZE", uniqueSizes, "c2");
    expect(c2Sizes).toEqual([
      { id: "s1", label: "S", state: "AVAILABLE", disabled: false },
      { id: "s2", label: "M", state: "NOT_OFFERED", disabled: true },
    ]);

    // Verify no SOLD_OUT is produced anywhere
    expect(c2Sizes.some((s) => s.state === "SOLD_OUT")).toBe(false);
  });

  it("findFirstMediaIndexForColor correctly locates matching media or falls back to 0", () => {
    const images = [
      { url: "main.jpg" },
      { url: "black-1.jpg", colorId: "black" },
      { url: "white-1.jpg", colorId: "white" },
      { url: "black-2.jpg", colorId: "black" },
    ];

    expect(findFirstMediaIndexForColor(images, null)).toBe(0);
    expect(findFirstMediaIndexForColor(images, "black")).toBe(1);
    expect(findFirstMediaIndexForColor(images, "white")).toBe(2);
    expect(findFirstMediaIndexForColor(images, "green")).toBe(0); // fallback
  });

  it("findMatchingDraftVariant and mapToPresentationSelectedVariant resolve correctly", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c1", sizeValueId: "s1", size: "S", sellerSku: "SKU-S", priceCents: 200000 },
        { id: "v2", colorId: "c1", sizeValueId: "s2", size: "M", sellerSku: "SKU-M", priceCents: 220000 },
      ],
    };

    const matched = findMatchingDraftVariant(draft, "COLOR_AND_SIZE", "c1", "s1");
    expect(matched?.id).toBe("v1");

    const mapped = mapToPresentationSelectedVariant(matched);
    expect(mapped).toEqual({
      id: "v1",
      sellerSku: "SKU-S",
      sku: "SKU-S",
      priceCents: 200000,
      size: "S",
    });

    expect(mapToPresentationSelectedVariant(null)).toBeNull();
  });
});
