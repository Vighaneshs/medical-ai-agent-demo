import { Doctor } from '@/types';
import { DoctorProfileCard } from '@/components/DoctorProfileCard';

async function fetchDoctors(): Promise<Doctor[]> {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
  const res = await fetch(`${apiUrl}/api/doctors`, { cache: 'no-store' });
  if (!res.ok) return [];
  const data = await res.json();
  return data.doctors ?? [];
}

export default async function DoctorsPage() {
  const doctors = await fetchDoctors();

  return (
    <div className="max-w-2xl mx-auto px-4 py-10">
      <h1 className="text-2xl font-bold mb-2">Our Doctors</h1>
      <p className="text-sm mb-6" style={{ color: 'var(--text-muted)' }}>
        Meet our team of specialist physicians. Chat with Kyron to book an appointment.
      </p>

      {doctors.length === 0 ? (
        <div className="glass p-6 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
          Unable to load doctors right now. Please try again.
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {doctors.map((doctor) => (
            <DoctorProfileCard key={doctor.id} doctor={doctor} />
          ))}
        </div>
      )}
    </div>
  );
}
