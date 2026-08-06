export type AuthCredentials = {
    email: string;
    password: string;
};

export type AuthTokens = {
    accessToken: string;
    refreshToken: string;
};

type TokensPayload = {
    accessToken?: unknown;
    access_token?: unknown;
    refreshToken?: unknown;
    refresh_token?: unknown;
    tokens?: unknown;
    data?: unknown;
};

export class ApiError extends Error {
    constructor(
        message: string,
        public readonly status: number
    ) {
        super(message);
        this.name = 'ApiError';
    }
}

const apiUrl = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

async function request(path: string, body: unknown): Promise<AuthTokens> {
    const response = await fetch(`${apiUrl}${path}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
    const payload: unknown = await response.json().catch(() => null);

    if (!response.ok) {
        throw new ApiError('Не удалось выполнить запрос авторизации', response.status);
    }

    return getTokens(payload);
}

async function requestWithoutResponse(path: string, body: unknown): Promise<void> {
    const response = await fetch(`${apiUrl}${path}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });

    if (!response.ok) {
        throw new ApiError('Не удалось выполнить запрос авторизации', response.status);
    }
}

function getTokens(payload: unknown): AuthTokens {
    const root = payload as TokensPayload | null;
    const tokens = (root?.tokens ?? root?.data ?? root) as TokensPayload | null;
    const accessToken = tokens?.accessToken ?? tokens?.access_token;
    const refreshToken = tokens?.refreshToken ?? tokens?.refresh_token;

    if (typeof accessToken !== 'string' || typeof refreshToken !== 'string') {
        throw new Error('Сервер вернул некорректные токены авторизации');
    }

    return { accessToken, refreshToken };
}

export const authApi = {
    signUp: (credentials: AuthCredentials) => request('/auth/register', credentials),
    signIn: (credentials: AuthCredentials) => request('/auth/login', credentials),
    refresh: (refreshToken: string) => request('/auth/refresh', { refreshToken }),
    logout: (refreshToken: string) => requestWithoutResponse('/auth/logout', { refreshToken }),
};
