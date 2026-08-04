import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import './index.css';
import { AppThemeProvider } from '@app/providers/theme';
import { AppRouter } from '@app/providers/router';

const rootElement: HTMLElement | null = document.getElementById('root');

if (!rootElement) {
    throw new Error('Root Element is missing');
}

createRoot(rootElement).render(
    <StrictMode>
        <AppThemeProvider>
            <AppRouter />
        </AppThemeProvider>
    </StrictMode>
);
