import { SSEChunk } from '@/types';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export async function sendMessage(
  sessionId: string,
  message: string,
  onChunk: (chunk: SSEChunk) => void,
): Promise<void> {
  const res = await fetch(`${API_URL}/api/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId, message }),
  });

  if (!res.ok || !res.body) {
    throw new Error('Chat request failed');
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() ?? '';

    for (const rawLine of lines) {
      const line = rawLine.trimEnd(); // strip \r and trailing whitespace
      if (!line.startsWith('data: ')) continue;
      const payload = line.slice(6);
      try {
        const chunk: SSEChunk = JSON.parse(payload);
        onChunk(chunk);
      } catch {
        console.warn('[SSE] failed to parse chunk:', payload);
      }
    }
  }
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

export async function initiatePhoneCall(sessionId: string, phone: string) {
  const res = await fetch(`${API_URL}/api/voice/call-phone`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId, phone }),
  });
  if (!res.ok) throw new Error('Failed to place phone call');
  return res.json();
}
