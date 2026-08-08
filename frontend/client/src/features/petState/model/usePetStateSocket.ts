import { useEffect, useState } from 'react';

import { createPetStateWebSocketUrl, requestPetStateWebSocketTicket } from '../api/petStateSocket';
import type { PetState, PetStateSocketStatus } from './types';

const RECONNECT_DELAY_MS = 3_000;

type UsePetStateSocketResult = {
    petState: PetState | null;
    status: PetStateSocketStatus;
};

type TokenBoundState<T> = {
    accessToken: string;
    value: T;
};

export function usePetStateSocket(accessToken: string | null): UsePetStateSocketResult {
    const [petState, setPetState] = useState<TokenBoundState<PetState> | null>(null);
    const [status, setStatus] = useState<TokenBoundState<PetStateSocketStatus> | null>(null);

    useEffect(() => {
        if (!accessToken) {
            return;
        }

        let isDisposed = false;
        let socket: WebSocket | null = null;
        let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

        const connect = async (): Promise<void> => {
            try {
                const ticket = await requestPetStateWebSocketTicket(accessToken);

                if (isDisposed) {
                    return;
                }

                socket = new WebSocket(createPetStateWebSocketUrl(ticket));

                socket.onopen = () => {
                    if (!isDisposed) {
                        setStatus({ accessToken, value: 'connected' });
                    }
                };

                socket.onmessage = (event: MessageEvent<string>) => {
                    const nextPetState = parsePetState(event.data);

                    if (nextPetState && !isDisposed) {
                        setPetState({ accessToken, value: nextPetState });
                    }
                };

                socket.onerror = () => {
                    if (!isDisposed) {
                        setStatus({ accessToken, value: 'error' });
                    }
                };

                socket.onclose = () => {
                    if (isDisposed) {
                        return;
                    }

                    setStatus({ accessToken, value: 'connecting' });
                    reconnectTimer = setTimeout(() => {
                        void connect();
                    }, RECONNECT_DELAY_MS);
                };
            } catch {
                if (!isDisposed) {
                    setStatus({ accessToken, value: 'error' });
                    reconnectTimer = setTimeout(() => {
                        void connect();
                    }, RECONNECT_DELAY_MS);
                }
            }
        };

        void connect();

        return () => {
            isDisposed = true;

            if (reconnectTimer) {
                clearTimeout(reconnectTimer);
            }

            socket?.close();
        };
    }, [accessToken]);

    const currentPetState = petState?.accessToken === accessToken ? petState.value : null;
    const currentStatus = status?.accessToken === accessToken ? status.value : 'connecting';

    return {
        petState: accessToken ? currentPetState : null,
        status: accessToken ? currentStatus : 'idle',
    };
}

function parsePetState(message: string): PetState | null {
    try {
        const value: unknown = JSON.parse(message);

        if (!isPetState(value)) {
            return null;
        }

        return value;
    } catch {
        return null;
    }
}

function isPetState(value: unknown): value is PetState {
    if (typeof value !== 'object' || value === null) {
        return false;
    }

    const petState = value as Partial<PetState>;

    return [
        petState.level,
        petState.experience,
        petState.health,
        petState.hunger,
        petState.happiness,
        petState.energy,
    ].every((field) => typeof field === 'number');
}
