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
