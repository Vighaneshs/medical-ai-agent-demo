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
      className="w-full flex items-center px-4 sm:px-6 gap-2 sm:gap-6 border-b"
      style={{
          height: 'var(--nav-height)',
          background: 'rgba(2, 8, 23, 0.65)',
          backdropFilter: 'blur(24px) saturate(1.8)',
          WebkitBackdropFilter: 'blur(24px) saturate(1.8)',
          borderColor: 'var(--glass-border)',
          boxShadow: '0 4px 30px rgba(0, 0, 0, 0.3)',
          position: 'sticky',
          top: 0,
          zIndex: 50,
        }}
      >
        {/* Brand */}
        <Link href="/" className="flex items-center gap-2 mr-2 sm:mr-4 flex-shrink-0 group">
          <div
            className="w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold shadow-[0_0_15px_rgba(87,125,232,0.3)] transition-transform group-hover:scale-105"
            style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))', color: '#fff' }}
        >
          K
        </div>
        <span className="font-semibold text-sm hidden xs:inline-block sm:inline-block">Kyron Medical</span>
      </Link>

      {/* Nav links */}
      {!isLoginPage && (
        <div className="flex flex-1 items-center gap-3 sm:gap-6 overflow-x-auto no-scrollbar ml-2 sm:ml-0">
          {links.map(({ href, label }) => {
            const isActive = pathname === href;
            return (
                <Link
                  key={href}
                  href={href}
                  className="text-xs sm:text-sm font-medium transition-all hover:text-white whitespace-nowrap"
                  style={{
                    color: isActive ? 'var(--text)' : 'var(--text-muted)',
                    borderBottom: isActive ? '2px solid var(--primary)' : '2px solid transparent',
                    textShadow: isActive ? '0 0 10px rgba(87,125,232,0.3)' : 'none',
                    paddingBottom: '2px',
                  }}
                >
                {label}
              </Link>
            );
          })}
        </div>
      )}

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
