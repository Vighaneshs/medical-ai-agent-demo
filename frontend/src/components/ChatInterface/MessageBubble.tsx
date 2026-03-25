'use client';

import { motion } from 'framer-motion';
import { ChatMessage } from '@/types';

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function renderContent(content: string) {
  // Bold: **text**
  const parts = content.split(/(\*\*[^*]+\*\*)/g);
  return parts.map((part, i) => {
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={i}>{part.slice(2, -2)}</strong>;
    }
    // Handle line breaks
    return part.split('\n').map((line, j, arr) => (
      <span key={`${i}-${j}`}>
        {line}
        {j < arr.length - 1 && <br />}
      </span>
    ));
  });
}

export function MessageBubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === 'user';

  return (
    <motion.div
      initial={{ opacity: 0, y: 16, scale: 0.96 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: 0.28, ease: 'easeOut' }}
      className={`flex gap-3 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
    >
      {/* Avatar */}
      <div
        className="flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold mt-1"
        style={{
          background: isUser
            ? 'rgba(87, 125, 232, 0.3)'
            : 'linear-gradient(135deg, #577DE8, #48ACF0)',
          border: '1px solid rgba(255,255,255,0.15)',
          fontSize: '10px',
        }}
      >
        {isUser ? 'You' : 'K'}
      </div>

      {/* Bubble */}
      <div className={`max-w-[78%] ${isUser ? 'items-end' : 'items-start'} flex flex-col gap-1`}>
        <div
          className={`px-4 py-3 text-sm leading-relaxed ${
            message.isEmergency
              ? 'bubble-emergency'
              : isUser
              ? 'bubble-user'
              : 'bubble-ai'
          }`}
        >
          {message.isEmergency && (
            <div className="text-lg mb-1">⚠️</div>
          )}
          <div style={{ color: isUser ? '#fff' : 'rgba(255,255,255,0.92)' }}>
            {renderContent(message.content)}
          </div>
        </div>
        <span className="text-xs px-1" style={{ color: 'var(--text-muted)' }}>
          {formatTime(message.createdAt)}
        </span>
      </div>
    </motion.div>
  );
}
