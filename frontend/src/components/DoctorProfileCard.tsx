import { Doctor } from '@/types';

export function DoctorProfileCard({ doctor }: { doctor: Doctor }) {
  return (
    <div className="glass-sm p-5 flex gap-4 items-start">
      {/* Avatar */}
      <div
        className="flex-shrink-0 w-14 h-14 rounded-2xl flex items-center justify-center text-lg font-bold"
        style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)' }}
      >
        {doctor.imageInitials}
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <div className="font-semibold text-sm">{doctor.name}</div>
        <div className="text-xs mt-0.5 mb-2" style={{ color: '#7BA4EF' }}>
          {doctor.specialty}
        </div>
        <p className="text-xs leading-relaxed" style={{ color: 'var(--text-muted)' }}>
          {doctor.bio}
        </p>
        {doctor.phone && (
          <div className="text-xs mt-3" style={{ color: 'var(--text-muted)' }}>
            <span className="font-medium" style={{ color: 'rgba(255,255,255,0.7)' }}>Phone: </span>
            {doctor.phone}
          </div>
        )}
      </div>

      {/* Available badge */}
      <div
        className="flex-shrink-0 text-xs font-medium px-2 py-1 rounded-full border"
        style={{
          background: 'rgba(46, 168, 74, 0.15)',
          borderColor: 'rgba(46, 168, 74, 0.3)',
          color: '#7df5a0',
        }}
      >
        Accepting patients
      </div>
    </div>
  );
}
