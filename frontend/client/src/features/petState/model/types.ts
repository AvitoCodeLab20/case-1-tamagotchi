export type PetState = {
    level: number;
    experience: number;
    health: number;
    hunger: number;
    happiness: number;
    energy: number;
};

export type PetStateSocketStatus = 'idle' | 'connecting' | 'connected' | 'error';
