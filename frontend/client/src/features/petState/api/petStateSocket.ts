const API_URL = import.meta.env.VITE_API_URL ?? window.location.origin;
const WEBSOCKET_URL = import.meta.env.VITE_WEBSOCKET_URL;

type WebSocketTicketResponse = {
    ticket: string;
    expires_at: string;
};

export async function requestPetStateWebSocketTicket(accessToken: string): Promise<string> {
    const response = await fetch(new URL('/api/v1/ws-ticket', API_URL), {
        method: 'POST',
        headers: {
            Authorization: `Bearer ${accessToken}`,
        },
    });

    if (!response.ok) {
        throw new Error(`Unable to create WebSocket ticket: ${response.status}`);
    }

    const data: unknown = await response.json();

    if (!isWebSocketTicketResponse(data)) {
        throw new Error('Invalid WebSocket ticket response');
    }

    return data.ticket;
}

export function createPetStateWebSocketUrl(ticket: string): string {
    const url = new URL('/api/v1/ws/pet', WEBSOCKET_URL ?? API_URL);

    if (!WEBSOCKET_URL) {
        url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    }

    url.searchParams.set('ticket', ticket);

    return url.toString();
}

function isWebSocketTicketResponse(value: unknown): value is WebSocketTicketResponse {
    if (typeof value !== 'object' || value === null) {
        return false;
    }

    const ticketResponse = value as Partial<WebSocketTicketResponse>;

    return (
        typeof ticketResponse.ticket === 'string' &&
        ticketResponse.ticket.length > 0 &&
        typeof ticketResponse.expires_at === 'string'
    );
}
