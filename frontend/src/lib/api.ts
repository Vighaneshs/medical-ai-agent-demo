import { SSEChunk } from '@/types';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1500;

async function attemptSendMessage(
  sessionId: string,
  message: string,
  onChunk: (chunk: SSEChunk) => void,
  receivedData: { flag: boolean },
): Promise<void> {
  const res = await fetch(`${API_URL}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId, message }),
  });

  if (!res.ok || !res.body) {
    throw new Error(`Chat request failed: ${res.status}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let receivedDone = false;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() ?? '';

    for (const rawLine of lines) {
      const line = rawLine.trimEnd();
      if (!line.startsWith('data: ')) continue;
      const payload = line.slice(6);
      try {
        const chunk: SSEChunk = JSON.parse(payload);
        receivedData.flag = true;
        if (chunk.done) receivedDone = true;
        onChunk(chunk);
      } catch {
        console.warn('[SSE] failed to parse chunk:', payload);
      }
    }
  }

  if (!receivedDone) {
    throw new Error('Stream ended without done event');
  }
}

export async function sendMessage(
  sessionId: string,
  message: string,
  onChunk: (chunk: SSEChunk) => void,
): Promise<void> {
  let lastError: Error = new Error('Unknown error');
  const receivedData = { flag: false };

  for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
    try {
      await attemptSendMessage(sessionId, message, onChunk, receivedData);
      return;
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err));
      console.warn(`[chat] attempt ${attempt}/${MAX_RETRIES} failed:`, lastError.message);

      // Don't retry if we already received partial data — retrying would re-fire onChunk
      // for the same events, creating duplicate or extra message bubbles.
      if (receivedData.flag) break;

      // Don't retry on 4xx client errors — they won't recover
      if (lastError.message.includes('400') || lastError.message.includes('401')) break;

      if (attempt < MAX_RETRIES) {
        await new Promise(resolve => setTimeout(resolve, RETRY_DELAY_MS * attempt));
      }
    }
  }

  throw lastError;
}

export async function getAvailability(doctorId: string) {
  const res = await fetch(`${API_URL}/api/availability?doctorId=${doctorId}`);
  if (!res.ok) throw new Error('Failed to fetch availability');
  const data = await res.json();
  return data.slots ?? data;
}

export async function getDoctors() {
  const res = await fetch(`${API_URL}/api/doctors`);
  if (!res.ok) throw new Error('Failed to fetch doctors');
  const data = await res.json();
  return data.doctors;
}

export async function getAppointment(sessionId: string) {
  const res = await fetch(`${API_URL}/api/appointment?sessionId=${sessionId}`);
  if (!res.ok) throw new Error('No appointment found');
  return res.json();
}

export async function initiateVoiceCall(sessionId: string) {
  const res = await fetch(`${API_URL}/api/voice/initiate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId }),
  });
  if (!res.ok) throw new Error('Failed to initiate voice call');
  return res.json();
}

export async function getSession(sessionId: string) {
  const res = await fetch(`${API_URL}/api/session?sessionId=${sessionId}`);
  if (!res.ok) throw new Error('Session not found');
  return res.json() as Promise<{
    state: string;
    patientFirstName?: string;
    doctorId?: string;
    selectedSlot?: import('@/types').TimeSlot;
    appointmentId?: string;
    messages?: import('@/types').ChatMessage[];
  }>;
}

export async function initiatePhoneCall(sessionId: string, phone: string) {
  const res = await fetch(`${API_URL}/api/voice/call-phone`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId, phone }),
  });
  if (!res.ok) throw new Error('Failed to place phone call');
  return res.json();
}
