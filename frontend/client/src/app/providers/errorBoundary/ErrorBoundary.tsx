import { type ErrorInfo, type ReactNode, Component } from 'react';
import { Button } from '@mui/material';

type ErrorBoundaryProps = {
    children: ReactNode;
};

type ErrorBoundaryState = {
    hasError: boolean;
};

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
    public state: ErrorBoundaryState = { hasError: false };

    public static getDerivedStateFromError(): ErrorBoundaryState {
        return { hasError: true };
    }

    public componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
        console.error('Application error', error, errorInfo);
    }

    public render(): ReactNode {
        if (this.state.hasError) {
            return (
                <main>
                    <h1>Что-то пошло не так</h1>
                    <p>Попробуйте перезагрузить страницу.</p>
                    <Button
                        sx={{ backgroundColor: 'var(--color-button-primary)' }}
                        onClick={() => window.location.reload()}
                    >
                        Перезагрузить
                    </Button>
                </main>
            );
        }

        return this.props.children;
    }
}
