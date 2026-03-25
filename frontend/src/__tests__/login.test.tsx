import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import LoginPage from '@/app/login/page';
import { SESSION_KEY, USERNAME_KEY, DEMO_PASSWORD } from '@/lib/constants';

const mockReplace = jest.fn();

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
}));

describe('LoginPage', () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
  });

  it('renders username input and submit button', () => {
    render(<LoginPage />);
    expect(screen.getByPlaceholderText(/e\.g\. Alice/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /start chatting/i })).toBeInTheDocument();
  });

  it('shows brand heading', () => {
    render(<LoginPage />);
    expect(screen.getByText('Kyron Medical')).toBeInTheDocument();
  });

  it('shows error when submitting with empty username', () => {
    render(<LoginPage />);
    fireEvent.click(screen.getByRole('button', { name: /start chatting/i }));
    expect(screen.getByText(/please enter a name/i)).toBeInTheDocument();
  });

  it('shows error when submitting whitespace-only username', () => {
    render(<LoginPage />);
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Alice/i), {
      target: { value: '   ' },
    });
    fireEvent.click(screen.getByRole('button', { name: /start chatting/i }));
    expect(screen.getByText(/please enter a name/i)).toBeInTheDocument();
  });

  it('sets USERNAME_KEY and SESSION_KEY in localStorage on valid submit', () => {
    render(<LoginPage />);
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Alice/i), {
      target: { value: 'Alice' },
    });
    fireEvent.change(screen.getByPlaceholderText(/enter demo password/i), {
      target: { value: DEMO_PASSWORD },
    });
    fireEvent.click(screen.getByRole('button', { name: /start chatting/i }));

    expect(localStorage.getItem(USERNAME_KEY)).toBe('Alice');
    expect(localStorage.getItem(SESSION_KEY)).toBe('user_alice');
  });

  it('redirects to / after successful login', () => {
    render(<LoginPage />);
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Alice/i), {
      target: { value: 'Bob' },
    });
    fireEvent.change(screen.getByPlaceholderText(/enter demo password/i), {
      target: { value: DEMO_PASSWORD },
    });
    fireEvent.click(screen.getByRole('button', { name: /start chatting/i }));

    expect(mockReplace).toHaveBeenCalledWith('/');
  });

  it('derives session id as lower-cased username', () => {
    render(<LoginPage />);
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Alice/i), {
      target: { value: 'UPPER' },
    });
    fireEvent.change(screen.getByPlaceholderText(/enter demo password/i), {
      target: { value: DEMO_PASSWORD },
    });
    fireEvent.click(screen.getByRole('button', { name: /start chatting/i }));

    expect(localStorage.getItem(SESSION_KEY)).toBe('user_upper');
  });

  it('redirects to / immediately if already logged in', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    render(<LoginPage />);
    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/');
    });
  });

  it('clears error when user types after an error', () => {
    render(<LoginPage />);
    // Trigger error
    fireEvent.click(screen.getByRole('button', { name: /start chatting/i }));
    expect(screen.getByText(/please enter a name/i)).toBeInTheDocument();

    // Start typing — error should clear
    fireEvent.change(screen.getByPlaceholderText(/e\.g\. Alice/i), {
      target: { value: 'a' },
    });
    expect(screen.queryByText(/please enter a name/i)).not.toBeInTheDocument();
  });
});
