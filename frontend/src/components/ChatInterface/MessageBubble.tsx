'use client';

import ReactMarkdown from 'react-markdown';
import { motion } from 'framer-motion';
import { ChatMessage } from '@/types';

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
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
          <div
            className="prose-chat"
            style={{ color: isUser ? '#fff' : 'rgba(255,255,255,0.92)' }}
          >
            {isUser ? (
              message.content
            ) : (
              <ReactMarkdown
                components={{
                  p: ({ children }) => <p className="mb-1 last:mb-0">{children}</p>,
                  strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
                  em: ({ children }) => <em className="italic">{children}</em>,
                  ul: ({ children }) => <ul className="list-disc pl-4 my-1 space-y-0.5">{children}</ul>,
                  ol: ({ children }) => <ol className="list-decimal pl-4 my-1 space-y-0.5">{children}</ol>,
                  li: ({ children }) => <li>{children}</li>,
                  h1: ({ children }) => <p className="font-semibold mb-1">{children}</p>,
                  h2: ({ children }) => <p className="font-semibold mb-1">{children}</p>,
                  h3: ({ children }) => <p className="font-medium mb-0.5">{children}</p>,
                  hr: () => <hr className="border-white/20 my-2" />,
                  code: ({ children }) => (
                    <code className="bg-white/10 px-1 rounded text-xs font-mono">{children}</code>
                  ),
                }}
              >
                {message.content}
              </ReactMarkdown>
            )}
          </div>
        </div>
        <span className="text-xs px-1" style={{ color: 'var(--text-muted)' }}>
          {formatTime(message.createdAt)}
        </span>
      </div>
    </motion.div>
  );
}
