## Plan: Voice/Form Sync with Dashboard-Managed VAPI Tools

VAPI tools are now configured in the VAPI dashboard (not injected from backend code), so implementation should focus on reliable webhook payload handling, session hydration, and bidirectional intake UX: (1) voice-collected intake populates the form/session, and (2) voice reads back current intake values and asks only for missing fields. Roll out in phases: post-call sync first, optional live sync later.

**Steps**
1. Baseline validation (already edited): keep backend `voice.go` aligned with dashboard-managed tools by relying on incoming tool calls, robust payload parsing, and metadata/phone session resolution.
2. Align intake schema between backend session payload and frontend types so voice-collected fields map cleanly into UI form fields. *depends on 1*
3. Extend intake form API to accept external initial values and safe refresh behavior when session data changes, without overwriting active manual edits. *depends on 2*
4. Wire call-end sync to hydrate the intake form from `GET /api/session`; if intake is complete, continue state flow without forcing form re-entry. *depends on 3*
5. Add backend readback behavior that uses current session `PatientInfo` as the source of truth for spoken confirmation (name, DOB, phone, email, reason) and prompts only for missing/invalid fields. *depends on 1*
6. Refine voice prompt wording for short spoken turns and explicit confirmation loops, while keeping dashboard tool names/contracts unchanged. *depends on 5*
7. Optional: add near-real-time voice-to-form updates during active calls via periodic session refresh or event stream + guarded merge policy. *depends on 4; can be deferred*
8. Add/adjust tests for webhook payload variants, session resolution fallback, form prefill behavior, and readback content/edge cases. *depends on 4 and 5*

**Relevant files**
- `/Users/vighaneshs/medical-ai-agent-demo/backend/handlers/voice.go` — now source of webhook robustness/session resolution/logging for dashboard-managed tools
- `/Users/vighaneshs/medical-ai-agent-demo/backend/handlers/voice_test.go` — extend coverage for payload shapes and provider/model override assumptions
- `/Users/vighaneshs/medical-ai-agent-demo/backend/handlers/session.go` — ensure session response includes complete intake payload for frontend hydration
- `/Users/vighaneshs/medical-ai-agent-demo/backend/services/prompts.go` — tune readback/missing-field spoken behavior
- `/Users/vighaneshs/medical-ai-agent-demo/backend/models/types.go` — source of truth for patient/session fields and state transitions
- `/Users/vighaneshs/medical-ai-agent-demo/frontend/src/lib/api.ts` — map session payload into frontend intake type
- `/Users/vighaneshs/medical-ai-agent-demo/frontend/src/types/index.ts` — shared intake/patient typing for UI hydration
- `/Users/vighaneshs/medical-ai-agent-demo/frontend/src/components/IntakeForm.tsx` — prefill + non-destructive merge logic
- `/Users/vighaneshs/medical-ai-agent-demo/frontend/src/components/ChatInterface/index.tsx` — call-end session sync and intake UI/state branching
- `/Users/vighaneshs/medical-ai-agent-demo/frontend/src/components/VoiceCallButton.tsx` — ensure onCallEnd sequencing triggers sync before UI decisions
- `/Users/vighaneshs/medical-ai-agent-demo/frontend/src/__tests__/intakeform.test.tsx` — prefill + dirty-field preservation tests

**Verification**
1. Backend tests: run `go test ./...` in `/Users/vighaneshs/medical-ai-agent-demo/backend` and verify voice handler tests cover dashboard-style payloads (`message.toolCallList`, top-level fallback, metadata locations, phone fallback).
2. Frontend tests: run targeted intake/chat tests and full test suite in `/Users/vighaneshs/medical-ai-agent-demo/frontend`.
3. Manual flow A (voice to form): provide intake via VAPI call, end call, confirm form/session fields are hydrated correctly.
4. Manual flow B (form to voice readback): edit intake in UI, trigger voice continuation, verify spoken confirmation uses latest session values and requests only missing fields.
5. Regression flow: complete booking via chat-only and voice-only paths to confirm state-machine parity.

**Decisions**
- Included: both requested capabilities under dashboard-managed tool configuration.
- Included: server-authoritative session model remains canonical for voice and form.
- Excluded (initial): backend-generated VAPI tool definitions (removed from scope by current architecture).
- Excluded (initial): per-utterance live form mutation during active call (optional later phase).

**Further Considerations**
1. Observability: keep raw webhook payload logging temporary or redacted in production to reduce PII risk.
2. Conflict policy: preserve actively edited form fields over incoming session updates during merge.
3. Dashboard governance: document required VAPI tool names/params outside code to avoid drift.
