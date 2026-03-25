'use client';

import { useState } from 'react';

interface IntakeFormProps {
  onSubmit: (message: string) => void;
  disabled?: boolean;
}

interface FormFields {
  firstName: string;
  lastName: string;
  dob: string;
  phone: string;
  email: string;
  reason: string;
}

const EMPTY: FormFields = {
  firstName: '', lastName: '', dob: '', phone: '', email: '', reason: '',
};

const inputStyle = {
  background: 'rgba(255,255,255,0.05)',
  border: '1px solid var(--glass-border)',
  borderRadius: 10,
  color: 'var(--text)',
  padding: '8px 12px',
  fontSize: 13,
  width: '100%',
  outline: 'none',
};

const errorStyle = {
  ...inputStyle,
  border: '1px solid var(--danger)',
};

export function IntakeForm({ onSubmit, disabled }: IntakeFormProps) {
  const [fields, setFields] = useState<FormFields>(EMPTY);
  const [touched, setTouched] = useState<Partial<Record<keyof FormFields, boolean>>>({});

  function set(key: keyof FormFields, value: string) {
    setFields(prev => ({ ...prev, [key]: value }));
  }

  function touch(key: keyof FormFields) {
    setTouched(prev => ({ ...prev, [key]: true }));
  }

  function validate(key: keyof FormFields, value: string): string | null {
    const v = value.trim();
    if (!v) return 'Required';
    switch (key) {
      case 'firstName':
      case 'lastName':
        if (v.length < 2) return 'Too short';
        if (!/^[A-Za-z\s'-]+$/.test(v)) return 'Letters only';
        break;
      case 'dob': {
        const d = new Date(v);
        const today = new Date();
        if (isNaN(d.getTime())) return 'Invalid date';
        if (d > today) return 'Cannot be in the future';
        if (today.getFullYear() - d.getFullYear() > 120) return 'Invalid date';
        break;
      }
      case 'phone':
        if (v.replace(/\D/g, '').length < 10) return 'At least 10 digits';
        break;
      case 'email':
        if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)) return 'Invalid email';
        break;
      case 'reason':
        if (v.length < 5) return 'Please describe your reason';
        break;
    }
    return null;
  }

  function hasError(key: keyof FormFields) {
    return touched[key] && validate(key, fields[key]) !== null;
  }

  function errorMsg(key: keyof FormFields) {
    if (!touched[key]) return null;
    return validate(key, fields[key]);
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const allTouched: Partial<Record<keyof FormFields, boolean>> = {};
    (Object.keys(EMPTY) as (keyof FormFields)[]).forEach(k => { allTouched[k] = true; });
    setTouched(allTouched);

    if ((Object.keys(EMPTY) as (keyof FormFields)[]).some(k => validate(k, fields[k]) !== null)) return;

    const text =
      `My name is ${fields.firstName.trim()} ${fields.lastName.trim()}, ` +
      `date of birth ${fields.dob}, ` +
      `phone ${fields.phone.trim()}, ` +
      `email ${fields.email.trim()}. ` +
      `Reason for visit: ${fields.reason.trim()}.`;

    onSubmit(text);
  }

  return (
    <div
      className="glass-sm mx-1"
      style={{ padding: '16px 18px', maxWidth: 480 }}
    >
      <p className="text-xs font-semibold mb-3 uppercase tracking-wide" style={{ color: 'var(--text-muted)' }}>
        Patient Information
      </p>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        {/* Row 1: First + Last name */}
        <div className="flex gap-2">
          <div className="flex flex-col gap-1 flex-1">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>First name</label>
            <input
              type="text"
              placeholder="Jane"
              value={fields.firstName}
              onChange={e => set('firstName', e.target.value)}
              onBlur={() => touch('firstName')}
              disabled={disabled}
              style={hasError('firstName') ? errorStyle : inputStyle}
            />
            {errorMsg('firstName') && <span className="text-xs" style={{ color: 'var(--danger)' }}>{errorMsg('firstName')}</span>}
          </div>
          <div className="flex flex-col gap-1 flex-1">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Last name</label>
            <input
              type="text"
              placeholder="Smith"
              value={fields.lastName}
              onChange={e => set('lastName', e.target.value)}
              onBlur={() => touch('lastName')}
              disabled={disabled}
              style={hasError('lastName') ? errorStyle : inputStyle}
            />
            {errorMsg('lastName') && <span className="text-xs" style={{ color: 'var(--danger)' }}>{errorMsg('lastName')}</span>}
          </div>
        </div>

        {/* Row 2: DOB + Phone */}
        <div className="flex gap-2">
          <div className="flex flex-col gap-1 flex-1">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Date of birth</label>
            <input
              type="date"
              value={fields.dob}
              max={new Date().toISOString().split('T')[0]}
              onChange={e => set('dob', e.target.value)}
              onBlur={() => touch('dob')}
              disabled={disabled}
              style={{
                ...(hasError('dob') ? errorStyle : inputStyle),
                colorScheme: 'dark',
              }}
            />
            {errorMsg('dob') && <span className="text-xs" style={{ color: 'var(--danger)' }}>{errorMsg('dob')}</span>}
          </div>
          <div className="flex flex-col gap-1 flex-1">
            <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Phone</label>
            <input
              type="tel"
              placeholder="(555) 000-0000"
              value={fields.phone}
              onChange={e => set('phone', e.target.value)}
              onBlur={() => touch('phone')}
              disabled={disabled}
              style={hasError('phone') ? errorStyle : inputStyle}
            />
            {errorMsg('phone') && <span className="text-xs" style={{ color: 'var(--danger)' }}>{errorMsg('phone')}</span>}
          </div>
        </div>

        {/* Row 3: Email */}
        <div className="flex flex-col gap-1">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Email</label>
          <input
            type="email"
            placeholder="jane@example.com"
            value={fields.email}
            onChange={e => set('email', e.target.value)}
            onBlur={() => touch('email')}
            disabled={disabled}
            style={hasError('email') ? errorStyle : inputStyle}
          />
          {errorMsg('email') && <span className="text-xs" style={{ color: 'var(--danger)' }}>{errorMsg('email')}</span>}
        </div>

        {/* Row 4: Reason */}
        <div className="flex flex-col gap-1">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Reason for visit</label>
          <textarea
            rows={3}
            placeholder="Describe your symptoms or reason…"
            value={fields.reason}
            onChange={e => set('reason', e.target.value)}
            onBlur={() => touch('reason')}
            disabled={disabled}
            style={{
              ...(hasError('reason') ? errorStyle : inputStyle),
              resize: 'none',
            }}
          />
          {errorMsg('reason') && <span className="text-xs" style={{ color: 'var(--danger)' }}>{errorMsg('reason')}</span>}
        </div>

        <div className="flex justify-end mt-1">
          <button
            type="submit"
            disabled={disabled}
            className="px-5 py-2 rounded-xl text-sm font-semibold transition-opacity"
            style={{
              background: 'linear-gradient(135deg, #577DE8, #48ACF0)',
              color: '#fff',
              opacity: disabled ? 0.5 : 1,
            }}
          >
            Continue →
          </button>
        </div>
      </form>
    </div>
  );
}
