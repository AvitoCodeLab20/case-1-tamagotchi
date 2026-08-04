import { Outlet } from 'react-router-dom';
import { QueryClientProvider } from '@app/providers/queryClient';
import { Header } from '@widgets/header';
export function App() {
    return (
        <QueryClientProvider>
            <Header />
            <Outlet />
        </QueryClientProvider>
    );
}
