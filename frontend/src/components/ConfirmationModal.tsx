'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { Doctor, TimeSlot } from '@/types';

interface Props {
  doctor: Doctor;
  slot: TimeSlot;
  patientName: string;
  onConfirm: (smsOptIn: boolean) => void;
}

function formatSlot(slot: TimeSlot): string {
  const d = new Date(slot.date + 'T00:00:00');
  const dateStr = d.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });
  const [h, m] = slot.startTime.split(':').map(Number);
  const ampm = h >= 12 ? 'PM' : 'AM';
  const h12 = h % 12 || 12;
  return `${dateStr} at ${h12}:${m.toString().padStart(2, '0')} ${ampm}`;
}

export function ConfirmationModal({ doctor, slot, patientName, onConfirm }: Props) {
  const [smsOptIn, setSmsOptIn] = useState(false);

  return (
    <motion.div
      initial={{ opacity: 0, y: 12, scale: 0.97 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 8, scale: 0.97 }}
      transition={{ duration: 0.25, ease: 'easeOut' }}
      className="glass mx-4 mb-4 p-5"
      style={{ borderRadius: 14 }}
    >
      <div className="text-xs font-medium mb-4" style={{ color: 'var(--accent)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
        Confirm Appointment
      </div>

      <div className="space-y-3 mb-4">
        <Row label="Patient" value={patientName} />
        <Row label="Doctor" value={doctor.name} />
        <Row label="Specialty" value={doctor.specialty} />
        <Row label="When" value={formatSlot(slot)} />
      </div>

      <div
        className="flex items-center justify-between px-3 py-3 rounded-xl mb-4"
        style={{ background: 'rgba(87,125,232,0.08)', border: '1px solid rgba(87,125,232,0.18)' }}
      >
        <div>
          <div className="text-sm font-medium">SMS Reminder</div>
          <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
            24-hour reminder to your phone
          </div>
        </div>
        <button
          onClick={() => setSmsOptIn(v => !v)}
          className="relative w-11 h-6 rounded-full transition-all flex-shrink-0"
          style={{
            background: smsOptIn ? 'var(--primary)' : 'rgba(255,255,255,0.1)',
            border: smsOptIn ? '1px solid transparent' : '1px solid rgba(255,255,255,0.15)',
            cursor: 'pointer',
          }}
        >
          <span
            className="absolute top-0.5 w-5 h-5 rounded-full transition-all"
            style={{
              left: smsOptIn ? 'calc(100% - 22px)' : '2px',
              background: '#fff',
              boxShadow: '0 1px 4px rgba(0,0,0,0.3)',
            }}
          />
        </button>
      </div>

      <motion.button
        whileTap={{ scale: 0.96 }}
        onClick={() => onConfirm(smsOptIn)}
        className="w-full py-3 rounded-xl text-sm font-semibold text-white"
        style={{ background: 'var(--success)', cursor: 'pointer' }}
      >
        Confirm Booking
      </motion.button>
    </motion.div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="text-xs flex-shrink-0" style={{ color: 'var(--text-muted)', minWidth: 64 }}>{label}</span>
      <span className="text-xs text-right font-medium">{value}</span>
    </div>
  );
}
