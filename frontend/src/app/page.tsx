import { ChatInterface } from '@/components/ChatInterface';

export default function Home() {
  return (
    <div className="gradient-bg h-screen w-screen overflow-hidden flex items-center justify-center">
      <div
        className="glass h-full w-full flex flex-col"
        style={{ maxWidth: 720, borderRadius: 0 }}
      >
        <ChatInterface />
      </div>
    </div>
  );
}
