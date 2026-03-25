'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { AnimatePresence } from 'framer-motion';
import { v4 as uuidv4 } from 'uuid';
import { ChatMessage, Doctor, SessionState, TimeSlot } from '@/types';
import { sendMessage, getDoctors } from '@/lib/api';
import { MessageBubble } from './MessageBubble';
import { TypingIndicator } from './TypingIndicator';
import { InputBar } from '@/components/InputBar';
import { VoiceCallButton } from '@/components/VoiceCallButton';
import { StatusBadge } from '@/components/StatusBadge';
import { DoctorCard } from '@/components/DoctorCard';
import { AppointmentSlots } from '@/components/AppointmentSlots';
import { ConfirmationModal } from '@/components/ConfirmationModal';

const SESSION_KEY = 'kyron_session_id';

function getOrCreateSessionId(): string {
  if (typeof window === 'undefined') return uuidv4();
  let id = localStorage.getItem(SESSION_KEY);
  if (!id) {
    id = uuidv4();
    localStorage.setItem(SESSION_KEY, id);
  }
  return id;
}

export function ChatInterface() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [sessionState, setSessionState] = useState<SessionState>('GREETING');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingText, setStreamingText] = useState('');

  const [matchedDoctorId, setMatchedDoctorId] = useState<string | null>(null);
  const [selectedSlot, setSelectedSlot] = useState<TimeSlot | null>(null);
  const [doctorsMap, setDoctorsMap] = useState<Record<string, Doctor>>({});
  const [patientFirstName, setPatientFirstName] = useState('');

  const sessionId = useRef(getOrCreateSessionId());
  const bottomRef = useRef<HTMLDivElement>(null);
  const hasGreeted = useRef(false);

  const scrollToBottom = useCallback(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => { scrollToBottom(); }, [messages, streamingText, scrollToBottom]);

  // Pre-load doctor data for overlays
  useEffect(() => {
    getDoctors().then((docs: Doctor[]) => {
      const map: Record<string, Doctor> = {};
      docs.forEach((d: Doctor) => { map[d.id] = d; });
      setDoctorsMap(map);
    }).catch(() => {});
  }, []);

  // Auto-send greeting on mount
  useEffect(() => {
    if (hasGreeted.current) return;
    hasGreeted.current = true;
    handleSend('Hello');
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSend = useCallback(async (text: string) => {
    if (isStreaming || !text.trim()) return;

    const userMsg: ChatMessage = {
      id: uuidv4(),
      role: 'user',
      content: text.trim(),
      createdAt: new Date().toISOString(),
    };

    const isAutoGreeting = text === 'Hello' && messages.length === 0;
    if (!isAutoGreeting) {
      setMessages(prev => [...prev, userMsg]);
    }

    setIsStreaming(true);
    setStreamingText('');

    let accumulated = '';
    let isEmergency = false;

    try {
      await sendMessage(sessionId.current, text, (chunk) => {
        if (chunk.text) {
          accumulated += chunk.text;
          setStreamingText(accumulated);
        }
        if (chunk.emergency) {
          isEmergency = true;
        }
        if (chunk.done) {
          if (chunk.newState) setSessionState(chunk.newState as SessionState);
          if (chunk.doctorId) setMatchedDoctorId(chunk.doctorId);
          if (chunk.selectedSlot) setSelectedSlot(chunk.selectedSlot);
        }
      });
    } catch {
      accumulated = "I'm having trouble connecting right now. Please try again.";
    }

    const aiMsg: ChatMessage = {
      id: uuidv4(),
      role: 'assistant',
      content: accumulated,
      createdAt: new Date().toISOString(),
      isEmergency,
    };

    setMessages(prev => [...prev, aiMsg]);
    setStreamingText('');
    setIsStreaming(false);
  }, [isStreaming, messages.length]);

  const matchedDoctor = matchedDoctorId ? doctorsMap[matchedDoctorId] : null;

  const handleSlotSelect = useCallback((slot: TimeSlot) => {
    const d = new Date(slot.date + 'T00:00:00');
    const dateStr = d.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });
    const [h, m] = slot.startTime.split(':').map(Number);
    const ampm = h >= 12 ? 'PM' : 'AM';
    const h12 = h % 12 || 12;
    handleSend(`I'll take ${dateStr} at ${h12}:${m.toString().padStart(2, '0')} ${ampm}`);
  }, [handleSend]);

  const handleConfirmBooking = useCallback((smsOptIn: boolean) => {
    const suffix = smsOptIn ? ', and yes to the SMS reminder' : '';
    handleSend(`Yes, please confirm my booking${suffix}`);
  }, [handleSend]);

  // Extract patient first name for confirmation modal display
  useEffect(() => {
    if (!patientFirstName && messages.length > 0) {
      const combined = messages.map(m => m.content).join(' ');
      const nameMatch = combined.match(/(?:I'm|I am|my name is|this is)\s+([A-Z][a-z]+)/i);
      if (nameMatch) setPatientFirstName(nameMatch[1]);
    }
  }, [messages, patientFirstName]);

  // Show scheduling overlays only when not currently streaming a response
  const showSchedulingOverlay = sessionState === 'SCHEDULING' && matchedDoctorId && !isStreaming;
  const showConfirmingOverlay = sessionState === 'CONFIRMING' && matchedDoctor && selectedSlot && !isStreaming;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div
        className="flex items-center justify-between px-6 py-4 border-b"
        style={{ borderColor: 'var(--glass-border)' }}
      >
        <div className="flex items-center gap-3">
          <div
            className="w-9 h-9 rounded-xl flex items-center justify-center text-sm font-bold"
            style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)' }}
          >
            K
          </div>
          <div>
            <div className="font-semibold text-sm">Kyron Medical</div>
            <div className="text-xs" style={{ color: 'var(--text-muted)' }}>AI Care Coordinator</div>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <StatusBadge state={sessionState} />
          <VoiceCallButton sessionId={sessionId.current} sessionState={sessionState} />
        </div>
      </div>

      {/* Messages area */}
      <div className="flex-1 overflow-y-auto px-4 py-6 space-y-4">
        <AnimatePresence initial={false}>
          {messages.map(msg => (
            <MessageBubble key={msg.id} message={msg} />
          ))}
        </AnimatePresence>

        {isStreaming && streamingText && (
          <MessageBubble
            message={{
              id: 'streaming',
              role: 'assistant',
              content: streamingText,
              createdAt: new Date().toISOString(),
            }}
          />
        )}

        <AnimatePresence>
          {isStreaming && !streamingText && <TypingIndicator />}
        </AnimatePresence>

        <div ref={bottomRef} />
      </div>

      {/* Scheduling overlays — appear between messages and input */}
      <AnimatePresence>
        {showSchedulingOverlay && matchedDoctor && (
          <DoctorCard key="doctor-card" doctor={matchedDoctor} />
        )}
      </AnimatePresence>

      <AnimatePresence>
        {showSchedulingOverlay && matchedDoctorId && (
          <AppointmentSlots
            key={`slots-${matchedDoctorId}`}
            doctorId={matchedDoctorId}
            onSelect={handleSlotSelect}
          />
        )}
      </AnimatePresence>

      <AnimatePresence>
        {showConfirmingOverlay && matchedDoctor && selectedSlot && (
          <ConfirmationModal
            key="confirm-modal"
            doctor={matchedDoctor}
            slot={selectedSlot}
            patientName={patientFirstName || 'Patient'}
            onConfirm={handleConfirmBooking}
          />
        )}
      </AnimatePresence>

      <InputBar onSend={handleSend} disabled={isStreaming} />
    </div>
  );
}
