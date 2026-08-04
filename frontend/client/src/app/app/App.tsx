import { Outlet } from 'react-router-dom';
import { QueryClientProvider } from '@app/providers/queryClient';

export function App() {
    return (
        <QueryClientProvider>
            <Outlet />
        </QueryClientProvider>
    );
}
