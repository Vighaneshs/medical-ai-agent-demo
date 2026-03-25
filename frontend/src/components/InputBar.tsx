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
      className="px-4 py-4 border-t"
      style={{ borderColor: 'var(--glass-border)', background: 'rgba(18,23,35,0.6)' }}
    >
      <div
        className="glass-sm flex items-end gap-3 px-4 py-3"
        style={{ transition: 'border-color 0.2s' }}
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
          whileTap={{ scale: 0.92 }}
          className="flex-shrink-0 w-9 h-9 rounded-xl flex items-center justify-center transition-all"
          style={{
            background: disabled || !value.trim()
              ? 'rgba(87, 125, 232, 0.25)'
              : 'var(--primary)',
            cursor: disabled || !value.trim() ? 'not-allowed' : 'pointer',
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
