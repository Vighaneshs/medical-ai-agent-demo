import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { IntakeForm } from '@/components/IntakeForm';

// IntakeForm does not use framer-motion, no mock needed.

// Helper: find the date-of-birth input (type="date", no placeholder).
// The label has no htmlFor, so we locate it via DOM sibling.
function getDobInput(): HTMLInputElement {
  return document.querySelector('input[type="date"]') as HTMLInputElement;
}

describe('IntakeForm', () => {
  const mockSubmit = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();
  });

  // ── 1. Renders all form fields ─────────────────────────────────────────────

  it('renders all form fields', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    expect(screen.getByPlaceholderText('Jane')).toBeInTheDocument();               // firstName
    expect(screen.getByPlaceholderText('Smith')).toBeInTheDocument();              // lastName
    expect(getDobInput()).toBeInTheDocument();                                       // dob (type="date")
    expect(screen.getByPlaceholderText('(555) 000-0000')).toBeInTheDocument();     // phone
    expect(screen.getByPlaceholderText('jane@example.com')).toBeInTheDocument();   // email
    expect(screen.getByPlaceholderText(/describe your symptoms/i)).toBeInTheDocument(); // reason
  });

  // ── 2. DOB input has max set to today ─────────────────────────────────────

  it('dob input has max set to today', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);
    const today = new Date().toISOString().split('T')[0];
    expect(getDobInput()).toHaveAttribute('max', today);
  });

  // ── 3. Does not show errors before submit ─────────────────────────────────

  it('does not show errors before submit', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    const inputs = [
      screen.getByPlaceholderText('Jane'),
      screen.getByPlaceholderText('Smith'),
      getDobInput(),
      screen.getByPlaceholderText('(555) 000-0000'),
      screen.getByPlaceholderText('jane@example.com'),
      screen.getByPlaceholderText(/describe your symptoms/i),
    ];

    for (const input of inputs) {
      // error border uses var(--danger); normal border uses var(--glass-border)
      expect((input as HTMLElement).style.border).not.toContain('var(--danger)');
    }
  });

  // ── 4. Shows errors on submit with empty fields ───────────────────────────

  it('shows errors on submit with empty fields', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);
    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    const inputs = [
      screen.getByPlaceholderText('Jane'),
      screen.getByPlaceholderText('Smith'),
      getDobInput(),
      screen.getByPlaceholderText('(555) 000-0000'),
      screen.getByPlaceholderText('jane@example.com'),
      screen.getByPlaceholderText(/describe your symptoms/i),
    ];

    for (const input of inputs) {
      expect((input as HTMLElement).style.border).toContain('var(--danger)');
    }
  });

  // ── 5. Does not show error for untouched fields ───────────────────────────

  it('does not show error for untouched fields', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    // Only blur the firstName input (leave the rest untouched)
    fireEvent.blur(screen.getByPlaceholderText('Jane'));

    // firstName should show error
    expect(screen.getByPlaceholderText('Jane').style.border).toContain('var(--danger)');

    // lastName was never touched — no error
    expect(screen.getByPlaceholderText('Smith').style.border).not.toContain('var(--danger)');
  });

  // ── 6. Blur on empty field shows error ────────────────────────────────────

  it('blur on empty field shows error', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);
    const firstNameInput = screen.getByPlaceholderText('Jane');

    expect(firstNameInput.style.border).not.toContain('var(--danger)');
    fireEvent.blur(firstNameInput);
    expect(firstNameInput.style.border).toContain('var(--danger)');
  });

  // ── 7. Blur on filled field does not show error ───────────────────────────

  it('blur on filled field does not show error', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);
    const firstNameInput = screen.getByPlaceholderText('Jane');

    fireEvent.change(firstNameInput, { target: { value: 'Alice' } });
    fireEvent.blur(firstNameInput);

    expect(firstNameInput.style.border).not.toContain('var(--danger)');
  });

  // ── 8. Does not submit with empty fields ─────────────────────────────────

  it('does not submit with empty fields', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);
    fireEvent.click(screen.getByRole('button', { name: /continue/i }));
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  // ── 9. Submits correct message format ────────────────────────────────────

  it('submits correct message format', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    fireEvent.change(screen.getByPlaceholderText('Jane'), { target: { value: 'John' } });
    fireEvent.change(screen.getByPlaceholderText('Smith'), { target: { value: 'Smith' } });
    fireEvent.change(getDobInput(), { target: { value: '1990-01-15' } });
    fireEvent.change(screen.getByPlaceholderText('(555) 000-0000'), { target: { value: '555-123-4567' } });
    fireEvent.change(screen.getByPlaceholderText('jane@example.com'), { target: { value: 'john@example.com' } });
    fireEvent.change(screen.getByPlaceholderText(/describe your symptoms/i), { target: { value: 'knee pain' } });

    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    expect(mockSubmit).toHaveBeenCalledTimes(1);
    expect(mockSubmit).toHaveBeenCalledWith(
      'My name is John Smith, date of birth 1990-01-15, phone +1555-123-4567, email john@example.com. Reason for visit: knee pain.',
    );
  });

  // ── 10. Does not reformat phone number ───────────────────────────────────

  it('does not reformat phone number', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    const rawPhone = '(800) 123-4567';
    fireEvent.change(screen.getByPlaceholderText('Jane'), { target: { value: 'Al' } });
    fireEvent.change(screen.getByPlaceholderText('Smith'), { target: { value: 'Bo' } });
    fireEvent.change(getDobInput(), { target: { value: '2000-06-01' } });
    fireEvent.change(screen.getByPlaceholderText('(555) 000-0000'), { target: { value: rawPhone } });
    fireEvent.change(screen.getByPlaceholderText('jane@example.com'), { target: { value: 'a@b.com' } });
    fireEvent.change(screen.getByPlaceholderText(/describe your symptoms/i), { target: { value: 'headache' } });

    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    expect(mockSubmit).toHaveBeenCalledWith(
      expect.stringContaining(`phone +1${rawPhone}`),
    );
  });

  // ── 11. Disables all inputs when disabled prop is true ────────────────────

  it('disables all inputs when disabled prop is true', () => {
    render(<IntakeForm onSubmit={mockSubmit} disabled={true} />);

    expect(screen.getByPlaceholderText('Jane')).toBeDisabled();
    expect(screen.getByPlaceholderText('Smith')).toBeDisabled();
    expect(getDobInput()).toBeDisabled();
    expect(screen.getByPlaceholderText('(555) 000-0000')).toBeDisabled();
    expect(screen.getByPlaceholderText('jane@example.com')).toBeDisabled();
    expect(screen.getByPlaceholderText(/describe your symptoms/i)).toBeDisabled();
    expect(screen.getByRole('button', { name: /continue/i })).toBeDisabled();
  });

  // ── 12. Trims whitespace before submission ────────────────────────────────

  it('trims whitespace before submission', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    // Fill firstName with only spaces — should be treated as empty
    fireEvent.change(screen.getByPlaceholderText('Jane'), { target: { value: '   ' } });
    fireEvent.change(screen.getByPlaceholderText('Smith'), { target: { value: 'Smith' } });
    fireEvent.change(getDobInput(), { target: { value: '1990-01-01' } });
    fireEvent.change(screen.getByPlaceholderText('(555) 000-0000'), { target: { value: '555-0000' } });
    fireEvent.change(screen.getByPlaceholderText('jane@example.com'), { target: { value: 'a@b.com' } });
    fireEvent.change(screen.getByPlaceholderText(/describe your symptoms/i), { target: { value: 'cough' } });

    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    // Validation should block submission
    expect(mockSubmit).not.toHaveBeenCalled();

    // firstName (whitespace-only) should now show error after submit attempt
    expect(screen.getByPlaceholderText('Jane').style.border).toContain('var(--danger)');
  });

  // ── 13. Typing in field after blur clears error ───────────────────────────

  it('typing in field after blur clears error', () => {
    render(<IntakeForm onSubmit={mockSubmit} />);

    const firstNameInput = screen.getByPlaceholderText('Jane');

    // Blur empty field to trigger error
    fireEvent.blur(firstNameInput);
    expect(firstNameInput.style.border).toContain('var(--danger)');

    // Type a valid value — field passes validation, error should be gone
    fireEvent.change(firstNameInput, { target: { value: 'Jo' } });
    expect(firstNameInput.style.border).not.toContain('var(--danger)');
  });
});
