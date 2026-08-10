import { Navigate, Outlet } from 'react-router-dom';
import { useSessionStore } from '@entities/session';
import { ERoutes } from '@entities/paths';

export function GuestOnlyRoute() {
    const status = useSessionStore((state) => state.status);

    if (status === 'checking') {
        return null;
    }

    return status === 'authenticated' ? <Navigate to={ERoutes.Home} replace /> : <Outlet />;
}
