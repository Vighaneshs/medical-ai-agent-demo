import { ChatInterface } from '@/components/ChatInterface';

export default function Home() {
  return (
    <div
      className="w-screen overflow-hidden flex items-center justify-center"
      style={{ height: 'calc(100vh - var(--nav-height))' }}
    >
      <div
        className="glass h-full w-full flex flex-col"
        style={{ maxWidth: 720, borderRadius: 0 }}
      >
        <ChatInterface />
      </div>
    </div>
  );
}
