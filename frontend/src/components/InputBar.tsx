'use client';

import { useState, useRef, KeyboardEvent } from 'react';
import { motion } from 'framer-motion';

interface Props {
  onSend: (text: string) => void;
  disabled: boolean;
}

export function InputBar({ onSend, disabled }: Props) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const submit = () => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setValue('');
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  const handleKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  const handleInput = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 120) + 'px';
  };

  return (
    <div
      className="px-6 py-4 border-t z-10"
      style={{ borderColor: 'var(--glass-border)', background: 'rgba(10, 20, 40, 0.5)', backdropFilter: 'blur(20px)' }}
    >
      <div
        className="glass flex items-end gap-3 px-4 py-3 shadow-[0_4px_20px_rgba(0,0,0,0.3)] transition-all"
        style={{ border: '1px solid var(--glass-border)' }}
      >
        <textarea
          ref={textareaRef}
          rows={1}
          value={value}
          onChange={e => { setValue(e.target.value); handleInput(); }}
          onKeyDown={handleKey}
          placeholder="Type a message…"
          disabled={disabled}
          className="flex-1 bg-transparent resize-none outline-none text-sm leading-relaxed"
          style={{
            color: 'var(--text)',
            caretColor: 'var(--accent)',
            maxHeight: '120px',
            // prevent iOS zoom
            fontSize: '16px',
          }}
        />

        <motion.button
          onClick={submit}
          disabled={disabled || !value.trim()}
          whileTap={{ scale: 0.90 }}
          whileHover={(!disabled && value.trim()) ? { scale: 1.05, boxShadow: '0 0 15px rgba(0,210,255,0.5)' } : {}}
          className="flex-shrink-0 w-10 h-10 rounded-xl flex items-center justify-center transition-all"
          style={{
            background: disabled || !value.trim()
              ? 'rgba(0, 210, 255, 0.15)'
              : 'linear-gradient(135deg, var(--primary), var(--primary-dark))',
            cursor: disabled || !value.trim() ? 'not-allowed' : 'pointer',
            border: '1px solid rgba(255,255,255,0.1)',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path
              d="M2 8L14 2L8 14L7 9L2 8Z"
              fill="white"
              fillOpacity={disabled || !value.trim() ? 0.4 : 1}
            />
          </svg>
        </motion.button>
      </div>
      <p className="text-center mt-2 text-xs" style={{ color: 'var(--text-muted)', opacity: 0.6 }}>
        Kyron Medical AI · Not a substitute for medical advice
      </p>
    </div>
  );
}
