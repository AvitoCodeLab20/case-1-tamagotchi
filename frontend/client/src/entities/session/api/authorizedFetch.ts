import { refreshSession } from './session';
import { useSessionStore } from '../model/sessionStore';

function withAuthorization(init: RequestInit | undefined, accessToken: string): RequestInit {
    const headers = new Headers(init?.headers);
    headers.set('Authorization', `Bearer ${accessToken}`);

    return { ...init, headers };
}

export async function authorizedFetch(
    input: RequestInfo | URL,
    init?: RequestInit
): Promise<Response> {
    const accessToken = useSessionStore.getState().accessToken;
    if (!accessToken) {
        throw new Error('Для запроса нужна авторизованная сессия');
    }

    const response = await fetch(input, withAuthorization(init, accessToken));
    if (response.status !== 401) {
        return response;
    }

    await refreshSession();
    const renewedAccessToken = useSessionStore.getState().accessToken;
    if (!renewedAccessToken) {
        return response;
    }

    return fetch(input, withAuthorization(init, renewedAccessToken));
}
