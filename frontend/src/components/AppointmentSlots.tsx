'use client';

import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { getAvailability } from '@/lib/api';
import { TimeSlot } from '@/types';

interface Props {
  doctorId: string;
  onSelect: (slot: TimeSlot) => void;
  focusDate?: string;
  onDatesLoaded?: (dates: string[]) => void;
}

function groupByDate(slots: TimeSlot[]): Record<string, TimeSlot[]> {
  return slots.reduce<Record<string, TimeSlot[]>>((acc, slot) => {
    if (!acc[slot.date]) acc[slot.date] = [];
    acc[slot.date].push(slot);
    return acc;
  }, {});
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00');
  return d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
}

function formatTime(t: string): string {
  const [h, m] = t.split(':').map(Number);
  const ampm = h >= 12 ? 'PM' : 'AM';
  const h12 = h % 12 || 12;
  return `${h12}:${m.toString().padStart(2, '0')} ${ampm}`;
}

export function AppointmentSlots({ doctorId, onSelect, focusDate, onDatesLoaded }: Props) {
  const [slots, setSlots] = useState<TimeSlot[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeDate, setActiveDate] = useState<string>('');

  useEffect(() => {
    getAvailability(doctorId)
      .then((data: TimeSlot[]) => {
        const available = data.filter(s => s.available);
        setSlots(available);
        if (available.length > 0) setActiveDate(available[0].date);
        onDatesLoaded?.(available.map(s => s.date));
      })
      .finally(() => setLoading(false));
  }, [doctorId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Sync the active date tab when the parent identifies a date being discussed
  useEffect(() => {
    if (focusDate && slots.some(s => s.date === focusDate)) {
      setActiveDate(focusDate);
    }
  }, [focusDate, slots]);

  const grouped = groupByDate(slots);
  const dates = Object.keys(grouped).slice(0, 10);

  if (loading) {
    return (
      <div className="glass mx-4 mb-4 p-4" style={{ borderRadius: 14 }}>
        <div className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading available times…</div>
      </div>
    );
  }

  if (slots.length === 0) {
    return (
      <div className="glass mx-4 mb-4 p-4" style={{ borderRadius: 14 }}>
        <div className="text-xs" style={{ color: 'var(--text-muted)' }}>No availability found. Please try another provider.</div>
      </div>
    );
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 12, scale: 0.97 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 8, scale: 0.97 }}
      transition={{ duration: 0.25, ease: 'easeOut' }}
      className="glass mx-4 mb-4 p-4"
      style={{ borderRadius: 14 }}
    >
      <div className="text-xs font-medium mb-3" style={{ color: 'var(--accent)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
        Available Times
      </div>

      {/* Date tabs */}
      <div className="flex gap-2 overflow-x-auto pb-2 mb-3" style={{ scrollbarWidth: 'none' }}>
        {dates.map(date => (
          <button
            key={date}
            onClick={() => setActiveDate(date)}
            className="flex-shrink-0 px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
            style={{
              background: activeDate === date ? 'var(--primary)' : 'rgba(87,125,232,0.12)',
              color: activeDate === date ? '#fff' : 'var(--text-muted)',
              border: activeDate === date ? '1px solid transparent' : '1px solid rgba(87,125,232,0.2)',
              cursor: 'pointer',
            }}
          >
            {formatDate(date)}
          </button>
        ))}
      </div>

      {/* Time grid */}
      <AnimatePresence mode="wait">
        {activeDate && (
          <motion.div
            key={activeDate}
            initial={{ opacity: 0, x: 8 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -8 }}
            transition={{ duration: 0.15 }}
            className="grid grid-cols-3 gap-2"
          >
            {(grouped[activeDate] ?? []).map(slot => (
              <motion.button
                key={`${slot.date}-${slot.startTime}`}
                whileTap={{ scale: 0.94 }}
                onClick={() => onSelect(slot)}
                className="py-2 px-1 rounded-xl text-xs font-medium text-center transition-all"
                style={{
                  background: 'rgba(87,125,232,0.12)',
                  border: '1px solid rgba(87,125,232,0.2)',
                  color: 'var(--text)',
                  cursor: 'pointer',
                }}
              >
                {formatTime(slot.startTime)}
              </motion.button>
            ))}
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}
