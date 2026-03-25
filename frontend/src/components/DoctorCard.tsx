'use client';

import { motion } from 'framer-motion';
import { Doctor } from '@/types';

interface Props {
  doctor: Doctor;
  onConfirm?: () => void;
}

const specialtyIcons: Record<string, string> = {
  Cardiology: '❤️',
  Orthopedics: '🦴',
  Dermatology: '🩺',
  Gastroenterology: '🫀',
  Neurology: '🧠',
};

export function DoctorCard({ doctor, onConfirm }: Props) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 10, scale: 0.97 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 8, scale: 0.97 }}
      transition={{ duration: 0.25, ease: 'easeOut' }}
      className="glass mx-4 mb-3 p-4"
      style={{ borderRadius: 14 }}
    >
      <div className="text-xs font-medium mb-3" style={{ color: 'var(--accent)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
        {onConfirm ? 'Recommended Specialist' : 'Scheduling With'}
      </div>

      <div className="flex items-center gap-3">
        <div
          className="w-11 h-11 rounded-2xl flex items-center justify-center text-sm font-bold flex-shrink-0"
          style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)', color: '#fff' }}
        >
          {doctor.imageInitials}
        </div>

        <div className="flex-1 min-w-0">
          <div className="font-semibold text-sm">{doctor.name}</div>
          <div className="text-xs mt-0.5 flex items-center gap-1.5" style={{ color: 'var(--accent)' }}>
            <span>{specialtyIcons[doctor.specialty] ?? '🏥'}</span>
            <span>{doctor.specialty}</span>
          </div>
          {doctor.bio && (
            <div className="text-xs mt-1 leading-relaxed line-clamp-2" style={{ color: 'var(--text-muted)' }}>
              {doctor.bio}
            </div>
          )}
        </div>
      </div>

      {onConfirm && (
        <motion.button
          whileTap={{ scale: 0.96 }}
          onClick={onConfirm}
          className="w-full mt-4 py-2.5 rounded-xl text-sm font-medium text-white"
          style={{ background: 'var(--primary)', cursor: 'pointer' }}
        >
          Confirm — Schedule with {doctor.name.split(' ').slice(1).join(' ')}
        </motion.button>
      )}
    </motion.div>
  );
}
