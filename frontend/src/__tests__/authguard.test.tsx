import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthGuard } from '@/components/AuthGuard';
import { USERNAME_KEY } from '@/lib/constants';

const mockReplace = jest.fn();
const mockUsePathname = jest.fn();

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: mockReplace }),
  usePathname: () => mockUsePathname(),
}));

describe('AuthGuard', () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
  });

  it('renders children when user is authenticated', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    mockUsePathname.mockReturnValue('/');

    render(
      <AuthGuard>
        <div>Protected Content</div>
      </AuthGuard>,
    );

    await waitFor(() => {
      expect(screen.getByText('Protected Content')).toBeInTheDocument();
    });
  });

  it('redirects to /login when unauthenticated on a protected route', async () => {
    mockUsePathname.mockReturnValue('/');

    render(
      <AuthGuard>
        <div>Protected Content</div>
      </AuthGuard>,
    );

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith('/login');
    });
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });

  it('renders children on /login without redirecting', async () => {
    mockUsePathname.mockReturnValue('/login');

    render(
      <AuthGuard>
        <div>Login Page</div>
      </AuthGuard>,
    );

    await waitFor(() => {
      expect(screen.getByText('Login Page')).toBeInTheDocument();
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('does not redirect on /login even if unauthenticated', async () => {
    // no USERNAME_KEY in localStorage
    mockUsePathname.mockReturnValue('/login');

    render(
      <AuthGuard>
        <div>Login Page</div>
      </AuthGuard>,
    );

    await waitFor(() => {
      expect(screen.getByText('Login Page')).toBeInTheDocument();
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('renders children when authenticated on any protected route', async () => {
    localStorage.setItem(USERNAME_KEY, 'alice');
    mockUsePathname.mockReturnValue('/doctors');

    render(
      <AuthGuard>
        <div>Doctors Page</div>
      </AuthGuard>,
    );

    await waitFor(() => {
      expect(screen.getByText('Doctors Page')).toBeInTheDocument();
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('protects all non-login routes when unauthenticated', async () => {
    const routes = ['/doctors', '/appointment', '/office', '/about'];

    for (const route of routes) {
      localStorage.clear();
      mockReplace.mockClear();
      mockUsePathname.mockReturnValue(route);

      const { unmount } = render(
        <AuthGuard>
          <div>Content</div>
        </AuthGuard>,
      );

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
      unmount();
    }
  });
});
