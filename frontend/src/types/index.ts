export type SessionState =
  | 'GREETING'
  | 'INTAKE'
  | 'MATCHING'
  | 'SCHEDULING'
  | 'CONFIRMING'
  | 'BOOKED'
  | 'PRESCRIPTION'
  | 'HOURS';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  createdAt: string;
  isEmergency?: boolean;
}

export interface Doctor {
  id: string;
  name: string;
  specialty: string;
  bio: string;
  imageInitials: string;
  phone: string;
}

export interface TimeSlot {
  doctorId: string;
  date: string;
  startTime: string;
  endTime: string;
  available: boolean;
}

export interface PatientInfo {
  firstName: string;
  lastName: string;
  dob: string;
  phone: string;
  email: string;
  reasonForVisit: string;
  smsOptIn: boolean;
}

export interface Appointment {
  id: string;
  sessionId: string;
  doctor: Doctor;
  slot: TimeSlot;
  patient: PatientInfo;
  bookedAt: string;
}

export interface SSEChunk {
  text?: string;
  emergency?: boolean;
  done?: boolean;
  newState?: SessionState;
  doctorId?: string;
  selectedSlot?: TimeSlot;
  appointmentId?: string;
  error?: string;
}
