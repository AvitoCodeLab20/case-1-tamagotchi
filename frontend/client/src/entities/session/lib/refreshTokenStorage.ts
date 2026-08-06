const refreshTokenKey = 'tamagotchi.refresh-token';

export function getRefreshToken(): string | null {
    return localStorage.getItem(refreshTokenKey);
}

export function saveRefreshToken(refreshToken: string): void {
    localStorage.setItem(refreshTokenKey, refreshToken);
}

export function removeRefreshToken(): void {
    localStorage.removeItem(refreshTokenKey);
}
