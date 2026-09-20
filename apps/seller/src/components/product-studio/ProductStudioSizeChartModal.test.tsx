/* @vitest-environment jsdom */
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ProductStudioSizeChartModal } from './ProductStudioSizeChartModal';
import type { SellerCategorySchema } from '@zamk/api-client/src/seller';

describe('ProductStudioSizeChartModal', () => {
  afterEach(() => {
    cleanup();
  });

  const mockSchema: SellerCategorySchema = {
    id: 'sch-hoodie',
    slug: 'hoodie',
    name: 'Худи',
    dimensionType: 'COLOR_AND_SIZE',
    sizeChartRequired: true,
    allowedSizeSystems: [{ id: 'sys-ru', code: 'RU', name: 'RU' }],
    attributes: [],
    sizeChartFields: [
      { code: 'chest', name: 'Обхват груди', unit: 'см', isRequired: true, sortOrder: 1 },
      { code: 'length', name: 'Длина изделия', unit: 'см', isRequired: true, sortOrder: 2 },
      { code: 'sleeve', name: 'Длина рукава', unit: 'см', isRequired: false, sortOrder: 3 },
    ],
  };

  const draftSizes = [
    { id: 'sz-s', label: 'S' },
    { id: 'sz-m', label: 'M' },
  ];

  it('renders modal when open with table headers, offered sizes, and progress badge', () => {
    render(
      <ProductStudioSizeChartModal
        isOpen={true}
        onClose={vi.fn()}
        schema={mockSchema}
        draftSizes={draftSizes}
        onSaveSizeChart={vi.fn()}
      />
    );

    expect(screen.getByTestId('size-chart-modal')).toBeTruthy();
    expect(screen.getByText('Таблица размеров и мерки')).toBeTruthy();
    expect(screen.getByText(/Обхват груди/i)).toBeTruthy();
    expect(screen.getByText(/Длина изделия/i)).toBeTruthy();
    expect(screen.getByText(/Длина рукава/i)).toBeTruthy();

    // Headers have units and asterisks
    expect(screen.getByText('Обхват груди')).toBeTruthy();
    expect(screen.getAllByText('(см)').length).toBe(3);

    // Rows for S and M
    expect(screen.getByText('S')).toBeTruthy();
    expect(screen.getByText('M')).toBeTruthy();

    // Inputs exist
    expect(screen.getByTestId('measurement-input-sz-s-chest')).toBeTruthy();
    expect(screen.getByTestId('measurement-input-sz-s-length')).toBeTruthy();
    expect(screen.getByTestId('measurement-input-sz-s-sleeve')).toBeTruthy();
    expect(screen.getByTestId('measurement-input-sz-m-chest')).toBeTruthy();
    expect(screen.getByTestId('measurement-input-sz-m-length')).toBeTruthy();
    expect(screen.getByTestId('measurement-input-sz-m-sleeve')).toBeTruthy();

    // Live progress: 4 required cells (2 sizes * 2 required fields: chest, length)
    expect(screen.getByText(/Заполнено 0 из 4 обязательных мерок/i)).toBeTruthy();
  });

  it('entering measurements updates local state and progress badge', () => {
    render(
      <ProductStudioSizeChartModal
        isOpen={true}
        onClose={vi.fn()}
        schema={mockSchema}
        draftSizes={draftSizes}
        onSaveSizeChart={vi.fn()}
      />
    );

    const sChest = screen.getByTestId('measurement-input-sz-s-chest');
    fireEvent.change(sChest, { target: { value: '95' } });

    expect(screen.getByText(/Заполнено 1 из 4 обязательных мерок/i)).toBeTruthy();

    const sLength = screen.getByTestId('measurement-input-sz-s-length');
    fireEvent.change(sLength, { target: { value: '68' } });

    expect(screen.getByText(/Заполнено 2 из 4 обязательных мерок/i)).toBeTruthy();
  });

  it('cancel button calls onClose without triggering onSaveSizeChart', () => {
    const handleClose = vi.fn();
    const handleSave = vi.fn();

    render(
      <ProductStudioSizeChartModal
        isOpen={true}
        onClose={handleClose}
        schema={mockSchema}
        draftSizes={draftSizes}
        onSaveSizeChart={handleSave}
      />
    );

    const sChest = screen.getByTestId('measurement-input-sz-s-chest');
    fireEvent.change(sChest, { target: { value: '95' } });

    const cancelBtn = screen.getByTestId('size-chart-modal-cancel');
    fireEvent.click(cancelBtn);

    expect(handleClose).toHaveBeenCalledTimes(1);
    expect(handleSave).not.toHaveBeenCalled();
  });

  it('Escape key triggers onClose', () => {
    const handleClose = vi.fn();

    render(
      <ProductStudioSizeChartModal
        isOpen={true}
        onClose={handleClose}
        schema={mockSchema}
        draftSizes={draftSizes}
        onSaveSizeChart={vi.fn()}
      />
    );

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(handleClose).toHaveBeenCalledTimes(1);
  });

  it('save button atomically calls onSaveSizeChart with canonical fields and row measurements', () => {
    const handleClose = vi.fn();
    const handleSave = vi.fn();

    render(
      <ProductStudioSizeChartModal
        isOpen={true}
        onClose={handleClose}
        schema={mockSchema}
        draftSizes={draftSizes}
        onSaveSizeChart={handleSave}
      />
    );

    // Fill all 4 required fields and 1 optional field
    fireEvent.change(screen.getByTestId('measurement-input-sz-s-chest'), { target: { value: '96' } });
    fireEvent.change(screen.getByTestId('measurement-input-sz-s-length'), { target: { value: '70' } });
    fireEvent.change(screen.getByTestId('measurement-input-sz-s-sleeve'), { target: { value: '62' } });

    fireEvent.change(screen.getByTestId('measurement-input-sz-m-chest'), { target: { value: '102' } });
    fireEvent.change(screen.getByTestId('measurement-input-sz-m-length'), { target: { value: '72' } });

    // Progress shows complete
    expect(screen.getByText(/Все обязательные мерки заполнены \(4 из 4\)/i)).toBeTruthy();

    const saveBtn = screen.getByTestId('size-chart-modal-save');
    fireEvent.click(saveBtn);

    expect(handleSave).toHaveBeenCalledTimes(1);
    const savedChart = handleSave.mock.calls[0][0];

    expect(savedChart.fields).toEqual([
      { code: 'chest', name: 'Обхват груди', unit: 'см', isRequired: true, sortOrder: 1 },
      { code: 'length', name: 'Длина изделия', unit: 'см', isRequired: true, sortOrder: 2 },
      { code: 'sleeve', name: 'Длина рукава', unit: 'см', isRequired: false, sortOrder: 3 },
    ]);

    expect(savedChart.rows).toEqual([
      {
        sizeValueId: 'sz-s',
        sizeValueName: 'S',
        measurements: { chest: 96, length: 70, sleeve: 62 },
      },
      {
        sizeValueId: 'sz-m',
        sizeValueName: 'M',
        measurements: { chest: 102, length: 72 },
      },
    ]);
    expect(handleClose).toHaveBeenCalledTimes(1);
  });
});
