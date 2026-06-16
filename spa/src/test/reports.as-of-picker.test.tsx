import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import { AsOfPicker } from '../components/reports/AsOfPicker';

test('renders "Now" when value is undefined', () => {
  const noop = (_: number | undefined) => {};
  render(<AsOfPicker label="As of" value={undefined} onChange={noop} />);
  expect(screen.getByText(/now/i)).toBeInTheDocument();
});

test('selecting a date emits Unix seconds', async () => {
  const calls: (number | undefined)[] = [];
  render(<AsOfPicker label="As of" value={undefined} onChange={(v) => calls.push(v)} />);
  const input = screen.getByLabelText(/as of/i) as HTMLInputElement;
  await userEvent.type(input, '2026-06-15');
  // The input change emits at least once with a positive Unix number
  expect(calls.some((c) => typeof c === 'number' && c > 0)).toBe(true);
});

test('clicking "Now" emits undefined', async () => {
  const calls: (number | undefined)[] = [];
  // Start with a specific time so "Now" is a transition.
  const t = Math.floor(new Date('2026-06-01T00:00:00Z').getTime() / 1000);
  render(<AsOfPicker label="As of" value={t} onChange={(v) => calls.push(v)} />);
  await userEvent.click(screen.getByRole('button', { name: /now/i }));
  expect(calls).toEqual([undefined]);
});
