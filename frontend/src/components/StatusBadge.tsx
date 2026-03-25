'use client';

import { SessionState } from '@/types';

const labels: Record<SessionState, string> = {
  GREETING:     'Welcome',
  INTAKE:       'Collecting Info',
  MATCHING:     'Matching Doctor',
  SCHEDULING:   'Scheduling',
  CONFIRMING:   'Confirming',
  BOOKED:       'Booked ✓',
  PRESCRIPTION: 'Prescription',
  HOURS:        'Office Info',
};

export function StatusBadge({ state }: { state: SessionState }) {
  const cls = `badge-${state.toLowerCase()}`;
  return (
    <span
      className={`text-xs font-medium px-2.5 py-1 rounded-full border ${cls}`}
      style={{ letterSpacing: '0.02em' }}
    >
      {labels[state]}
    </span>
  );
}
