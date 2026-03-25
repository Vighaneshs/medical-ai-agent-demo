import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { NavBar } from '@/components/NavBar';
import { SESSION_KEY, USERNAME_KEY } from '@/lib/constants';

const mockPush = jest.fn();
const mockUsePathname = jest.fn();

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
  usePathname: () => mockUsePathname(),
}));

// Lightweight Link mock — renders as a plain anchor
jest.mock('next/link', () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...rest
  }: {
    href: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

describe('NavBar', () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
  });

  it('always renders the brand name', () => {
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);
    expect(screen.getByText('Kyron Medical')).toBeInTheDocument();
  });

  it('shows nav links on the chat page', async () => {
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);
    expect(screen.getByText('Chat')).toBeInTheDocument();
    expect(screen.getByText('Our Doctors')).toBeInTheDocument();
    expect(screen.getByText('My Appointment')).toBeInTheDocument();
    expect(screen.getByText('Office Info')).toBeInTheDocument();
  });

  it('hides nav links on the /login page', () => {
    mockUsePathname.mockReturnValue('/login');
    render(<NavBar />);
    expect(screen.queryByText('Chat')).not.toBeInTheDocument();
    expect(screen.queryByText('Our Doctors')).not.toBeInTheDocument();
  });

  it('shows the username pill when logged in', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);

    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });
  });

  it('shows the Sign out button when logged in', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument();
    });
  });

  it('does not show Sign out button when not logged in', async () => {
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);

    // Wait a tick to let useEffect run
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /sign out/i })).not.toBeInTheDocument();
    });
  });

  it('clears USERNAME_KEY and SESSION_KEY on sign out', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    localStorage.setItem(SESSION_KEY, 'user_alice');
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /sign out/i }));

    expect(localStorage.getItem(USERNAME_KEY)).toBeNull();
    expect(localStorage.getItem(SESSION_KEY)).toBeNull();
  });

  it('navigates to /login after sign out', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    mockUsePathname.mockReturnValue('/');
    render(<NavBar />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /sign out/i }));

    expect(mockPush).toHaveBeenCalledWith('/login');
  });

  it('does not show user section on /login page', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    mockUsePathname.mockReturnValue('/login');
    render(<NavBar />);

    // Wait for effects
    await new Promise(r => setTimeout(r, 0));

    expect(screen.queryByText('alice')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /sign out/i })).not.toBeInTheDocument();
  });
});
