import { Suspense } from 'react';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';

import { ErrorBoundary } from '@app/providers/errorBoundary';

import { routes } from './routes';

const router = createBrowserRouter(routes);

export const AppRouter = () => {
    return (
        <ErrorBoundary>
            <Suspense fallback={null}>
                <RouterProvider router={router} />
            </Suspense>
        </ErrorBoundary>
    );
};
