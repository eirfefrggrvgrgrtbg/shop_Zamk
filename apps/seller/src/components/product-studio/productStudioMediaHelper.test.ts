import { describe, it, expect } from 'vitest';
import {
  validateImageFile,
  getMediaProgressText,
  MIN_PRODUCT_IMAGES,
  MAX_PRODUCT_IMAGES,
  MAX_FILE_SIZE_BYTES,
  MIN_IMAGE_WIDTH,
  MIN_IMAGE_HEIGHT,
} from './productStudioMediaHelper';

describe('productStudioMediaHelper', () => {
  it('accepts valid JPEG, PNG, and WebP files', async () => {
    const jpgFile = new File(['dummy'], 'photo.jpg', { type: 'image/jpeg' });
    const pngFile = new File(['dummy'], 'photo.png', { type: 'image/png' });
    const webpFile = new File(['dummy'], 'photo.webp', { type: 'image/webp' });

    expect((await validateImageFile(jpgFile)).valid).toBe(true);
    expect((await validateImageFile(pngFile)).valid).toBe(true);
    expect((await validateImageFile(webpFile)).valid).toBe(true);
  });

  it('rejects unsupported file extensions or MIME types with clear Russian copy', async () => {
    const gifFile = new File(['dummy'], 'animation.gif', { type: 'image/gif' });
    const pdfFile = new File(['dummy'], 'doc.pdf', { type: 'application/pdf' });

    const resGif = await validateImageFile(gifFile);
    expect(resGif.valid).toBe(false);
    expect(resGif.error).toBe('Поддерживаются JPG, PNG и WebP');

    const resPdf = await validateImageFile(pdfFile);
    expect(resPdf.valid).toBe(false);
    expect(resPdf.error).toBe('Поддерживаются JPG, PNG и WebP');
  });

  it('rejects files exceeding 10 MB', async () => {
    // 11 MB file
    const largeFile = new File(['x'], 'huge.jpg', { type: 'image/jpeg' });
    Object.defineProperty(largeFile, 'size', { value: MAX_FILE_SIZE_BYTES + 1024 });

    const res = await validateImageFile(largeFile);
    expect(res.valid).toBe(false);
    expect(res.error).toBe('Файл слишком большой — максимум 10 МБ');
  });

  it('enforces canonical backend portrait orientation rule (w < h)', async () => {
    const landscapeFile = new File(['dummy'], 'landscape.jpg', { type: 'image/jpeg' });
    (landscapeFile as any).__dimensions = { width: 1200, height: 800 };

    const resLandscape = await validateImageFile(landscapeFile);
    expect(resLandscape.valid).toBe(false);
    expect(resLandscape.error).toBe(
      'Для товара нужны вертикальные фотографии. Загрузите изображение в вертикальном формате.'
    );

    const squareFile = new File(['dummy'], 'square.jpg', { type: 'image/jpeg' });
    (squareFile as any).__dimensions = { width: 1000, height: 1000 };

    const resSquare = await validateImageFile(squareFile);
    expect(resSquare.valid).toBe(false);
    expect(resSquare.error).toBe(
      'Для товара нужны вертикальные фотографии. Загрузите изображение в вертикальном формате.'
    );
  });

  it('enforces canonical backend minimum dimension rule (min 800×1000 px)', async () => {
    const smallFile = new File(['dummy'], 'small.jpg', { type: 'image/jpeg' });
    (smallFile as any).__dimensions = { width: 600, height: 800 }; // w < h, but both below min

    const resSmall = await validateImageFile(smallFile);
    expect(resSmall.valid).toBe(false);
    expect(resSmall.error).toBe(
      'Изображение слишком маленькое. Минимальный размер — 800×1000 пикселей.'
    );
  });

  it('accepts valid vertical image satisfying canonical dimensions (>= 800×1000 px)', async () => {
    const validFile = new File(['dummy'], 'valid.jpg', { type: 'image/jpeg' });
    (validFile as any).__dimensions = { width: MIN_IMAGE_WIDTH, height: MIN_IMAGE_HEIGHT };

    const res = await validateImageFile(validFile);
    expect(res.valid).toBe(true);
    expect(res.error).toBeUndefined();
  });

  it('accepts non-4:5 portrait image such as 900×1200 satisfying portrait orientation and min 800×1000 px', async () => {
    const portraitFile = new File(['dummy'], 'portrait_900_1200.jpg', { type: 'image/jpeg' });
    (portraitFile as any).__dimensions = { width: 900, height: 1200 };

    const res = await validateImageFile(portraitFile);
    expect(res.valid).toBe(true);
    expect(res.error).toBeUndefined();
  });

  it('generates canonical progress text across counts aligned with MIN=3 and MAX=8', () => {
    expect(getMediaProgressText(0)).toBe(`Фото * · 0 из ${MIN_PRODUCT_IMAGES} минимум`);
    expect(getMediaProgressText(1)).toBe(`Фото 1 из ${MIN_PRODUCT_IMAGES}`);
    expect(getMediaProgressText(2)).toBe(`Фото 2 из ${MIN_PRODUCT_IMAGES}`);
    expect(getMediaProgressText(3)).toBe(`Фото ${MIN_PRODUCT_IMAGES} из ${MIN_PRODUCT_IMAGES} · готово`);
    expect(getMediaProgressText(5)).toBe(`Фото 5 из ${MIN_PRODUCT_IMAGES} · готово`);
    expect(getMediaProgressText(MAX_PRODUCT_IMAGES)).toBe(`Фото ${MAX_PRODUCT_IMAGES} из ${MAX_PRODUCT_IMAGES} · максимум`);
  });
});
