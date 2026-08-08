import { useEffect, type PropsWithChildren } from 'react';
import { restoreSession } from '@entities/session';

export function SessionBootstrap({ children }: PropsWithChildren) {
    useEffect(() => {
        void restoreSession();
    }, []);

    return children;
}
