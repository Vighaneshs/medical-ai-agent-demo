'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { v4 as uuidv4 } from 'uuid';
import { ChatMessage, Doctor, SessionState, TimeSlot } from '@/types';
import { sendMessage, getDoctors } from '@/lib/api';
import { SESSION_KEY } from '@/lib/constants';
import { MessageBubble } from './MessageBubble';
import { TypingIndicator } from './TypingIndicator';
import { InputBar } from '@/components/InputBar';
import { VoiceCallButton } from '@/components/VoiceCallButton';
import { StatusBadge } from '@/components/StatusBadge';
import { DoctorCard } from '@/components/DoctorCard';
import { AppointmentSlots } from '@/components/AppointmentSlots';
import { ConfirmationModal } from '@/components/ConfirmationModal';
import { IntakeForm } from '@/components/IntakeForm';

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
  const [sendError, setSendError] = useState(false);
  const [availableSlotDates, setAvailableSlotDates] = useState<string[]>([]);
  const [focusSlotDate, setFocusSlotDate] = useState('');
  const lastMessageRef = useRef<string>('');

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

    lastMessageRef.current = text;
    setSendError(false);
    setIsStreaming(true);
    setStreamingText('');

    let accumulated = '';
    let isEmergency = false;
    let failed = false;

    try {
      await sendMessage(sessionId.current, text, (chunk) => {
        if (chunk.newMessage) {
          if (accumulated.trim()) {
            // Backend started a new continuation turn — flush current text as its own bubble
            setMessages(prev => [...prev, {
              id: uuidv4(),
              role: 'assistant',
              content: accumulated.trim(),
              createdAt: new Date().toISOString(),
              isEmergency,
            }]);
          }
          accumulated = '';
          isEmergency = false;
          setStreamingText('');
          return;
        }
        if (chunk.text) {
          accumulated += chunk.text;
          setStreamingText(accumulated);
        }
        if (chunk.emergency) {
          isEmergency = true;
        }
        if (chunk.done) {
          console.debug('[chat] done', chunk);
          if (chunk.newState) setSessionState(chunk.newState as SessionState);
          if (chunk.doctorId) setMatchedDoctorId(chunk.doctorId);
          if (chunk.selectedSlot) setSelectedSlot(chunk.selectedSlot);
        }
      });
    } catch {
      failed = true;
      setSendError(true);
    }

    if (accumulated.trim()) {
      const aiMsg: ChatMessage = {
        id: uuidv4(),
        role: 'assistant',
        content: accumulated.trim(),
        createdAt: new Date().toISOString(),
        isEmergency,
      };
      setMessages(prev => [...prev, aiMsg]);
    } else if (failed && !isAutoGreeting) {
      // Remove the user message that got no response so retry is clean
      setMessages(prev => prev.filter(m => m.id !== userMsg.id));
    }

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
    const suffix = smsOptIn ? ', and yes please send an SMS reminder' : ', no SMS reminder needed';
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

  // Sync slot date tab when AI discusses a specific date during SCHEDULING
  useEffect(() => {
    if (sessionState !== 'SCHEDULING' || availableSlotDates.length === 0) return;
    const lastAI = [...messages].reverse().find(m => m.role === 'assistant');
    if (!lastAI) return;

    // 1. Direct ISO date match (AI often echoes the YYYY-MM-DD from its context)
    const isoMatch = lastAI.content.match(/\b(\d{4}-\d{2}-\d{2})\b/);
    if (isoMatch && availableSlotDates.includes(isoMatch[1])) {
      setFocusSlotDate(isoMatch[1]);
      return;
    }

    // 2. "Month Day" natural-language match (e.g. "March 31" or "Apr 1")
    for (const date of availableSlotDates) {
      const d = new Date(date + 'T00:00:00');
      const long = d.toLocaleDateString('en-US', { month: 'long', day: 'numeric' });   // "March 31"
      const short = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }); // "Mar 31"
      if (lastAI.content.includes(long) || lastAI.content.includes(short)) {
        setFocusSlotDate(date);
        return;
      }
    }
  }, [messages, sessionState, availableSlotDates]);

  const showQuickReplies =
    (sessionState === 'GREETING' || sessionState === 'BOOKED') &&
    !isStreaming &&
    messages.some(m => m.role === 'assistant');

  const quickReplies = sessionState === 'BOOKED'
    ? [
        { label: 'Book another appointment', message: 'I\'d like to book another appointment' },
        { label: 'Prescription refill', message: 'I need a prescription refill' },
        { label: 'Office hours & location', message: 'What are your office hours and location?' },
      ]
    : [
        { label: 'Schedule an appointment', message: 'I want to schedule an appointment with a specialist' },
        { label: 'Prescription refill', message: 'I need a prescription refill' },
        { label: 'Office hours & location', message: 'What are your office hours and location?' },
      ];

  // Show scheduling overlays only when not currently streaming a response
  const showSchedulingOverlay = sessionState === 'SCHEDULING' && matchedDoctorId && !isStreaming && !selectedSlot;
  const showConfirmingOverlay = sessionState === 'CONFIRMING' && matchedDoctor && selectedSlot && !isStreaming;
  const showMatchingButtons = sessionState === 'MATCHING' && matchedDoctorId && !isStreaming;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div
        className="flex flex-wrap items-center justify-between gap-y-3 px-3 sm:px-6 py-3 sm:py-4 border-b backdrop-blur-3xl z-10"
        style={{ borderColor: 'var(--glass-border)', background: 'var(--glass-bg)' }}
      >
        <div className="flex items-center gap-2 sm:gap-3">
          <div
            className="w-8 h-8 sm:w-10 sm:h-10 rounded-xl flex items-center justify-center text-sm font-bold shadow-[0_0_15px_rgba(87,125,232,0.3)] hidden xs:flex"
            style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))' }}
          >
            K
          </div>
          <div>
            <div className="font-semibold text-sm">Kyron Medical</div>
            <div className="text-xs" style={{ color: 'var(--text-muted)' }}>AI Care Coordinator</div>
          </div>
        </div>
        <div className="flex items-center gap-2 sm:gap-3 flex-wrap justify-end">
          <StatusBadge state={sessionState} />
          <VoiceCallButton sessionId={sessionId.current} sessionState={sessionState} />
        </div>
      </div>

      {/* Messages area */}
      <div className="flex-1 overflow-y-auto px-2 sm:px-4 py-4 sm:py-6 space-y-4">
        <AnimatePresence initial={false}>
          {messages.filter(msg => msg.content.trim()).map(msg => (
            <MessageBubble key={msg.id} message={msg} />
          ))}
          {isStreaming && streamingText.trim() ? (
            <MessageBubble
              key="streaming"
              message={{
                id: 'streaming',
                role: 'assistant',
                content: streamingText,
                createdAt: new Date().toISOString(),
              }}
            />
          ) : isStreaming ? (
            <TypingIndicator key="typing" />
          ) : null}
        </AnimatePresence>

        <AnimatePresence>
          {sendError && !isStreaming && (
            <motion.div
              key="send-error"
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              className="flex items-center gap-3 px-1"
            >
              <span className="text-xs" style={{ color: 'var(--danger)' }}>
                Something went wrong — no response received.
              </span>
              <button
                onClick={() => handleSend(lastMessageRef.current)}
                className="text-xs px-3 py-1.5 rounded-lg font-medium"
                style={{
                  background: 'rgba(87,125,232,0.12)',
                  border: '1px solid rgba(87,125,232,0.3)',
                  color: '#7BA4EF',
                  cursor: 'pointer',
                }}
              >
                Retry
              </button>
            </motion.div>
          )}
        </AnimatePresence>

        <AnimatePresence>
          {sessionState === 'INTAKE' && !isStreaming && (
            <motion.div
              key="intake-form"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 4 }}
              transition={{ duration: 0.2 }}
            >
              <IntakeForm onSubmit={handleSend} disabled={isStreaming} />
            </motion.div>
          )}
        </AnimatePresence>

        <AnimatePresence>
          {showMatchingButtons && (
            <motion.div
              key="matching-buttons"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 4 }}
              transition={{ duration: 0.2 }}
              className="flex gap-2 px-1"
            >
              <button
                onClick={() => handleSend('Yes, that doctor looks great')}
                className="px-5 py-2 rounded-xl text-sm font-semibold text-white transition-opacity"
                style={{ background: 'var(--success)', cursor: 'pointer' }}
              >
                Yes, book this doctor
              </button>
              <button
                onClick={() => handleSend('No, I would like a different doctor')}
                className="px-5 py-2 rounded-xl text-sm font-semibold transition-opacity"
                style={{
                  background: 'rgba(255,255,255,0.06)',
                  border: '1px solid var(--glass-border)',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                }}
              >
                No, different doctor
              </button>
            </motion.div>
          )}
        </AnimatePresence>

        <AnimatePresence>
          {showQuickReplies && (
            <motion.div
              key="quick-replies"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 8 }}
              transition={{ duration: 0.2 }}
              className="flex flex-wrap gap-2 px-1"
            >
              {quickReplies.map(({ label, message }) => (
                <button
                  key={label}
                  onClick={() => handleSend(message)}
                  className="px-3 py-1.5 sm:px-4 sm:py-2 rounded-full text-xs sm:text-sm font-medium border transition-all"
                  style={{
                    background: 'rgba(87, 125, 232, 0.1)',
                    borderColor: 'rgba(87, 125, 232, 0.3)',
                    color: 'var(--text)',
                    boxShadow: '0 4px 15px rgba(0,0,0,0.2)'
                  }}
                  onMouseEnter={e => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'rgba(87, 125, 232, 0.2)';
                    (e.currentTarget as HTMLButtonElement).style.borderColor = 'rgba(87, 125, 232, 0.6)';
                    (e.currentTarget as HTMLButtonElement).style.boxShadow = '0 0 15px rgba(87, 125, 232, 0.3)';
                  }}
                  onMouseLeave={e => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'rgba(87, 125, 232, 0.1)';
                    (e.currentTarget as HTMLButtonElement).style.borderColor = 'rgba(87, 125, 232, 0.3)';
                    (e.currentTarget as HTMLButtonElement).style.boxShadow = '0 4px 15px rgba(0,0,0,0.2)';
                  }}
                >
                  {label}
                </button>
              ))}
            </motion.div>
          )}
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
            focusDate={focusSlotDate}
            onDatesLoaded={setAvailableSlotDates}
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
            onCancel={() => handleSend('No, I want to pick a different time')}
          />
        )}
      </AnimatePresence>

      <InputBar onSend={handleSend} disabled={isStreaming} />
    </div>
  );
}
