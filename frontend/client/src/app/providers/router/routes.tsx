import { lazy } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';

import { App } from '@app/app';
import { ERoutes } from '@entities/paths/path.ts';
import { GuestOnlyRoute } from './GuestOnlyRoute';

const HomePage = lazy(() =>
    import('@pages/homePage').then(({ HomePage }) => ({ default: HomePage }))
);
const LeaderboardPage = lazy(() =>
    import('@pages/leaderboardPage').then(({ LeaderboardPage }) => ({ default: LeaderboardPage }))
);
const AuthPage = lazy(() =>
    import('@pages/authPage').then(({ AuthPage }) => ({
        default: AuthPage,
    }))
);

export const routes: RouteObject[] = [
    {
        path: ERoutes.Home,
        element: <App />,
        children: [
            {
                index: true,
                element: <HomePage />,
            },
            {
                path: ERoutes.Leaderboard,
                element: <LeaderboardPage />,
            },
            {
                path: ERoutes.Auth,
                element: <GuestOnlyRoute />,
                children: [
                    {
                        index: true,
                        element: <AuthPage />,
                    },
                ],
            },
        ],
    },
    {
        path: '*',
        element: <Navigate to={ERoutes.Home} />,
    },
];
