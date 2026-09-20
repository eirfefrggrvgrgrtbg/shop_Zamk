/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { ProductStudioCompositionModal } from './ProductStudioCompositionModal';

vi.mock('@zamk/api-client/src/seller', () => {
  return {
    getSellerMaterials: vi.fn().mockImplementation(() =>
      Promise.resolve([
        { id: 'mat-cotton', code: 'COTTON', nameRu: 'Хлопок' },
        { id: 'mat-cashmere', code: 'CASHMERE', nameRu: 'Кашемир' },
        { id: 'mat-silk', code: 'SILK', nameRu: 'Шелк' },
        { id: 'mat-viscose', code: 'VISCOSE', nameRu: 'Вискоза' },
        { id: 'mat-lyocell', code: 'LYOCELL', nameRu: 'Лиоцелл' },
        { id: 'mat-poly', code: 'POLYESTER', nameRu: 'Полиэстер' },
        { id: 'mat-polyamide', code: 'POLYAMIDE', nameRu: 'Полиамид' },
        { id: 'mat-polyurethane', code: 'POLYURETHANE', nameRu: 'Полиуретан' },
        { id: 'mat-wool', code: 'WOOL', nameRu: 'Шерсть' },
      ])
    ),
  };
});

describe('ProductStudioCompositionModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  const pickMaterial = (rowIndex: number, materialId: string) => {
    const trigger = screen.getByTestId(`composition-material-trigger-${rowIndex}`);
    fireEvent.click(trigger);
    const option = screen.getByTestId(`material-option-${materialId}`);
    fireEvent.click(option);
  };

  it('A: empty modal opens with one editable row already present (no dashed box, no native select)', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[]}
      />
    );

    // Modal title & subtitle
    expect(screen.getByText('Состав')).toBeTruthy();
    expect(screen.getByText('Укажите материалы и их доли')).toBeTruthy();

    // First row exists immediately
    await waitFor(() => {
      expect(screen.getByTestId('composition-row-0')).toBeTruthy();
      expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      expect(screen.getByTestId('composition-percentage-input-0')).toBeTruthy();
    });

    // Zero native <select> elements in DOM
    expect(document.querySelector('select')).toBeNull();

    // Apply button starts disabled
    const applyBtn = screen.getByTestId('composition-modal-apply') as HTMLButtonElement;
    expect(applyBtn.disabled).toBe(true);
  });

  it('B: "Добавить материал" adds a new editable row and opens combobox', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-row-0')).toBeTruthy();
    });

    const addBtn = screen.getByTestId('add-material-row-button');
    fireEvent.click(addBtn);

    await waitFor(() => {
      expect(screen.getByTestId('composition-row-1')).toBeTruthy();
      expect(screen.getByTestId('composition-material-trigger-1')).toBeTruthy();
      expect(screen.getByTestId('composition-percentage-input-1')).toBeTruthy();
    });
  });

  it('C & L: selecting materials and percentages preserves canonical material IDs and applies atomically', async () => {
    const onSave = vi.fn();
    const onClose = vi.fn();

    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={onClose}
        onSave={onSave}
        materialComposition={[]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
    });

    // Row 0: Cotton 80%
    pickMaterial(0, 'mat-cotton');
    fireEvent.change(screen.getByTestId('composition-percentage-input-0'), {
      target: { value: '80' },
    });

    // Add Row 1: Polyester 20%
    fireEvent.click(screen.getByTestId('add-material-row-button'));
    await waitFor(() => {
      expect(screen.getByTestId('composition-material-trigger-1')).toBeTruthy();
    });

    pickMaterial(1, 'mat-poly');
    fireEvent.change(screen.getByTestId('composition-percentage-input-1'), {
      target: { value: '20' },
    });

    // Check sum and status
    expect(screen.getByTestId('composition-sum-value').textContent).toBe('100%');
    expect(screen.getByTestId('composition-sum-status').textContent).toBe('Состав заполнен');

    const applyBtn = screen.getByTestId('composition-modal-apply') as HTMLButtonElement;
    expect(applyBtn.disabled).toBe(false);

    fireEvent.click(applyBtn);

    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith({
      materialComposition: [
        { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 },
        { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 20 },
      ],
      material: 'Хлопок — 80%, Полиэстер — 20%',
    });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('D: duplicate material selection is disabled in subsequent selectors', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
    });

    // Select cotton on row 0
    pickMaterial(0, 'mat-cotton');

    // Add row 1
    fireEvent.click(screen.getByTestId('add-material-row-button'));
    await waitFor(() => {
      expect(screen.getByTestId('composition-material-trigger-1')).toBeTruthy();
    });

    // Open row 1 combobox
    fireEvent.click(screen.getByTestId('composition-material-trigger-1'));

    // In row 1 combobox, cotton option should be disabled
    const cottonOption = screen.getByTestId('material-option-mat-cotton') as HTMLButtonElement;
    expect(cottonOption.disabled).toBe(true);
    expect(cottonOption.textContent).toContain('Уже добавлен');

    // Other option should be enabled
    const polyOption = screen.getByTestId('material-option-mat-poly') as HTMLButtonElement;
    expect(polyOption.disabled).toBe(false);
  });

  it('E: 80 + 20: sum = 100, Apply enabled', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[
          { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 },
          { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 20 },
        ]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-sum-value').textContent).toBe('100%');
      expect(screen.getByTestId('composition-sum-status').textContent).toBe('Состав заполнен');
      expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(false);
    });
  });

  it('F: 60 + 20: sum = 80, Apply disabled with remainder message', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[
          { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 60 },
          { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 20 },
        ]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-sum-value').textContent).toBe('80%');
      expect(screen.getByTestId('composition-sum-status').textContent).toBe('Нужно добавить ещё 20%');
      expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(true);
    });
  });

  it('G: 80 + 30: sum = 110, Apply disabled with overflow message', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[
          { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 },
          { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 30 },
        ]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-sum-value').textContent).toBe('110%');
      expect(screen.getByTestId('composition-sum-status').textContent).toBe('Сумма превышает 100% на 10%');
      expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(true);
    });
  });

  it('H & I: missing material or percentage leaves Apply disabled even if total looks like 100', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
    });

    // Enter percentage 100 without selecting material
    fireEvent.change(screen.getByTestId('composition-percentage-input-0'), {
      target: { value: '100' },
    });

    expect(screen.getByTestId('composition-sum-value').textContent).toBe('100%');
    expect(screen.getByTestId('composition-sum-status').textContent).toBe('Заполните все строки состава');
    expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(true);

    // Select material but clear percentage
    pickMaterial(0, 'mat-cotton');
    fireEvent.change(screen.getByTestId('composition-percentage-input-0'), {
      target: { value: '' },
    });

    expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(true);
  });

  it('J: delete row recalculates sum immediately and resetting last row clears it', async () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[
          { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 },
          { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 20 },
        ]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-sum-value').textContent).toBe('100%');
    });

    // Delete row 1 (Polyester 20%)
    fireEvent.click(screen.getByTestId('composition-remove-row-1'));

    expect(screen.queryByTestId('composition-row-1')).toBeNull();
    expect(screen.getByTestId('composition-sum-value').textContent).toBe('80%');
    expect(screen.getByTestId('composition-sum-status').textContent).toBe('Нужно добавить ещё 20%');

    // Delete the only remaining row (resets it to empty)
    fireEvent.click(screen.getByTestId('composition-remove-row-0'));
    expect(screen.getByTestId('composition-row-0')).toBeTruthy();
    expect(screen.getByTestId('composition-material-trigger-0').textContent).toContain('Выберите материал');
    expect((screen.getByTestId('composition-percentage-input-0') as HTMLInputElement).value).toBe('');
    expect(screen.getByTestId('composition-sum-value').textContent).toBe('0%');
  });

  it('K: cancel button discards pending edits without mutating draft', async () => {
    const onSave = vi.fn();
    const onClose = vi.fn();

    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={onClose}
        onSave={onSave}
        materialComposition={[
          { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 100 },
        ]}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId('composition-percentage-input-0')).toBeTruthy();
    });

    // Change 100 -> 50
    fireEvent.change(screen.getByTestId('composition-percentage-input-0'), {
      target: { value: '50' },
    });

    const cancelBtn = screen.getByTestId('composition-modal-cancel');
    fireEvent.click(cancelBtn);

    expect(onSave).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('P: production UI contains zero occurrences of free-text alternative or care copy', () => {
    render(
      <ProductStudioCompositionModal
        isOpen={true}
        onClose={vi.fn()}
        onSave={vi.fn()}
        materialComposition={[]}
      />
    );

    expect(screen.queryByText(/Альтернатива/i)).toBeNull();
    expect(screen.queryByText(/Дополнение/i)).toBeNull();
    expect(screen.queryByText(/Текстовое описание состава/i)).toBeNull();
    expect(screen.queryByText(/Уход за изделием/i)).toBeNull();
    expect(screen.queryByTestId('composition-text-input')).toBeNull();
    expect(screen.queryByTestId('care-instructions-input')).toBeNull();
  });

  describe('Section 16: Percent Input Natural Behavior Tests', () => {
    it('16.A: focus percentage, type "80" => exact value "80"', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );
      await waitFor(() => {
        expect(screen.getByTestId('composition-percentage-input-0')).toBeTruthy();
      });
      const input = screen.getByTestId('composition-percentage-input-0') as HTMLInputElement;

      fireEvent.change(input, { target: { value: '8' } });
      expect(input.value).toBe('8');

      fireEvent.change(input, { target: { value: '80' } });
      expect(input.value).toBe('80');
    });

    it('16.B: clear input => value ""', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[{ materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 }]}
        />
      );
      await waitFor(() => {
        expect(screen.getByTestId('composition-percentage-input-0')).toBeTruthy();
      });
      const input = screen.getByTestId('composition-percentage-input-0') as HTMLInputElement;
      expect(input.value).toBe('80');

      fireEvent.change(input, { target: { value: '' } });
      expect(input.value).toBe('');
    });

    it('16.C: paste/type "20" => "20"', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );
      await waitFor(() => {
        expect(screen.getByTestId('composition-percentage-input-0')).toBeTruthy();
      });
      const input = screen.getByTestId('composition-percentage-input-0') as HTMLInputElement;

      fireEvent.change(input, { target: { value: '20' } });
      expect(input.value).toBe('20');
    });

    it('16.D: type "101" => represented as invalid, NOT silently changed to "100"', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );
      await waitFor(() => {
        expect(screen.getByTestId('composition-percentage-input-0')).toBeTruthy();
      });
      const input = screen.getByTestId('composition-percentage-input-0') as HTMLInputElement;

      fireEvent.change(input, { target: { value: '101' } });
      expect(input.value).toBe('101');
      // Must have red error border styling
      expect(input.className).toContain('border-red-500');

      const applyBtn = screen.getByTestId('composition-modal-apply') as HTMLButtonElement;
      expect(applyBtn.disabled).toBe(true);
    });

    it('16.E: 80 + 20 => Apply enabled', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[
            { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 },
            { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 20 },
          ]}
        />
      );
      await waitFor(() => {
        const applyBtn = screen.getByTestId('composition-modal-apply') as HTMLButtonElement;
        expect(applyBtn.disabled).toBe(false);
      });
    });

    it('16.F: 80 + 30 => Apply disabled', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[
            { materialId: 'mat-cotton', materialName: 'Хлопок', percentage: 80 },
            { materialId: 'mat-poly', materialName: 'Полиэстер', percentage: 30 },
          ]}
        />
      );
      await waitFor(() => {
        const applyBtn = screen.getByTestId('composition-modal-apply') as HTMLButtonElement;
        expect(applyBtn.disabled).toBe(true);
      });
    });
  });

  describe('Section 17: Material Combobox Tests', () => {
    it('click opens custom popover, search filters values, selection writes canonical ID, Escape closes', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      });

      // No native select in DOM
      expect(document.querySelector('select')).toBeNull();

      // Click trigger opens custom popover
      const trigger = screen.getByTestId('composition-material-trigger-0');
      fireEvent.click(trigger);

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-popover-0')).toBeTruthy();
        expect(screen.getByTestId('material-search-input')).toBeTruthy();
      });

      // Search filters values: typing "шер" should show Шерсть and hide Хлопок
      const searchInput = screen.getByTestId('material-search-input');
      fireEvent.change(searchInput, { target: { value: 'шер' } });

      expect(screen.getByTestId('material-option-mat-wool')).toBeTruthy();
      expect(screen.queryByTestId('material-option-mat-cotton')).toBeNull();

      // Click option writes material to row and closes popover
      fireEvent.click(screen.getByTestId('material-option-mat-wool'));
      expect(screen.queryByTestId('composition-material-popover-0')).toBeNull();
      expect(trigger.textContent).toContain('Шерсть');

      // Re-opening and pressing Escape closes popover
      fireEvent.click(trigger);
      expect(screen.getByTestId('composition-material-popover-0')).toBeTruthy();
      fireEvent.keyDown(window, { key: 'Escape' });
      expect(screen.queryByTestId('composition-material-popover-0')).toBeNull();
    });

    it('search regression for expanded fashion catalog: каш, поли, лио', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('composition-material-trigger-0'));

      await waitFor(() => {
        expect(screen.getByTestId('material-search-input')).toBeTruthy();
      });

      const searchInput = screen.getByTestId('material-search-input');

      // "каш" -> Кашемир
      fireEvent.change(searchInput, { target: { value: 'каш' } });
      expect(screen.getByTestId('material-option-mat-cashmere')).toBeTruthy();
      expect(screen.queryByTestId('material-option-mat-cotton')).toBeNull();

      // "поли" -> Полиэстер, Полиамид, Полиуретан
      fireEvent.change(searchInput, { target: { value: 'поли' } });
      expect(screen.getByTestId('material-option-mat-poly')).toBeTruthy();
      expect(screen.getByTestId('material-option-mat-polyamide')).toBeTruthy();
      expect(screen.getByTestId('material-option-mat-polyurethane')).toBeTruthy();
      expect(screen.queryByTestId('material-option-mat-cotton')).toBeNull();

      // "лио" -> Лиоцелл
      fireEvent.change(searchInput, { target: { value: 'лио' } });
      expect(screen.getByTestId('material-option-mat-lyocell')).toBeTruthy();
      expect(screen.queryByTestId('material-option-mat-poly')).toBeNull();
    });
  });

  describe('Section 18: Add Material Regression Test', () => {
    it('exact flow: open empty, pick Cotton, 80, click add material -> row 2, pick Poly, 20 -> 100%, Apply enabled, Add Material disabled', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      });

      // Choose Cotton
      pickMaterial(0, 'mat-cotton');

      // Enter 80
      fireEvent.change(screen.getByTestId('composition-percentage-input-0'), {
        target: { value: '80' },
      });

      // Add Material button is enabled
      const addBtn = screen.getByTestId('add-material-row-button') as HTMLButtonElement;
      expect(addBtn.disabled).toBe(false);

      // Click "+ Добавить материал" -> row 2 appears
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(screen.getByTestId('composition-row-1')).toBeTruthy();
        expect(screen.getByTestId('composition-material-trigger-1')).toBeTruthy();
      });

      // Choose Polyester
      pickMaterial(1, 'mat-poly');

      // Enter 20
      fireEvent.change(screen.getByTestId('composition-percentage-input-1'), {
        target: { value: '20' },
      });

      // Assertions: 100%, Apply enabled, Add Material disabled
      expect(screen.getByTestId('composition-sum-value').textContent).toBe('100%');
      expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(false);
      expect(addBtn.disabled).toBe(true);
      expect(addBtn.title).toBe('Состав уже составляет 100%');
    });
  });

  describe('Section 16: Simplified Geometry & Popover Behavior', () => {
    it('popover is rendered via portal (not inside row DOM) and does not disrupt row flow', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      });

      // Opening material picker does NOT add an extra row
      expect(screen.getAllByTestId(/composition-row-/).length).toBe(1);

      // Open picker
      fireEvent.click(screen.getByTestId('composition-material-trigger-0'));

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-popover-0')).toBeTruthy();
      });

      // Popover is NOT rendered inside row flow
      const row0 = screen.getByTestId('composition-row-0');
      const popover = screen.getByTestId('composition-material-popover-0');
      expect(row0.contains(popover)).toBe(false);
      expect(document.body.contains(popover)).toBe(true);

      // Row count is still exactly 1
      expect(screen.getAllByTestId(/composition-row-/).length).toBe(1);

      // Percentage input stays present and editable while picker is open
      const percentInput = screen.getByTestId('composition-percentage-input-0');
      expect(percentInput).toBeTruthy();
      fireEvent.change(percentInput, { target: { value: '80' } });
      expect((percentInput as HTMLInputElement).value).toBe('80');

      // Escape closes picker
      fireEvent.keyDown(window, { key: 'Escape' });
      expect(screen.queryByTestId('composition-material-popover-0')).toBeNull();

      // Outside click closes picker
      fireEvent.click(screen.getByTestId('composition-material-trigger-0'));
      expect(screen.getByTestId('composition-material-popover-0')).toBeTruthy();
      fireEvent.mouseDown(document.body);
      expect(screen.queryByTestId('composition-material-popover-0')).toBeNull();
    });

    it('+ Добавить материал creates a new row with its own picker trigger and percentage input, 80 + 20 enables Apply', async () => {
      render(
        <ProductStudioCompositionModal
          isOpen={true}
          onClose={vi.fn()}
          onSave={vi.fn()}
          materialComposition={[]}
        />
      );

      await waitFor(() => {
        expect(screen.getByTestId('composition-material-trigger-0')).toBeTruthy();
      });

      // Select Cotton on row 0 and set 80
      pickMaterial(0, 'mat-cotton');
      fireEvent.change(screen.getByTestId('composition-percentage-input-0'), { target: { value: '80' } });

      // Click '+ Добавить материал'
      const addBtn = screen.getByTestId('add-material-row-button');
      fireEvent.click(addBtn);

      await waitFor(() => {
        expect(screen.getByTestId('composition-row-1')).toBeTruthy();
        expect(screen.getByTestId('composition-material-trigger-1')).toBeTruthy();
        expect(screen.getByTestId('composition-percentage-input-1')).toBeTruthy();
      });

      // Select Poly on row 1 and set 20
      pickMaterial(1, 'mat-poly');
      fireEvent.change(screen.getByTestId('composition-percentage-input-1'), { target: { value: '20' } });

      // 80 + 20 enables Apply button
      expect(screen.getByTestId('composition-sum-value').textContent).toBe('100%');
      expect((screen.getByTestId('composition-modal-apply') as HTMLButtonElement).disabled).toBe(false);
    });
  });
});

