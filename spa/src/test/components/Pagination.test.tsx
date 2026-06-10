import { Pagination } from '@/components/transactions/Pagination';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, test, vi } from 'vitest';

describe('Pagination', () => {
  test('renders page X of Y', () => {
    render(<Pagination total={120} limit={50} offset={50} onChange={() => {}} />);
    expect(screen.getByText('Page 2 of 3')).toBeInTheDocument();
  });

  test('Prev disabled on first page', () => {
    render(<Pagination total={120} limit={50} offset={0} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: /prev/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /next/i })).not.toBeDisabled();
  });

  test('Next disabled on last page', () => {
    render(<Pagination total={120} limit={50} offset={100} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled();
  });

  test('clicking Next calls onChange with next offset', () => {
    const onChange = vi.fn();
    render(<Pagination total={120} limit={50} offset={0} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(onChange).toHaveBeenCalledWith(50);
  });

  test('clicking Prev calls onChange with previous offset', () => {
    const onChange = vi.fn();
    render(<Pagination total={120} limit={50} offset={100} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: /prev/i }));
    expect(onChange).toHaveBeenCalledWith(50);
  });

  test('total <= limit hides pagination', () => {
    const { container } = render(
      <Pagination total={10} limit={50} offset={0} onChange={() => {}} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
