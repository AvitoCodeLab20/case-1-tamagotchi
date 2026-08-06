import { create } from 'zustand';
import type { AuthTokens } from '../api/authApi';

export type SessionStatus = 'checking' | 'authenticated' | 'anonymous';

type SessionState = {
    status: SessionStatus;
    accessToken: string | null;
    startSession: (tokens: AuthTokens) => void;
    setChecking: () => void;
    clearSession: () => void;
};

export const useSessionStore = create<SessionState>((set) => ({
    status: 'checking',
    accessToken: null,
    startSession: ({ accessToken }) => set({ status: 'authenticated', accessToken }),
    setChecking: () => set({ status: 'checking' }),
    clearSession: () => set({ status: 'anonymous', accessToken: null }),
}));
