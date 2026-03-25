import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'Kyron Medical — AI Care Coordinator',
  description: 'Schedule appointments and manage your care with Kyron Medical AI.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
