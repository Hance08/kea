import { MismatchDialog } from '@/components/reconcile/MismatchDialog';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { withServerConfig } from '../test-app';

describe('MismatchDialog', () => {
  it('renders nothing when open is false', () => {
    render(
      withServerConfig(
        <MismatchDialog
          open={false}
          differenceCents={1000}
          pending={false}
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      ),
    );
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  });

  it('renders "short" copy when difference is positive', () => {
    render(
      withServerConfig(
        <MismatchDialog
          open={true}
          differenceCents={1000}
          pending={false}
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      ),
    );
    expect(screen.getByText(/short \$10\.00/i)).toBeInTheDocument();
  });

  it('renders "over" copy when difference is negative', () => {
    render(
      withServerConfig(
        <MismatchDialog
          open={true}
          differenceCents={-1000}
          pending={false}
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      ),
    );
    expect(screen.getByText(/over \$10\.00/i)).toBeInTheDocument();
  });

  it('fires onCancel when Cancel is clicked', () => {
    const onCancel = vi.fn();
    render(
      withServerConfig(
        <MismatchDialog
          open={true}
          differenceCents={1000}
          pending={false}
          onCancel={onCancel}
          onConfirm={vi.fn()}
        />,
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('fires onConfirm when "Reconcile anyway" is clicked', () => {
    const onConfirm = vi.fn();
    render(
      withServerConfig(
        <MismatchDialog
          open={true}
          differenceCents={1000}
          pending={false}
          onCancel={vi.fn()}
          onConfirm={onConfirm}
        />,
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /reconcile anyway/i }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('fires onCancel when ESC is pressed', () => {
    const onCancel = vi.fn();
    render(
      withServerConfig(
        <MismatchDialog
          open={true}
          differenceCents={1000}
          pending={false}
          onCancel={onCancel}
          onConfirm={vi.fn()}
        />,
      ),
    );
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('disables both buttons while pending', () => {
    render(
      withServerConfig(
        <MismatchDialog
          open={true}
          differenceCents={1000}
          pending={true}
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      ),
    );
    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /reconcile anyway/i })).toBeDisabled();
  });
});
