'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { initiateVoiceCall, initiatePhoneCall } from '@/lib/api';
import { SessionState } from '@/types';

interface Props {
  sessionId: string;
  sessionState: SessionState;
}

type BrowserCallState = 'idle' | 'connecting' | 'active' | 'ending';

export function VoiceCallButton({ sessionId }: Props) {
  const [browserState, setBrowserState] = useState<BrowserCallState>('idle');
  const [showPhoneInput, setShowPhoneInput] = useState(false);
  const [phone, setPhone] = useState('');
  const [countryCode, setCountryCode] = useState('+1');
  const [phoneCalling, setPhoneCalling] = useState(false);
  const [phoneSuccess, setPhoneSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const vapiRef = useState<any>(null);

  // ── Browser call ────────────────────────────────────────────────────────────

  const startBrowserCall = async () => {
    if (browserState !== 'idle') return;
    setBrowserState('connecting');
    setError(null);
    try {
      const config = await initiateVoiceCall(sessionId);
      const { default: Vapi } = await import('@vapi-ai/web');
      const vapi = new Vapi(process.env.NEXT_PUBLIC_VAPI_PUBLIC_KEY || '');
      vapiRef[1](vapi);
      vapi.on('call-start', () => setBrowserState('active'));
      vapi.on('call-end', () => { setBrowserState('idle'); vapiRef[1](null); });
      vapi.on('error', (err: any) => {
        console.error('Vapi error:', err);
        setError('Call failed. Please try again.');
        setBrowserState('idle');
      });
      await vapi.start(config.assistantId, config.assistantOverrides);
    } catch {
      setError('Could not start call. Please try again.');
      setBrowserState('idle');
    }
  };

  const endBrowserCall = () => {
    setBrowserState('ending');
    const vapi = vapiRef[0];
    if (vapi) vapi.stop();
    else setBrowserState('idle');
  };

  // ── Phone call ──────────────────────────────────────────────────────────────

  const startPhoneCall = async () => {
    if (!phone.trim()) return;
    setPhoneCalling(true);
    setError(null);
    try {
      await initiatePhoneCall(sessionId, `${countryCode}${phone.trim()}`);
      setPhoneCalling(false);
      setPhoneSuccess(true);
      setShowPhoneInput(false);
      setTimeout(() => setPhoneSuccess(false), 4000);
    } catch {
      setError('Could not place call. Please try again.');
      setPhoneCalling(false);
    }
  };

  // ── Active browser call overlay ─────────────────────────────────────────────

  if (browserState === 'active' || browserState === 'ending') {
    return (
      <AnimatePresence>
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.95 }}
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ background: 'rgba(18, 23, 35, 0.85)', backdropFilter: 'blur(8px)' }}
        >
          <div className="glass p-8 flex flex-col items-center gap-6 mx-4" style={{ maxWidth: 340, width: '100%' }}>
            <div
              className="w-16 h-16 rounded-full flex items-center justify-center"
              style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)' }}
            >
              <svg width="24" height="24" viewBox="0 0 24 24" fill="white">
                <path d="M6.6 10.8c1.4 2.8 3.8 5.1 6.6 6.6l2.2-2.2c.3-.3.7-.4 1-.2 1.1.4 2.3.6 3.6.6.6 0 1 .4 1 1V20c0 .6-.4 1-1 1-9.4 0-17-7.6-17-17 0-.6.4-1 1-1h3.5c.6 0 1 .4 1 1 0 1.3.2 2.5.6 3.6.1.3 0 .7-.2 1L6.6 10.8z"/>
              </svg>
            </div>
            <div className="text-center">
              <div className="font-semibold mb-1">
                {browserState === 'ending' ? 'Ending call…' : 'Speaking with Kyron AI'}
              </div>
              <div className="text-sm" style={{ color: 'var(--text-muted)' }}>Your chat history is retained</div>
            </div>
            {browserState === 'active' && (
              <div className="flex items-center gap-1.5 h-10">
                {[0, 1, 2, 3, 4].map(i => (
                  <div key={i} className="waveform-bar" />
                ))}
              </div>
            )}
            <motion.button
              whileTap={{ scale: 0.95 }}
              onClick={endBrowserCall}
              className="w-full py-3 rounded-xl font-medium text-sm text-white"
              style={{ background: '#E85757', cursor: 'pointer' }}
            >
              End Call
            </motion.button>
          </div>
        </motion.div>
      </AnimatePresence>
    );
  }

  // ── Idle — show both options ─────────────────────────────────────────────────

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center gap-2">
        {/* In-browser call */}
        <motion.button
          whileTap={{ scale: 0.93 }}
          onClick={startBrowserCall}
          disabled={browserState === 'connecting'}
          title="Call in browser"
          className="flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium transition-opacity"
          style={{
            background: browserState === 'connecting' ? 'rgba(87,125,232,0.3)' : 'var(--primary)',
            color: '#fff',
            border: '1px solid rgba(255,255,255,0.15)',
            opacity: browserState === 'connecting' ? 0.7 : 1,
            cursor: browserState === 'connecting' ? 'not-allowed' : 'pointer',
          }}
        >
          {browserState === 'connecting' ? (
            <motion.div
              animate={{ rotate: 360 }}
              transition={{ duration: 0.8, repeat: Infinity, ease: 'linear' }}
              className="w-3 h-3 border-2 border-white border-t-transparent rounded-full"
            />
          ) : (
            <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 15c1.66 0 3-1.34 3-3V6c0-1.66-1.34-3-3-3S9 4.34 9 6v6c0 1.66 1.34 3 3 3zm-1-9c0-.55.45-1 1-1s1 .45 1 1v6c0 .55-.45 1-1 1s-1-.45-1-1V6zm6 6c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-2.08c3.39-.49 6-3.39 6-6.92h-2z"/>
            </svg>
          )}
          {browserState === 'connecting' ? 'Connecting…' : 'Call in browser'}
        </motion.button>

        {/* Call my phone */}
        <motion.button
          whileTap={{ scale: 0.93 }}
          onClick={() => { setShowPhoneInput(v => !v); setError(null); }}
          title="Call my phone"
          className="flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium"
          style={{
            background: showPhoneInput ? 'rgba(87,125,232,0.2)' : 'rgba(87,125,232,0.08)',
            color: '#7BA4EF',
            border: '1px solid rgba(87,125,232,0.35)',
            cursor: 'pointer',
          }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
            <path d="M6.6 10.8c1.4 2.8 3.8 5.1 6.6 6.6l2.2-2.2c.3-.3.7-.4 1-.2 1.1.4 2.3.6 3.6.6.6 0 1 .4 1 1V20c0 .6-.4 1-1 1-9.4 0-17-7.6-17-17 0-.6.4-1 1-1h3.5c.6 0 1 .4 1 1 0 1.3.2 2.5.6 3.6.1.3 0 .7-.2 1L6.6 10.8z"/>
          </svg>
          Call my phone
        </motion.button>
      </div>

      {/* Phone number input */}
      <AnimatePresence>
        {showPhoneInput && (
          <motion.div
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.15 }}
            className="flex gap-2"
          >
            <select
              value={countryCode}
              onChange={e => setCountryCode(e.target.value)}
              className="h-8 rounded-lg px-1 text-xs"
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid var(--glass-border)',
                color: 'var(--text)',
                outline: 'none',
                cursor: 'pointer',
              }}
            >
              {['+1','+44','+91','+61','+49','+33','+52','+55','+81','+86'].map(c => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
            <input
              type="tel"
              placeholder="555 000-0000"
              value={phone}
              onChange={e => setPhone(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && startPhoneCall()}
              autoFocus
              className="h-8 rounded-lg px-3 text-xs"
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid var(--glass-border)',
                color: 'var(--text)',
                outline: 'none',
                width: 130,
              }}
            />
            <motion.button
              whileTap={{ scale: 0.95 }}
              onClick={startPhoneCall}
              disabled={phoneCalling || !phone.trim()}
              className="h-8 px-3 rounded-lg text-xs font-medium text-white"
              style={{
                background: 'linear-gradient(135deg, #577DE8, #48ACF0)',
                opacity: phoneCalling || !phone.trim() ? 0.5 : 1,
                cursor: phoneCalling || !phone.trim() ? 'not-allowed' : 'pointer',
              }}
            >
              {phoneCalling ? 'Calling…' : 'Call me'}
            </motion.button>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Success / error feedback */}
      <AnimatePresence>
        {phoneSuccess && (
          <motion.span
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="text-xs"
            style={{ color: '#48ACF0' }}
          >
            Calling your phone now…
          </motion.span>
        )}
      </AnimatePresence>
      {error && <span className="text-xs" style={{ color: 'var(--danger)' }}>{error}</span>}
    </div>
  );
}
