import React from 'react';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { VoiceCallButton } from '@/components/VoiceCallButton';
import { initiateVoiceCall, initiatePhoneCall } from '@/lib/api';

// ── Mocks ─────────────────────────────────────────────────────────────────────

jest.mock('@/lib/api', () => ({
  initiateVoiceCall: jest.fn(),
  initiatePhoneCall: jest.fn(),
}));

// Mock @vapi-ai/web dynamic import used inside startBrowserCall
jest.mock('@vapi-ai/web', () => {
  const mockVapi = {
    on: jest.fn(),
    start: jest.fn().mockResolvedValue(undefined),
    stop: jest.fn(),
  };
  return { default: jest.fn(() => mockVapi) };
});

// Mock framer-motion — render children directly, skip animation wrappers
jest.mock('framer-motion', () => ({
  motion: {
    button: React.forwardRef(({ children, ...props }: React.PropsWithChildren<React.ButtonHTMLAttributes<HTMLButtonElement>>, ref: React.Ref<HTMLButtonElement>) => (
      <button ref={ref} {...props}>{children}</button>
    )),
    div: React.forwardRef(({ children, ...props }: React.PropsWithChildren<React.HTMLAttributes<HTMLDivElement>>, ref: React.Ref<HTMLDivElement>) => (
      <div ref={ref} {...props}>{children}</div>
    )),
    span: React.forwardRef(({ children, ...props }: React.PropsWithChildren<React.HTMLAttributes<HTMLSpanElement>>, ref: React.Ref<HTMLSpanElement>) => (
      <span ref={ref} {...props}>{children}</span>
    )),
  },
  AnimatePresence: ({ children }: React.PropsWithChildren) => <>{children}</>,
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

const mockedInitiateVoiceCall = initiateVoiceCall as jest.MockedFunction<typeof initiateVoiceCall>;
const mockedInitiatePhoneCall = initiatePhoneCall as jest.MockedFunction<typeof initiatePhoneCall>;

const defaultProps = {
  sessionId: 'session_test',
  sessionState: 'GREETING' as const,
};

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('VoiceCallButton', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  // ── 1. Renders both call buttons ───────────────────────────────────────────

  it('renders both call buttons', () => {
    render(<VoiceCallButton {...defaultProps} />);

    expect(screen.getByTitle('Call in browser')).toBeInTheDocument();
    expect(screen.getByTitle('Call my phone')).toBeInTheDocument();
  });

  // ── 2. Phone input hidden by default ──────────────────────────────────────

  it('phone input hidden by default', () => {
    render(<VoiceCallButton {...defaultProps} />);
    expect(screen.queryByPlaceholderText('+1 (555) 000-0000')).not.toBeInTheDocument();
  });

  // ── 3. Clicking "Call my phone" shows phone input ─────────────────────────

  it('clicking call my phone shows phone input', () => {
    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call my phone'));
    expect(screen.getByPlaceholderText('+1 (555) 000-0000')).toBeInTheDocument();
  });

  // ── 4. Clicking "Call my phone" again hides phone input ───────────────────

  it('clicking call my phone again hides phone input', () => {
    render(<VoiceCallButton {...defaultProps} />);

    fireEvent.click(screen.getByTitle('Call my phone'));
    expect(screen.getByPlaceholderText('+1 (555) 000-0000')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Call my phone'));
    expect(screen.queryByPlaceholderText('+1 (555) 000-0000')).not.toBeInTheDocument();
  });

  // ── 5. "Call me" button disabled when phone is empty ─────────────────────

  it('call me button disabled when phone is empty', () => {
    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call my phone'));

    const callMeButton = screen.getByRole('button', { name: /call me/i });
    expect(callMeButton).toBeDisabled();
  });

  // ── 6. "Call me" button enabled when phone has value ─────────────────────

  it('call me button enabled when phone has value', () => {
    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call my phone'));

    const phoneInput = screen.getByPlaceholderText('+1 (555) 000-0000');
    fireEvent.change(phoneInput, { target: { value: '+1 555-0000' } });

    const callMeButton = screen.getByRole('button', { name: /call me/i });
    expect(callMeButton).not.toBeDisabled();
  });

  // ── 7. Phone call success shows success message ───────────────────────────

  it('phone call success shows success message', async () => {
    mockedInitiatePhoneCall.mockResolvedValue({ success: true });

    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call my phone'));

    fireEvent.change(screen.getByPlaceholderText('+1 (555) 000-0000'), {
      target: { value: '+1 555-0000' },
    });
    fireEvent.click(screen.getByRole('button', { name: /call me/i }));

    await waitFor(() => {
      expect(screen.getByText('Calling your phone now…')).toBeInTheDocument();
    });
  });

  // ── 8. Phone call error shows error message ───────────────────────────────

  it('phone call error shows error message', async () => {
    mockedInitiatePhoneCall.mockRejectedValue(new Error('Network error'));

    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call my phone'));

    fireEvent.change(screen.getByPlaceholderText('+1 (555) 000-0000'), {
      target: { value: '+1 555-0000' },
    });
    fireEvent.click(screen.getByRole('button', { name: /call me/i }));

    await waitFor(() => {
      expect(screen.getByText('Could not place call. Please try again.')).toBeInTheDocument();
    });
  });

  // ── 9. Browser call error shows error ─────────────────────────────────────

  it('browser call error shows error', async () => {
    mockedInitiateVoiceCall.mockRejectedValue(new Error('Server error'));

    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call in browser'));

    await waitFor(() => {
      expect(screen.getByText('Could not start call. Please try again.')).toBeInTheDocument();
    });
  });

  // ── 10. Call in browser button shows connecting state ─────────────────────

  it('call in browser button shows connecting state', async () => {
    // Make initiateVoiceCall hang so we can observe the connecting state
    let resolveCall!: (value: unknown) => void;
    mockedInitiateVoiceCall.mockReturnValue(
      new Promise(resolve => { resolveCall = resolve; }),
    );

    render(<VoiceCallButton {...defaultProps} />);
    fireEvent.click(screen.getByTitle('Call in browser'));

    // Button text changes to "Connecting…" while the promise is pending
    await waitFor(() => {
      expect(screen.getByText('Connecting…')).toBeInTheDocument();
    });

    // The browser-call button should now be disabled (browserState === 'connecting')
    const callInBrowserBtn = screen.getByTitle('Call in browser');
    expect(callInBrowserBtn).toBeDisabled();

    // Clean up — resolve the promise so no act() warnings leak
    await act(async () => {
      resolveCall({ assistantId: 'asst_123', assistantOverrides: {} });
    });
  });
});
