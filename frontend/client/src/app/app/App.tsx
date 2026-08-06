import { Outlet } from 'react-router-dom';
import { QueryClientProvider } from '@app/providers/queryClient';
import { SessionBootstrap } from '@app/providers/session';
export function App() {
    return (
        <SessionBootstrap>
            <QueryClientProvider>
                <Outlet />
            </QueryClientProvider>
        </SessionBootstrap>
    );
}
