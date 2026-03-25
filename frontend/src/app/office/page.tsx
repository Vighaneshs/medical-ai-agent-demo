export default function OfficePage() {
  return (
    <div className="max-w-2xl mx-auto px-4 py-10">
      <h1 className="text-2xl font-bold mb-6">Office Information</h1>

      <div className="glass p-6 mb-4 flex flex-col gap-5">
        <Section title="Hours">
          <Row label="Monday – Friday" value="8:00 AM – 6:00 PM" />
          <Row label="Saturday" value="9:00 AM – 2:00 PM" />
          <Row label="Sunday" value="Closed" />
        </Section>

        <div style={{ borderTop: '1px solid var(--glass-border)' }} />

        <Section title="Location">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            123 Wellness Avenue, Suite 400<br />
            San Francisco, CA 94102
          </p>
          <a
            href="https://maps.google.com/?q=123+Wellness+Avenue+San+Francisco+CA"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium mt-2 inline-block"
            style={{ color: '#7BA4EF' }}
          >
            Open in Google Maps →
          </a>
        </Section>

        <div style={{ borderTop: '1px solid var(--glass-border)' }} />

        <Section title="Contact">
          <Row label="Phone" value="(415) 555-0192" />
          <Row label="Email" value="hello@kyronmedical.com" />
          <Row label="Fax" value="(415) 555-0193" />
        </Section>

        <div style={{ borderTop: '1px solid var(--glass-border)' }} />

        <Section title="Parking & Transit">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            Free parking available in the building garage (enter from Wellness Ave).
            Nearest BART station: Civic Center, 5-minute walk.
          </p>
        </Section>
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
    <div className="flex justify-between items-center text-sm py-1">
      <span style={{ color: 'var(--text-muted)' }}>{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}
