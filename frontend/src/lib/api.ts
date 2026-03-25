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

    for (const line of lines) {
      if (!line.startsWith('data: ')) continue;
      try {
        const chunk: SSEChunk = JSON.parse(line.slice(6));
        onChunk(chunk);
      } catch {
        // skip malformed
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

export async function initiateVoiceCall(sessionId: string) {
  const res = await fetch(`${API_URL}/api/voice/initiate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sessionId }),
  });
  if (!res.ok) throw new Error('Failed to initiate voice call');
  return res.json();
}
