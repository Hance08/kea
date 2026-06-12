import { ViewToggle } from '@/components/balances/ViewToggle';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

describe('ViewToggle', () => {
  it('renders a list button and a cards button', () => {
    render(<ViewToggle value="list" onChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: /list view/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cards view/i })).toBeInTheDocument();
  });

  it('marks the active button with aria-pressed=true and the inactive with false', () => {
    render(<ViewToggle value="cards" onChange={vi.fn()} />);
    expect(screen.getByRole('button', { name: /cards view/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: /list view/i })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('calls onChange with the clicked view', async () => {
    const onChange = vi.fn();
    render(<ViewToggle value="list" onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: /cards view/i }));
    expect(onChange).toHaveBeenCalledWith('cards');
    await userEvent.click(screen.getByRole('button', { name: /list view/i }));
    expect(onChange).toHaveBeenCalledWith('list');
  });

  it('does not call onChange when clicking the already-active button', async () => {
    const onChange = vi.fn();
    render(<ViewToggle value="list" onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: /list view/i }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
