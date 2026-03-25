'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { Appointment } from '@/types';
import { getAppointment } from '@/lib/api';
import { SESSION_KEY } from '@/lib/constants';

export default function AppointmentPage() {
  const [appointment, setAppointment] = useState<Appointment | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    const sessionId = localStorage.getItem(SESSION_KEY);
    if (!sessionId) {
      setLoading(false);
      setError(true);
      return;
    }
    getAppointment(sessionId)
      .then(setAppointment)
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-10">
        <div className="glass p-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
          Loading your appointment…
        </div>
      </div>
    );
  }

  if (error || !appointment) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-10">
        <h1 className="text-2xl font-bold mb-4">My Appointment</h1>
        <div className="glass p-8 text-center">
          <div className="text-3xl mb-3">📅</div>
          <p className="text-sm mb-4" style={{ color: 'var(--text-muted)' }}>
            No appointment booked yet.
          </p>
          <Link
            href="/"
            className="text-sm font-medium px-4 py-2 rounded-full"
            style={{ background: 'rgba(87,125,232,0.15)', color: '#7BA4EF', border: '1px solid rgba(87,125,232,0.3)' }}
          >
            Chat with Kyron to book one →
          </Link>
        </div>
      </div>
    );
  }

  const d = new Date(appointment.slot.date + 'T00:00:00');
  const dateStr = d.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
  const [h, m] = appointment.slot.startTime.split(':').map(Number);
  const ampm = h >= 12 ? 'PM' : 'AM';
  const h12 = h % 12 || 12;
  const timeStr = `${h12}:${m.toString().padStart(2, '0')} ${ampm}`;

  return (
    <div className="max-w-2xl mx-auto px-4 py-10">
      <h1 className="text-2xl font-bold mb-6">My Appointment</h1>

      {/* Confirmation banner */}
      <div
        className="rounded-xl px-5 py-4 mb-4 border flex items-center gap-3"
        style={{ background: 'rgba(46,168,74,0.1)', borderColor: 'rgba(46,168,74,0.3)' }}
      >
        <span className="text-xl">✅</span>
        <div>
          <div className="text-sm font-semibold" style={{ color: '#7df5a0' }}>Appointment Confirmed</div>
          <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
            Booked on {new Date(appointment.bookedAt).toLocaleDateString()}
          </div>
        </div>
      </div>

      <div className="glass p-6 flex flex-col gap-5">
        {/* Doctor */}
        <Section title="Doctor">
          <div className="flex items-center gap-3">
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center text-sm font-bold flex-shrink-0"
              style={{ background: 'linear-gradient(135deg, #577DE8, #48ACF0)' }}
            >
              {appointment.doctor.imageInitials}
            </div>
            <div>
              <div className="text-sm font-semibold">{appointment.doctor.name}</div>
              <div className="text-xs mt-0.5" style={{ color: '#7BA4EF' }}>{appointment.doctor.specialty}</div>
            </div>
          </div>
        </Section>

        <Divider />

        {/* Date & time */}
        <Section title="Date & Time">
          <Row label="Date" value={dateStr} />
          <Row label="Time" value={`${timeStr} – ${appointment.slot.endTime}`} />
        </Section>

        <Divider />

        {/* Patient */}
        <Section title="Patient">
          <Row label="Name" value={`${appointment.patient.firstName} ${appointment.patient.lastName}`} />
          <Row label="Date of Birth" value={appointment.patient.dob} />
          <Row label="Phone" value={appointment.patient.phone} />
          <Row label="Email" value={appointment.patient.email} />
          <Row label="Reason" value={appointment.patient.reasonForVisit} />
        </Section>

        {appointment.patient.smsOptIn && (
          <>
            <Divider />
            <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
              📱 SMS reminders enabled for this appointment.
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h2 className="text-xs font-semibold uppercase tracking-wider mb-3" style={{ color: 'var(--text-muted)' }}>
        {title}
      </h2>
      {children}
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-start text-sm py-1 gap-4">
      <span className="flex-shrink-0" style={{ color: 'var(--text-muted)' }}>{label}</span>
      <span className="font-medium text-right">{value}</span>
    </div>
  );
}

function Divider() {
  return <div style={{ borderTop: '1px solid var(--glass-border)' }} />;
}
