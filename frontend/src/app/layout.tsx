import type { Metadata } from 'next';
import './globals.css';
import { NavBar } from '@/components/NavBar';
import { AuthGuard } from '@/components/AuthGuard';

export const metadata: Metadata = {
  title: 'Kyron Medical — AI Care Coordinator',
  description: 'Schedule appointments and manage your care with Kyron Medical AI.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="gradient-bg min-h-screen">
        <NavBar />
        <AuthGuard>
          <main>{children}</main>
        </AuthGuard>
      </body>
    </html>
  );
}
