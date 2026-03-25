# medical-ai-agent-demo

# Backend
cp .env.example backend/.env  # fill in ANTHROPIC_API_KEY at minimum
cd backend && go run .

# Frontend (new terminal)
cd frontend
echo "NEXT_PUBLIC_API_URL=http://localhost:8080" > .env.local
npm run dev