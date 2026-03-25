'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { initiateVoiceCall } from '@/lib/api';
import { SessionState } from '@/types';

interface Props {
  sessionId: string;
  sessionState: SessionState;
}

type CallState = 'idle' | 'connecting' | 'active' | 'ending';

export function VoiceCallButton({ sessionId, sessionState }: Props) {
  const [callState, setCallState] = useState<CallState>('idle');
  const [error, setError] = useState<string | null>(null);
  const vapiRef = useState<any>(null);

  const startCall = async () => {
    if (callState !== 'idle') return;
    setCallState('connecting');
    setError(null);

    try {
      const config = await initiateVoiceCall(sessionId);

      const { default: Vapi } = await import('@vapi-ai/web');
      const vapi = new Vapi(process.env.NEXT_PUBLIC_VAPI_PUBLIC_KEY || '');
      vapiRef[1](vapi);

      vapi.on('call-start', () => setCallState('active'));
      vapi.on('call-end', () => {
        setCallState('idle');
        vapiRef[1](null);
      });
      vapi.on('error', (err: any) => {
        console.error('Vapi error:', err);
        setError('Call failed. Please try again.');
        setCallState('idle');
      });

      await vapi.start(config.assistantId, config.assistantOverrides);
    } catch (err) {
      console.error('Voice call error:', err);
      setError('Could not start call. Please try again.');
      setCallState('idle');
    }
  };

  const endCall = () => {
    setCallState('ending');
    const vapi = vapiRef[0];
    if (vapi) {
      vapi.stop();
    } else {
      setCallState('idle');
    }
  };

  if (callState === 'active' || callState === 'ending') {
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
                {callState === 'ending' ? 'Ending call…' : 'Speaking with Kyron AI'}
              </div>
              <div className="text-sm" style={{ color: 'var(--text-muted)' }}>
                Your chat history is retained
              </div>
            </div>

            {/* Waveform */}
            {callState === 'active' && (
              <div className="flex items-center gap-1.5 h-10">
                {[0, 1, 2, 3, 4].map(i => (
                  <div key={i} className="waveform-bar" />
                ))}
              </div>
            )}

            <motion.button
              whileTap={{ scale: 0.95 }}
              onClick={endCall}
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

  return (
    <div className="flex flex-col items-end gap-1">
      <motion.button
        whileTap={{ scale: 0.93 }}
        onClick={startCall}
        disabled={callState === 'connecting'}
        title="Switch to voice call"
        className={`w-9 h-9 rounded-xl flex items-center justify-center ${
          callState === 'idle' ? 'voice-pulse' : ''
        }`}
        style={{
          background: callState === 'connecting'
            ? 'rgba(87, 125, 232, 0.3)'
            : 'var(--primary)',
          cursor: callState === 'connecting' ? 'not-allowed' : 'pointer',
          border: '1px solid rgba(255,255,255,0.15)',
        }}
      >
        {callState === 'connecting' ? (
          <motion.div
            animate={{ rotate: 360 }}
            transition={{ duration: 0.8, repeat: Infinity, ease: 'linear' }}
            className="w-4 h-4 border-2 border-white border-t-transparent rounded-full"
          />
        ) : (
          <svg width="16" height="16" viewBox="0 0 24 24" fill="white">
            <path d="M6.6 10.8c1.4 2.8 3.8 5.1 6.6 6.6l2.2-2.2c.3-.3.7-.4 1-.2 1.1.4 2.3.6 3.6.6.6 0 1 .4 1 1V20c0 .6-.4 1-1 1-9.4 0-17-7.6-17-17 0-.6.4-1 1-1h3.5c.6 0 1 .4 1 1 0 1.3.2 2.5.6 3.6.1.3 0 .7-.2 1L6.6 10.8z"/>
          </svg>
        )}
      </motion.button>
      {error && (
        <span className="text-xs" style={{ color: 'var(--danger)' }}>{error}</span>
      )}
    </div>
  );
}
