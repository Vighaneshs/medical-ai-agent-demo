'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { SESSION_KEY, USERNAME_KEY } from '@/lib/constants';

const links = [
  { href: '/',            label: 'Chat' },
  { href: '/doctors',     label: 'Our Doctors' },
  { href: '/appointment', label: 'My Appointment' },
  { href: '/office',      label: 'Office Info' },
];

export function NavBar() {
  const pathname = usePathname();
  const router = useRouter();
  const [username, setUsername] = useState<string | null>(null);

  useEffect(() => {
    setUsername(localStorage.getItem(USERNAME_KEY));
  }, [pathname]); // re-read on route change (handles login/logout)

  function handleSignOut() {
    localStorage.removeItem(USERNAME_KEY);
    localStorage.removeItem(SESSION_KEY);
    router.push('/login');
  }

  const isLoginPage = pathname === '/login';

  return (
    <nav
      className="w-full flex items-center px-6 gap-6 border-b"
      style={{
        height: 'var(--nav-height)',
        background: 'rgba(18, 23, 35, 0.85)',
        backdropFilter: 'blur(16px)',
        WebkitBackdropFilter: 'blur(16px)',
        borderColor: 'var(--glass-border)',
        position: 'sticky',
        top: 0,
        zIndex: 50,
      }}
    >
      {/* Brand */}
      <Link href="/" className="flex items-center gap-2 mr-4 flex-shrink-0">
        <div
          className="w-7 h-7 rounded-lg flex items-center justify-center text-xs font-bold"
          style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)' }}
        >
          K
        </div>
        <span className="font-semibold text-sm">Kyron Medical</span>
      </Link>

      {/* Nav links — hidden on login page */}
      {!isLoginPage && links.map(({ href, label }) => {
        const isActive = pathname === href;
        return (
          <Link
            key={href}
            href={href}
            className="text-sm font-medium transition-colors"
            style={{
              color: isActive ? '#7BA4EF' : 'var(--text-muted)',
              borderBottom: isActive ? '2px solid #577DE8' : '2px solid transparent',
              paddingBottom: '2px',
            }}
          >
            {label}
          </Link>
        );
      })}

      {/* Spacer */}
      <div className="flex-1" />

      {/* User identity */}
      {!isLoginPage && username && (
        <div className="flex items-center gap-3 flex-shrink-0">
          <span
            className="text-xs font-medium px-3 py-1 rounded-full border"
            style={{
              background: 'rgba(87,125,232,0.12)',
              borderColor: 'rgba(87,125,232,0.3)',
              color: '#7BA4EF',
            }}
          >
            {username}
          </span>
          <button
            onClick={handleSignOut}
            className="text-xs font-medium transition-colors"
            style={{ color: 'var(--text-muted)' }}
            onMouseEnter={e => (e.currentTarget.style.color = '#fff')}
            onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
          >
            Sign out
          </button>
        </div>
      )}
    </nav>
  );
}
