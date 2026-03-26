'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { SESSION_KEY, USERNAME_KEY, DEMO_PASSWORD } from '@/lib/constants';

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  // Already logged in → skip to chat
  useEffect(() => {
    if (localStorage.getItem(USERNAME_KEY)) {
      router.replace('/');
    }
  }, [router]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = username.trim();
    if (!trimmed) {
      setError('Please enter a name to continue.');
      return;
    }
    if (password !== DEMO_PASSWORD) {
      setError('Incorrect password.');
      return;
    }
    const sessionId = 'user_' + trimmed.toLowerCase();
    localStorage.setItem(USERNAME_KEY, trimmed);
    localStorage.setItem(SESSION_KEY, sessionId);
    router.replace('/');
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="glass w-full max-w-sm p-8 flex flex-col gap-6">
        {/* Logo */}
        <div className="flex flex-col items-center gap-3">
          <div
            className="w-14 h-14 rounded-2xl flex items-center justify-center text-2xl font-bold"
            style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)' }}
          >
            K
          </div>
          <div className="text-center">
            <div className="font-semibold text-lg">Kyron Medical</div>
            <div className="text-sm mt-0.5" style={{ color: 'var(--text-muted)' }}>
              AI Care Coordinator
            </div>
          </div>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              Your name (any username)
            </label>
            <input
              type="text"
              autoFocus
              placeholder="e.g. Alice"
              value={username}
              onChange={e => { setUsername(e.target.value); setError(''); }}
              className="w-full px-4 py-2.5 rounded-xl text-sm outline-none"
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid var(--glass-border)',
                color: 'var(--text)',
              }}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              Access password (It is - 1234)
            </label>
            <input
              type="password"
              placeholder="Enter demo password"
              value={password}
              onChange={e => { setPassword(e.target.value); setError(''); }}
              className="w-full px-4 py-2.5 rounded-xl text-sm outline-none"
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: `1px solid ${error ? 'var(--danger)' : 'var(--glass-border)'}`,
                color: 'var(--text)',
              }}
            />
            {error && (
              <p className="text-xs" style={{ color: 'var(--danger)' }}>{error}</p>
            )}
          </div>

          <button
            type="submit"
            className="w-full py-2.5 rounded-xl text-sm font-semibold transition-opacity hover:opacity-90"
            style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)', color: '#fff' }}
          >
            Start chatting →
          </button>
        </form>

        <p className="text-center text-xs" style={{ color: 'var(--text-muted)' }}>
          Access restricted to authorised demo users.
        </p>
      </div>
    </div>
  );
}
