import { authApi, type AuthCredentials, type AuthTokens } from './authApi';
import { getRefreshToken, removeRefreshToken, saveRefreshToken } from '../lib/refreshTokenStorage';
import { useSessionStore } from '../model/sessionStore';

let refreshPromise: Promise<void> | null = null;

function establishSession(tokens: AuthTokens): void {
    saveRefreshToken(tokens.refreshToken);
    useSessionStore.getState().startSession(tokens);
}

function clearSession(): void {
    removeRefreshToken();
    useSessionStore.getState().clearSession();
}

export async function signIn(credentials: AuthCredentials): Promise<void> {
    establishSession(await authApi.signIn(credentials));
}

export async function signUp(credentials: AuthCredentials): Promise<void> {
    establishSession(await authApi.signUp(credentials));
}

export async function refreshSession(): Promise<void> {
    if (refreshPromise) {
        return refreshPromise;
    }

    refreshPromise = refreshWithLock()
        .catch((error: unknown) => {
            clearSession();
            throw error;
        })
        .finally(() => {
            refreshPromise = null;
        });

    return refreshPromise;
}

async function refreshWithLock(): Promise<void> {
    const refresh = async () => {
        // Read after obtaining the browser-wide lock: another tab may have rotated it.
        const refreshToken = getRefreshToken();
        if (!refreshToken) {
            clearSession();
            return;
        }

        establishSession(await authApi.refresh(refreshToken));
    };

    if (navigator.locks) {
        await navigator.locks.request('tamagotchi-refresh-token', refresh);
        return;
    }

    await refresh();
}

export async function restoreSession(): Promise<void> {
    useSessionStore.getState().setChecking();

    try {
        await refreshSession();
    } catch {
        // An expired or revoked refresh token is a normal anonymous state.
    }
}

export async function logout(): Promise<void> {
    const refreshToken = getRefreshToken();
    clearSession();

    if (!refreshToken) {
        return;
    }

    try {
        await authApi.logout(refreshToken);
    } catch {
        // Local logout must succeed even if the network is unavailable.
    }
}
